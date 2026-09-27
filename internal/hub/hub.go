/*
internal/hub/hub.go
模块：房间协调与生命周期中枢
职责：
- 集中注册与管理所有对局房间实例
- 定时轮询驱动对局倒计时、离线判负与空房间回收
- 异步持久化对局结果记录与错误日志追踪
*/

package hub

import (
	"log"
	"sync"
	"time"

	"xiangqi/internal/auth"
	"xiangqi/internal/game"
	"xiangqi/internal/gomoku"
	"xiangqi/internal/store"
)

type gameRecordShim struct {
	gameType               string
	code                   string
	redID, blackID         int64
	result, reason, moves  string
	timeMode               string
	timeSec                int
	started, ended         string
}

// Hub 房间中心
type Hub struct {
	mu      sync.Mutex
	rooms   map[string]*Room
	store   *store.Store
	auth    *auth.Manager
	connSeq int64
}

// NewHub 创建 Hub 并启动后台巡检
func NewHub(st *store.Store, am *auth.Manager) *Hub {
	h := &Hub{
		rooms: map[string]*Room{},
		store: st,
		auth:  am,
	}
	go h.loop()
	return h
}

func (h *Hub) nextConnID() int64 {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.connSeq++
	return h.connSeq
}

// CreateRoom 创建房间并安排房主入座
func (h *Hub) CreateRoom(c *Client, gameType, timeMode string, timeSec int, side string, isAI bool, aiDiff string, visibility string) *Room {
	if gameType != "gomoku" {
		gameType = "xiangqi"
	}
	if timeMode != "budget" && timeMode != "per_move" {
		timeMode = "budget"
	}
	if timeSec <= 0 {
		timeSec = 1200
	}
	h.mu.Lock()
	var code string
	for {
		code = genCode(6)
		if _, ok := h.rooms[code]; !ok {
			break
		}
	}
	r := newRoom(h, code, gameType, timeMode, timeSec, isAI, aiDiff, visibility)
	h.rooms[code] = r
	h.mu.Unlock()

	r.mu.Lock()
	defer r.mu.Unlock()
	// random：房主入座前随机决定其执子
	if side == "random" {
		if gameType == "gomoku" {
			if time.Now().UnixNano()%2 == 0 {
				side = "black"
			} else {
				side = "white"
			}
		} else {
			if time.Now().UnixNano()%2 == 0 {
				side = "red"
			} else {
				side = "black"
			}
		}
	}
	r.addSeat(c, side)

	// 如果是人机对弈模式，自动安排 AI 对手入座并立即开赛
	if isAI {
		aiNameStr := aiName(aiDiff)
		aiSeat := &seat{uid: -1, name: aiNameStr, online: true, isAI: true}

		if gameType == "gomoku" {
			if side == "black" {
				r.white = aiSeat
				r.aiColor = gomoku.White
			} else {
				r.black = aiSeat
				r.aiColor = gomoku.Black
			}
		} else {
			if side == "red" {
				r.black = aiSeat
				r.aiColor = game.Black
			} else {
				r.red = aiSeat
				r.aiColor = game.Red
			}
		}
		r.startGame()
	}

	return r
}

// GetRoom 按房间号查找
func (h *Hub) GetRoom(code string) (*Room, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	r, ok := h.rooms[code]
	return r, ok
}

// ActiveRoomInfo 活跃房间简要信息（供后台监控）
type ActiveRoomInfo struct {
	Code        string `json:"code"`
	GameType    string `json:"gameType"`
	Status      string `json:"status"`
	RedName     string `json:"redName"`
	BlackName   string `json:"blackName"`
	WhiteName   string `json:"whiteName"`
	SpecsCount  int    `json:"specsCount"`
	TimeMode    string `json:"timeMode"`
	TimeSeconds int    `json:"timeSeconds"`
	MovesCount  int    `json:"movesCount"`
	CreatedAt   string `json:"createdAt"`
	StartedAt   string `json:"startedAt"`
}

