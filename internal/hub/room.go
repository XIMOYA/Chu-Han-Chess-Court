/*
internal/hub/room.go
模块：对局房间状态与生命周期
职责：
- 管理红黑双方座席与观战席位分配
- 驱动一局象棋的开始、终局结算与 Rematch 换先
- 维护包干与步时计时器、走子结算及悔棋回退
*/

package hub

import (
	"encoding/json"
	"math/rand"
	"sync"
	"time"

	"xiangqi/internal/ai"
	"xiangqi/internal/game"
	"xiangqi/internal/gomoku"
)

const (
	statusWaiting  = "waiting"
	statusPlaying  = "playing"
	statusFinished = "finished"

	disconnectGrace = 90 * time.Second
	emptyRoomTTL    = 5 * time.Minute
	pendingTimeout  = 30 * time.Second
)

type seat struct {
	uid       int64
	name      string
	client    *Client
	online    bool
	isAI      bool
	offlineAt time.Time
	timeLeft  time.Duration
}

func (s *seat) bind(c *Client) {
	s.client = c
	s.online = true
	s.offlineAt = time.Time{}
}

type pendingReq struct {
	kind      string // undo / draw
	from      *seat
	steps     int // 1（刚下完）或 2（轮到自己）
	createdAt time.Time
}

// Room 对局房间
type Room struct {
	mu           sync.Mutex
	code         string
	gameType     string // xiangqi / gomoku
	hub          *Hub
	red          *seat // 象棋红方
	black        *seat // 象棋黑方 或 五子棋黑方（先手）
	white        *seat // 五子棋白方（后手）
	specs        map[int64]*Client
	g            *game.Game
	gomoku       *gomoku.Game
	status       string
	timeMode     string
	timeSec      int
	turnStart    time.Time
	startedAt    time.Time
	pending      *pendingReq
	chats        []ChatMsg
	rematch      map[int64]bool
	recorded     bool
	winner       int
	finishReason string
	createdAt    time.Time
	isAI         bool
	aiDifficulty string
	aiColor      int
	visibility   string // public / private
}

func aiName(diff string) string {
	switch diff {
	case "easy":
		return "棋苑童子 (初学)"
	case "hard":
		return "博弈国手 (大师)"
	default:
		return "弈林教习 (进阶)"
	}
}

func newRoom(h *Hub, code, gameType, timeMode string, timeSec int, isAI bool, aiDiff string, visibility string) *Room {
	if gameType != "gomoku" {
		gameType = "xiangqi"
	}
	if aiDiff == "" {
		aiDiff = "medium"
	}
	if isAI {
		visibility = "private"
	} else if visibility != "private" {
		visibility = "public"
	}
	return &Room{
		code:         code,
		gameType:     gameType,
		hub:          h,
		specs:        map[int64]*Client{},
		status:       statusWaiting,
		timeMode:     timeMode,
		timeSec:      timeSec,
		rematch:      map[int64]bool{},
		createdAt:    time.Now(),
		isAI:         isAI,
		aiDifficulty: aiDiff,
		visibility:   visibility,
	}
}

func (r *Room) seatForColor(color int) *seat {
	if r.gameType == "gomoku" {
		if color == gomoku.Black {
			return r.black
		}
		return r.white
	}
	if color == game.Red {
		return r.red
	}
	return r.black
}

func (r *Room) otherSeat(s *seat) *seat {
	if r.gameType == "gomoku" {
		if r.black == s {
			return r.white
		}
		return r.black
	}
	if r.red == s {
		return r.black
	}
	return r.red
}

func (r *Room) seatOf(c *Client) *seat {
	if r.gameType == "gomoku" {
		if r.black != nil && r.black.client == c {
			return r.black
		}
		if r.white != nil && r.white.client == c {
			return r.white
		}
		return nil
	}
	if r.red != nil && r.red.client == c {
		return r.red
	}
	if r.black != nil && r.black.client == c {
		return r.black
	}
	return nil
}

