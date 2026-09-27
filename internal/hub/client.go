/*
internal/hub/client.go
模块：WebSocket 客户端连接管理
职责：
- 维持单个 WebSocket 连接的生命周期与心跳机制
- 处理客户端上行的走棋、提和、悔棋、聊天与房间信令
- 线程安全维护当前连接归属房间状态与消息发送管道
*/

package hub

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"xiangqi/internal/game"
	"xiangqi/internal/gomoku"
)

// Client 一个 WebSocket 连接
type Client struct {
	conn     *websocket.Conn
	hub      *Hub
	uid      int64
	username string
	connID   int64
	roomMu   sync.RWMutex
	room     *Room
	sendCh   chan []byte
	closed   chan struct{}
	once     sync.Once
}

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 20 * time.Second
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// ServeWS 处理 WebSocket 升级
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &Client{
		conn:   conn,
		hub:    h,
		connID: h.nextConnID(),
		sendCh: make(chan []byte, 32),
		closed: make(chan struct{}),
	}
	c.username = fmt.Sprintf("游客%04d", c.connID%10000)
	if token := r.URL.Query().Get("token"); token != "" {
		if claims, perr := h.auth.Parse(token); perr == nil {
			c.uid = claims.UID
			c.username = claims.Username
		}
	}
	go c.writePump()
	c.readPump()
}

func (c *Client) readPump() {
	defer c.shutdown()
	c.conn.SetReadLimit(8192)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		var msg ClientMsg
		if err := c.conn.ReadJSON(&msg); err != nil {
			return
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
		c.handle(&msg)
	}
}