// GetActiveRooms 获取所有活跃房间列表
func (h *Hub) GetActiveRooms() []ActiveRoomInfo {
	h.mu.Lock()
	rooms := make([]*Room, 0, len(h.rooms))
	for _, r := range h.rooms {
		rooms = append(rooms, r)
	}
	h.mu.Unlock()

	out := make([]ActiveRoomInfo, 0, len(rooms))
	for _, r := range rooms {
		r.mu.Lock()
		info := ActiveRoomInfo{
			Code:        r.code,
			GameType:    r.gameType,
			Status:      r.status,
			SpecsCount:  len(r.specs),
			TimeMode:    r.timeMode,
			TimeSeconds: r.timeSec,
			CreatedAt:   r.createdAt.Format(time.RFC3339),
		}
		if !r.startedAt.IsZero() {
			info.StartedAt = r.startedAt.Format(time.RFC3339)
		}
		if r.red != nil {
			info.RedName = r.red.name
		}
		if r.black != nil {
			info.BlackName = r.black.name
		}
		if r.white != nil {
			info.WhiteName = r.white.name
		}
		if r.gameType == "gomoku" && r.gomoku != nil {
			info.MovesCount = len(r.gomoku.History)
		} else if r.g != nil {
			info.MovesCount = len(r.g.History)
		}
		r.mu.Unlock()
		out = append(out, info)
	}
	return out
}

// CloseRoom 管理员强制解散房间
func (h *Hub) CloseRoom(code string) bool {
	h.mu.Lock()
	r, ok := h.rooms[code]
	if ok {
		delete(h.rooms, code)
	}
	h.mu.Unlock()

	if !ok || r == nil {
		return false
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	seats := []*seat{r.red, r.black, r.white}
	for _, s := range seats {
		if s != nil && s.client != nil {
			r.fail(s.client, "该房间已被管理员强制解散")
			s.client.leaveRoom()
		}
	}
	for _, sp := range r.specs {
		if sp != nil {
			r.fail(sp, "该房间已被管理员强制解散")
			sp.leaveRoom()
		}
	}
	return true
}

// GetLiveStats 获取当前在线与房间统计
func (h *Hub) GetLiveStats() (int, int64) {
	h.mu.Lock()
	roomCount := len(h.rooms)
	conns := h.connSeq
	h.mu.Unlock()
	return roomCount, conns
}

// GetPublicWaitingRooms 获取全服公开等待中的对战擂台
func (h *Hub) GetPublicWaitingRooms() []PublicRoomInfo {
	h.mu.Lock()
	rooms := make([]*Room, 0, len(h.rooms))
	for _, r := range h.rooms {
		rooms = append(rooms, r)
	}
	h.mu.Unlock()

	out := make([]PublicRoomInfo, 0)
	for _, r := range rooms {
		r.mu.Lock()
		if r.status == statusWaiting && r.visibility == "public" && !r.isAI {
			owner := ""
			sideStr := "random"
			if r.gameType == "gomoku" {
				if r.black != nil && r.white == nil {
					owner = r.black.name
					sideStr = "black"
				} else if r.white != nil && r.black == nil {
					owner = r.white.name
					sideStr = "white"
				}
			} else {
				if r.red != nil && r.black == nil {
					owner = r.red.name
					sideStr = "red"
				} else if r.black != nil && r.red == nil {
					owner = r.black.name
					sideStr = "black"
				}
			}
			if owner != "" {
				out = append(out, PublicRoomInfo{
					Code:        r.code,
					GameType:    r.gameType,
					OwnerName:   owner,
					Side:        sideStr,
					TimeMode:    r.timeMode,
					TimeSeconds: r.timeSec,
					CreatedAt:   r.createdAt.Format(time.RFC3339),
				})
			}
		}
		r.mu.Unlock()
	}
	return out
}

// FindMatch 极速匹配：搜寻现存公开等待且未满员的对应棋种房间
func (h *Hub) FindMatch(gameType string) (string, bool) {
	if gameType == "" {
		gameType = "xiangqi"
	}
	h.mu.Lock()
	rooms := make([]*Room, 0, len(h.rooms))
	for _, r := range h.rooms {
		rooms = append(rooms, r)
	}
	h.mu.Unlock()

	for _, r := range rooms {
		r.mu.Lock()
		matchable := r.status == statusWaiting &&
			r.visibility == "public" &&
			!r.isAI &&
			r.gameType == gameType

		if matchable {
			hasEmpty := false
			if r.gameType == "gomoku" {
				hasEmpty = (r.black == nil || r.white == nil)
			} else {
				hasEmpty = (r.red == nil || r.black == nil)
			}
			if hasEmpty {
				code := r.code
				r.mu.Unlock()
				return code, true
			}
		}
		r.mu.Unlock()
	}
	return "", false
}

func (h *Hub) saveRecord(rec *gameRecordShim) {
	r := &store.GameRecord{
		GameType: rec.gameType,
		Code:     rec.code, RedID: rec.redID, BlackID: rec.blackID,
		Result: rec.result, Reason: rec.reason, Moves: rec.moves,
		TimeMode: rec.timeMode, TimeSeconds: rec.timeSec,
		StartedAt: rec.started, EndedAt: rec.ended,
	}
	if _, err := h.store.RecordGame(r); err != nil {
		log.Printf("[store] 保存对局记录失败 code=%s: %v", rec.code, err)
	}
}

func (h *Hub) loop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for now := range ticker.C {
		h.tick(now)
	}
}