// addSeat 等待中的房间安排玩家入座
func (r *Room) addSeat(c *Client, preferred string) *seat {
	s := &seat{uid: c.uid, name: c.username}
	s.bind(c)

	if r.gameType == "gomoku" {
		switch preferred {
		case "white":
			if r.white == nil {
				r.white = s
			} else {
				r.black = s
			}
		case "black":
			if r.black == nil {
				r.black = s
			} else {
				r.white = s
			}
		default:
			if r.black == nil {
				r.black = s
			} else {
				r.white = s
			}
		}
		return s
	}

	switch preferred {
	case "red":
		if r.red == nil {
			r.red = s
		} else {
			r.black = s
		}
	case "black":
		if r.black == nil {
			r.black = s
		} else {
			r.red = s
		}
	default:
		if r.red == nil {
			r.red = s
		} else {
			r.black = s
		}
	}
	return s
}

func (r *Room) startGame() {
	if r.gameType == "gomoku" {
		if r.black == nil || r.white == nil {
			return
		}
		d := time.Duration(r.timeSec) * time.Second
		r.black.timeLeft, r.white.timeLeft = d, d
		r.gomoku = gomoku.NewGame()
		r.status = statusPlaying
		r.turnStart = time.Now()
		r.startedAt = r.turnStart
		r.pending = nil
		r.rematch = map[int64]bool{}
		r.recorded = false
		if r.isAI && r.aiColor == gomoku.Black {
			go r.scheduleAIMove()
		}
		return
	}

	if r.red == nil || r.black == nil {
		return
	}
	d := time.Duration(r.timeSec) * time.Second
	r.red.timeLeft, r.black.timeLeft = d, d
	r.g = game.NewGame()
	r.status = statusPlaying
	r.turnStart = time.Now()
	r.startedAt = r.turnStart
	r.pending = nil
	r.rematch = map[int64]bool{}
	r.recorded = false
	if r.isAI && r.aiColor == game.Red {
		go r.scheduleAIMove()
	}
}

// scheduleAIMove 驱动 AI 模拟思考并落子
func (r *Room) scheduleAIMove() {
	// 拟真思考延时（450ms ~ 900ms）
	time.Sleep(time.Duration(450+rand.Intn(450)) * time.Millisecond)

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.status != statusPlaying {
		return
	}

	if r.gameType == "gomoku" {
		if r.gomoku == nil || r.gomoku.Side != r.aiColor {
			return
		}
		bestPos := ai.DecideGomokuMove(r.gomoku.Board, r.aiColor, r.aiDifficulty)
		ok, _ := r.gomoku.MakeMove(bestPos)
		if ok {
			r.settleMove(r.aiColor)
			if r.gomoku.Over {
				r.finishGame(r.gomoku.Winner, r.gomoku.FinishReason)
			}
			r.broadcast()
		}
		return
	}

	if r.g == nil || r.g.Side != r.aiColor {
		return
	}
	bestMove, ok := ai.DecideXiangqiMove(&r.g.Board, r.aiColor, r.aiDifficulty)
	if ok {
		ok, _ = r.g.MakeMove(bestMove.From, bestMove.To)
		if ok {
			r.settleMove(r.aiColor)
			if r.g.Over {
				r.finishGame(r.g.Winner, r.g.Reason)
			}
			r.broadcast()
		}
	}
}

func sideName(color int, gameType string) string {
	if gameType == "gomoku" {
		if color == gomoku.Black {
			return "black"
		}
		return "white"
	}
	if color == game.Red {
		return "red"
	}
	return "black"
}

// finishGame 结束对局：winner 为对应游戏胜方或 0 (和棋)
func (r *Room) finishGame(winner int, reason string) {
	if r.status == statusFinished {
		return
	}
	r.status = statusFinished
	r.winner = winner
	r.finishReason = reason
	r.pending = nil
	if reason == "timeout" {
		var curSide int
		if r.gameType == "gomoku" && r.gomoku != nil {
			curSide = r.gomoku.Side
		} else if r.g != nil {
			curSide = r.g.Side
		}
		if timedOutSeat := r.seatForColor(curSide); timedOutSeat != nil {
			timedOutSeat.timeLeft = 0
		}
	}
	r.recordResult(winner, reason)
}