func (c *Client) writePump() {
	ping := time.NewTicker(pingPeriod)
	defer ping.Stop()
	for {
		select {
		case <-c.closed:
			return
		case data := <-c.sendCh:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ping.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) getRoom() *Room {
	c.roomMu.RLock()
	defer c.roomMu.RUnlock()
	return c.room
}

func (c *Client) setRoom(r *Room) {
	c.roomMu.Lock()
	defer c.roomMu.Unlock()
	c.room = r
}

func (c *Client) shutdown() {
	c.once.Do(func() {
		close(c.closed)
		_ = c.conn.Close()
		c.leaveRoom()
	})
}

func (c *Client) inRoom(f func(*Room)) {
	r := c.getRoom()
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	f(r)
}

func (c *Client) failMsg(text string) {
	c.trySend(ServerMsg{Type: sError, Err: text})
}

func colorOfSeat(r *Room, s *seat) int {
	if r.gameType == "gomoku" {
		if r.black == s {
			return gomoku.Black
		}
		return gomoku.White
	}
	if r.red == s {
		return game.Red
	}
	return game.Black
}

func (c *Client) handle(msg *ClientMsg) {
	switch msg.Type {
	case cCreateRoom:
		if c.uid == 0 {
			c.failMsg("请先登录后再创建房间")
			return
		}
		if c.getRoom() != nil {
			c.failMsg("你已在一个房间中，请先离开")
			return
		}
		r := c.hub.CreateRoom(c, msg.GameType, msg.TimeMode, msg.TimeSeconds, msg.Side, msg.IsAI, msg.AIDifficulty, msg.Visibility)
		c.setRoom(r)
		r.mu.Lock()
		r.sendTo(c)
		r.mu.Unlock()

	case cJoinRoom:
		code := strings.ToUpper(strings.TrimSpace(msg.Code))
		r, ok := c.hub.GetRoom(code)
		if !ok {
			c.failMsg("房间不存在，请核对房间号")
			return
		}
		// 若已在其他房间，先平滑退出旧房间
		if oldR := c.getRoom(); oldR != nil && oldR != r {
			c.leaveRoom()
		}
		r.mu.Lock()
		c.enterRoom(r, msg.Spectate)
		r.mu.Unlock()

	case cMove:
		c.inRoom(func(r *Room) {
			s := r.seatOf(c)
			if s == nil {
				r.fail(c, "观战者不能走棋")
				return
			}
			color := colorOfSeat(r, s)
			if r.status != statusPlaying {
				return
			}
			if r.gameType == "gomoku" {
				if r.gomoku == nil {
					return
				}
				if r.gomoku.Side != color {
					r.fail(c, "还没轮到你走棋")
					return
				}
				target := gomoku.Pos{Row: msg.To.Row, Col: msg.To.Col}
				ok, err := r.gomoku.MakeMove(target)
				if !ok {
					r.fail(c, "非法落子："+err.Error())
					return
				}
				r.settleMove(color)
				if r.gomoku.Over {
					r.finishGame(r.gomoku.Winner, r.gomoku.FinishReason)
				} else if r.isAI && r.gomoku.Side == r.aiColor {
					go r.scheduleAIMove()
				}
				r.broadcast()
				return
			}

			if r.g == nil {
				return
			}
			if r.g.Side != color {
				r.fail(c, "还没轮到你走棋")
				return
			}
			ok, err := r.g.MakeMove(msg.From, msg.To)
			if !ok {
				r.fail(c, "非法走法："+err.Error())
				return
			}
			r.settleMove(color)
			if r.g.Over {
				r.finishGame(r.g.Winner, r.g.Reason)
			} else if r.isAI && r.g.Side == r.aiColor {
				go r.scheduleAIMove()
			}
			r.broadcast()
		})

	case cUndoReq:
		c.inRoom(func(r *Room) {
			s := r.seatOf(c)
			if s == nil || r.status != statusPlaying {
				return
			}
			if r.isAI {
				// 人机模式秒速悔棋，直接回退双方各一步（如果轮到自己），或回退刚走的一步
				steps := 2
				curMovesCount := 0
				if r.gameType == "gomoku" && r.gomoku != nil {
					curMovesCount = len(r.gomoku.History)
				} else if r.g != nil {
					curMovesCount = len(r.g.History)
				}
				if curMovesCount == 0 {
					r.fail(c, "开局第一步无法悔棋")
					return
				}
				if curMovesCount < 2 {
					steps = 1
				}
				if r.applyUndo(steps) {
					r.notify(c, "已为您回退棋步")
					r.broadcast()
				}
				return
			}

			if r.pending != nil {
				r.fail(c, "已有一个待处理请求")
				return
			}
			color := colorOfSeat(r, s)
			var steps int

			if r.gameType == "gomoku" {
				if r.gomoku == nil {
					return
				}
				if r.gomoku.Side != color {
					last := r.gomoku.LastMove()
					if last != nil && last.Color == color && len(r.gomoku.History) >= 1 {
						steps = 1
					} else {
						r.fail(c, "当前无法申请悔棋")
						return
					}
				} else {
					if len(r.gomoku.History) < 2 {
						r.fail(c, "开局第一步无法悔棋")
						return
					}
					steps = 2
				}
			} else {
				if r.g == nil {
					return
				}
				if r.g.Side != color {
					last := r.g.LastMove()
					if last != nil && last.Color == color && len(r.g.History) >= 1 {
						steps = 1
					} else {
						r.fail(c, "当前无法申请悔棋")
						return
					}
				} else {
					if len(r.g.History) < 2 {
						r.fail(c, "开局第一步无法悔棋")
						return
					}
					steps = 2
				}
			}

			r.pending = &pendingReq{kind: "undo", from: s, steps: steps, createdAt: time.Now()}
			r.sendRequest(RequestMsg{Kind: "undo", From: s.name}, r.otherSeat(s))
			r.broadcast()
		})

	case cUndoAccept, cUndoReject:
		c.inRoom(func(r *Room) {
			s := r.seatOf(c)
			if s == nil || r.pending == nil || r.pending.kind != "undo" || r.pending.from == s {
				return
			}
			requester := r.pending.from
			steps := r.pending.steps
			createdAt := r.pending.createdAt
			r.pending = nil

			// 补齐申请弹窗期间所占用的时间
			if !createdAt.IsZero() {
				r.turnStart = r.turnStart.Add(time.Since(createdAt))
			}

			if msg.Type == cUndoAccept {
				if r.applyUndo(steps) {
					r.notify(requester.client, "对方同意了悔棋")
				}
			} else {
				r.notify(requester.client, "对方拒绝了悔棋")
			}
			r.broadcast()
		})

	case cDrawReq:
		c.inRoom(func(r *Room) {
			s := r.seatOf(c)
			if s == nil || r.status != statusPlaying {
				return
			}
			if r.pending != nil {
				r.fail(c, "已有一个待处理请求")
				return
			}
			r.pending = &pendingReq{kind: "draw", from: s, createdAt: time.Now()}
			r.sendRequest(RequestMsg{Kind: "draw", From: s.name}, r.otherSeat(s))
			r.broadcast()
		})

	case cDrawAccept, cDrawReject:
		c.inRoom(func(r *Room) {
			s := r.seatOf(c)
			if s == nil || r.pending == nil || r.pending.kind != "draw" || r.pending.from == s {
				return
			}
			requester := r.pending.from
			createdAt := r.pending.createdAt
			r.pending = nil

			// 补齐申请弹窗期间所占用的时间
			if !createdAt.IsZero() {
				r.turnStart = r.turnStart.Add(time.Since(createdAt))
			}

			if msg.Type == cDrawAccept {
				r.finishGame(0, "draw_accepted")
				r.notify(requester.client, "对方同意和棋")
			} else {
				r.notify(requester.client, "对方拒绝和棋")
			}
			r.broadcast()
		})

	case cResign:
		c.inRoom(func(r *Room) {
			s := r.seatOf(c)
			if s == nil || r.status != statusPlaying {
				return
			}
			color := colorOfSeat(r, s)
			r.finishGame(-color, "resign")
			r.broadcast()
		})

	case cChat:
		c.inRoom(func(r *Room) {
			if c.uid == 0 {
				r.fail(c, "游客只能观战，登录后可发言")
				return
			}
			text := strings.TrimSpace(msg.Text)
			if text == "" {
				return
			}
			if len([]rune(text)) > 200 {
				text = string([]rune(text)[:200])
			}
			ch := ChatMsg{Name: c.username, Text: text, TS: time.Now().UnixMilli()}
			r.chats = append(r.chats, ch)
			if len(r.chats) > 100 {
				r.chats = r.chats[len(r.chats)-100:]
			}
			r.broadcastChat(ch)
		})

	case cRematch:
		c.inRoom(func(r *Room) {
			if r.status != statusFinished {
				return
			}
			if r.isAI {
				// 人机模式直接换先重开
				if r.gameType == "gomoku" {
					r.black, r.white = r.white, r.black
					r.aiColor = -r.aiColor
				} else {
					r.red, r.black = r.black, r.red
					r.aiColor = -r.aiColor
				}
				r.startGame()
				r.broadcast()
				return
			}
			r.tryRematch(c)
			r.broadcast()
		})
	}
}

// enterRoom 进入房间（持锁）：处理重连、入座、观战
func (c *Client) enterRoom(r *Room, spectate bool) {
	if c.getRoom() == r {
		r.sendTo(c)
		return
	}
	c.setRoom(r)

	// 同 uid 玩家重连，恢复座位（仅在非显式观战意图时）
	if !spectate && c.uid != 0 {
		seats := []*seat{r.red, r.black}
		if r.gameType == "gomoku" {
			seats = []*seat{r.black, r.white}
		}
		for _, s := range seats {
			if s != nil && s.uid == c.uid {
				s.bind(c)
				r.broadcast()
				return
			}
		}
	}

	// 等待中且有空座，且非显式观战意图，登录用户才入座
	hasEmpty := false
	if r.gameType == "gomoku" {
		hasEmpty = r.black == nil || r.white == nil
	} else {
		hasEmpty = r.red == nil || r.black == nil
	}

	if !spectate && r.status == statusWaiting && c.uid != 0 && hasEmpty {
		r.addSeat(c, "")
		bothReady := false
		if r.gameType == "gomoku" {
			bothReady = r.black != nil && r.white != nil
		} else {
			bothReady = r.red != nil && r.black != nil
		}
		if bothReady {
			r.startGame()
			r.broadcast()
		} else {
			r.sendTo(c)
		}
		return
	}
	// 其余或显式声明观战的用户进入观战席位
	r.specs[c.connID] = c
	r.broadcast()
}

func (c *Client) leaveRoom() {
	c.roomMu.Lock()
	r := c.room
	c.room = nil
	c.roomMu.Unlock()

	if r == nil {
		return
	}
	r.mu.Lock()
	c.detach(r)
	r.broadcast()
	r.mu.Unlock()
}

// detach 从房间移除连接（持锁）；座位仅在仍绑定该连接时才标记离线
func (c *Client) detach(r *Room) {
	seats := []*seat{r.red, r.black}
	if r.gameType == "gomoku" {
		seats = []*seat{r.black, r.white}
	}
	for _, s := range seats {
		if s != nil && s.client == c {
			s.client = nil
			s.online = false
			s.offlineAt = time.Now()
		}
	}
	if sp, ok := r.specs[c.connID]; ok && sp == c {
		delete(r.specs, c.connID)
	}
}