func (h *Hub) tick(now time.Time) {
	h.mu.Lock()
	rooms := make([]*Room, 0, len(h.rooms))
	for _, r := range h.rooms {
		rooms = append(rooms, r)
	}
	h.mu.Unlock()

	for _, r := range rooms {
		r.mu.Lock()
		r.tick(now)
		empty := r.isEmpty()
		createdAt := r.createdAt
		code := r.code
		r.mu.Unlock()

		if empty && now.Sub(createdAt) > emptyRoomTTL {
			h.mu.Lock()
			r.mu.Lock()
			shouldDelete := r.isEmpty() && now.Sub(r.createdAt) > emptyRoomTTL
			r.mu.Unlock()
			if shouldDelete {
				delete(h.rooms, code)
			}
			h.mu.Unlock()
		}
	}
}

func (r *Room) isEmpty() bool {
	if r.gameType == "gomoku" {
		return (r.black == nil || r.black.client == nil) &&
			(r.white == nil || r.white.client == nil) &&
			len(r.specs) == 0
	}
	return (r.red == nil || r.red.client == nil) &&
		(r.black == nil || r.black.client == nil) &&
		len(r.specs) == 0
}

// tick 房间内超时/离线判定（持锁调用）
func (r *Room) tick(now time.Time) {
	isPlaying := false
	var curSide int
	if r.gameType == "gomoku" {
		isPlaying = r.status == statusPlaying && r.gomoku != nil
		if isPlaying {
			curSide = r.gomoku.Side
		}
	} else {
		isPlaying = r.status == statusPlaying && r.g != nil
		if isPlaying {
			curSide = r.g.Side
		}
	}

	if isPlaying {
		// 待处理请求超时检查（30秒未响应自动取消，防止恶意拖延时间）
		if r.pending != nil && !r.pending.createdAt.IsZero() && now.Sub(r.pending.createdAt) > pendingTimeout {
			requester := r.pending.from
			r.turnStart = r.turnStart.Add(now.Sub(r.pending.createdAt))
			r.pending = nil
			if requester != nil && requester.client != nil {
				r.notify(requester.client, "对方未在时限内回应，请求已自动取消")
			}
			r.broadcast()
		}

		// 走棋超时：若有弹窗挂起，暂不判负（给被申请方应答思考时间）
		if r.pending == nil && r.currentDeadline(now) <= 0 {
			winner := -curSide
			r.finishGame(winner, "timeout")
			r.broadcast()
			return
		}

		// 玩家离线超时
		seats := []*seat{r.red, r.black}
		if r.gameType == "gomoku" {
			seats = []*seat{r.black, r.white}
		}
		for _, s := range seats {
			if s != nil && !s.online && now.Sub(s.offlineAt) > disconnectGrace {
				other := r.otherSeat(s)
				winnerColor := game.Red
				if r.gameType == "gomoku" {
					winnerColor = gomoku.Black
					if other == r.white {
						winnerColor = gomoku.White
					}
				} else {
					if other == r.black {
						winnerColor = game.Black
					}
				}
				r.finishGame(winnerColor, "disconnect")
				r.broadcast()
				return
			}
		}
	}
}