func (r *Room) recordResult(winner int, reason string) {
	if r.recorded {
		return
	}
	var player1, player2 *seat
	if r.gameType == "gomoku" {
		if r.black == nil || r.white == nil {
			return
		}
		player1, player2 = r.black, r.white
	} else {
		if r.red == nil || r.black == nil {
			return
		}
		player1, player2 = r.red, r.black
	}
	r.recorded = true

	result := "draw"
	if r.gameType == "gomoku" {
		if winner == gomoku.Black {
			result = "black"
		} else if winner == gomoku.White {
			result = "white"
		}
	} else {
		if winner == game.Red {
			result = "red"
		} else if winner == game.Black {
			result = "black"
		}
	}

	movesJSON := ""
	if r.gameType == "gomoku" && r.gomoku != nil {
		b, _ := json.Marshal(r.gomoku.History)
		movesJSON = string(b)
	} else if r.g != nil {
		b, _ := json.Marshal(r.g.History)
		movesJSON = string(b)
	}

	now := time.Now().Format(time.RFC3339)
	rec := &gameRecordShim{
		gameType: r.gameType,
		code:     r.code,
		redID:    player1.uid,
		blackID:  player2.uid,
		result:   result,
		reason:   reason,
		moves:    movesJSON,
		timeMode: r.timeMode,
		timeSec:  r.timeSec,
		started:  r.startedAt.Format(time.RFC3339),
		ended:    now,
	}
	go r.hub.saveRecord(rec)
}

// settleMove 走棋后结算计时
func (r *Room) settleMove(moverColor int) {
	now := time.Now()
	elapsed := now.Sub(r.turnStart)
	if r.timeMode == "budget" {
		s := r.seatForColor(moverColor)
		if s != nil {
			s.timeLeft -= elapsed
			if s.timeLeft < 0 {
				s.timeLeft = 0
			}
		}
	}
	r.turnStart = now
}

// currentDeadline 当前走棋方的剩余思考时间
func (r *Room) currentDeadline(now time.Time) time.Duration {
	var curSide int
	if r.gameType == "gomoku" {
		if r.gomoku == nil {
			return 0
		}
		curSide = r.gomoku.Side
	} else {
		if r.g == nil {
			return 0
		}
		curSide = r.g.Side
	}
	if r.timeMode == "budget" {
		s := r.seatForColor(curSide)
		if s == nil {
			return 0
		}
		return s.timeLeft - now.Sub(r.turnStart)
	}
	return time.Duration(r.timeSec)*time.Second - now.Sub(r.turnStart)
}

// applyUndo 同意悔棋：根据申请时的步数（1 步或 2 步）回退
func (r *Room) applyUndo(steps int) bool {
	if r.gameType == "gomoku" {
		if r.gomoku == nil || steps <= 0 || len(r.gomoku.History) < steps {
			return false
		}
		if !r.gomoku.Undo(steps) {
			return false
		}
		r.turnStart = time.Now()
		r.pending = nil
		return true
	}

	if r.g == nil || steps <= 0 || len(r.g.History) < steps {
		return false
	}
	if !r.g.Undo(steps) {
		return false
	}
	r.turnStart = time.Now()
	r.pending = nil
	return true
}

// tryRematch 投票再来一局，双方都投则重开并换先
func (r *Room) tryRematch(c *Client) bool {
	s := r.seatOf(c)
	if s == nil {
		return false
	}
	r.rematch[s.uid] = true

	if r.gameType == "gomoku" {
		if r.black != nil && r.white != nil && r.rematch[r.black.uid] && r.rematch[r.white.uid] {
			r.black, r.white = r.white, r.black
			r.startGame()
			return true
		}
		return false
	}

	if r.red != nil && r.black != nil && r.rematch[r.red.uid] && r.rematch[r.black.uid] {
		// 双方同意再来一局，红黑座席对调换先
		r.red, r.black = r.black, r.red
		r.startGame()
		return true
	}
	return false
}

var codeAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

func genCode(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = codeAlphabet[rand.Intn(len(codeAlphabet))]
	}
	return string(b)
}
