/*
internal/hub/state.go
模块：房间状态序列化与连接私有视图投影
职责：
- 动态组装当前连接视角的 RoomState 快照（支持象棋与五子棋双模式）
- 实时计算各座席思考倒计时与读秒状态
- 映射棋盘二维/FEN 状态、着法记录与身份判定
*/

package hub

import (
	"encoding/json"
	"time"

	"xiangqi/internal/game"
	"xiangqi/internal/gomoku"
)

func (r *Room) seatInfo(s *seat, now time.Time) *SeatInfo {
	info := &SeatInfo{
		UserID: s.uid, Name: s.name, Online: s.online, IsAI: s.isAI,
		TimeLeft: s.timeLeft.Milliseconds(),
	}
	var curSide int
	isCurrent := false
	if r.status == statusPlaying {
		if r.gameType == "gomoku" && r.gomoku != nil {
			curSide = r.gomoku.Side
			isCurrent = r.seatForColor(curSide) == s
		} else if r.g != nil {
			curSide = r.g.Side
			isCurrent = r.seatForColor(curSide) == s
		}
	}

	if isCurrent {
		elapsed := now.Sub(r.turnStart)
		if r.timeMode == "budget" {
			info.TimeLeft = (s.timeLeft - elapsed).Milliseconds()
		} else {
			info.TimeLeft = (time.Duration(r.timeSec)*time.Second - elapsed).Milliseconds()
		}
		if info.TimeLeft < 0 {
			info.TimeLeft = 0
		}
	}
	if r.timeMode == "per_move" && (!isCurrent) {
		info.TimeLeft = int64(r.timeSec) * 1000
	}
	return info
}

// state 生成某连接视角的全量房间状态（持锁调用）
func (r *Room) state(c *Client) *RoomState {
	now := time.Now()
	st := &RoomState{
		GameType:     r.gameType,
		Code:         r.code,
		Status:       r.status,
		TimeMode:     r.timeMode,
		TimeSeconds:  r.timeSec,
		Spectators:   []SpectatorInfo{},
		Chats:        append(make([]ChatMsg, 0, len(r.chats)), r.chats...),
		RematchVote:  []string{},
		ServerTime:   now.UnixMilli(),
		Moves:        make([]any, 0),
		IsAI:         r.isAI,
		AIDifficulty: r.aiDifficulty,
	}

	if r.gameType == "gomoku" {
		if r.black != nil {
			st.Black = r.seatInfo(r.black, now)
		}
		if r.white != nil {
			st.White = r.seatInfo(r.white, now)
		}
		if r.gomoku != nil {
			st.GomokuBoard = r.gomoku.BoardSlice()
			for _, m := range r.gomoku.History {
				st.Moves = append(st.Moves, m)
			}
			st.Side = sideName(r.gomoku.Side, r.gameType)
		}
		if r.status == statusFinished {
			st.Over = true
			switch r.winner {
			case gomoku.Black:
				st.Winner = "black"
			case gomoku.White:
				st.Winner = "white"
			default:
				st.Winner = "draw"
			}
			st.Reason = r.finishReason
		}
	} else {
		if r.red != nil {
			st.Red = r.seatInfo(r.red, now)
		}
		if r.black != nil {
			st.Black = r.seatInfo(r.black, now)
		}
		if r.g != nil {
			st.FEN = r.g.FEN()
			for _, m := range r.g.History {
				st.Moves = append(st.Moves, m)
			}
			st.Side = sideName(r.g.Side, r.gameType)
			st.InCheck = game.IsInCheck(&r.g.Board, r.g.Side)
		}
		if r.status == statusFinished {
			st.Over = true
			switch r.winner {
			case game.Red:
				st.Winner = "red"
			case game.Black:
				st.Winner = "black"
			default:
				st.Winner = "draw"
			}
			st.Reason = r.finishReason
		}
	}

	for id, sp := range r.specs {
		st.Spectators = append(st.Spectators, SpectatorInfo{
			ConnID: id, Name: sp.username, You: sp == c,
		})
	}
	if r.pending != nil {
		st.Pending = &RequestMsg{Kind: r.pending.kind, From: r.pending.from.name}
	}
	for uid := range r.rematch {
		var name string
		if r.gameType == "gomoku" {
			if r.black != nil && r.black.uid == uid {
				name = r.black.name
			} else if r.white != nil && r.white.uid == uid {
				name = r.white.name
			}
		} else {
			if r.red != nil && r.red.uid == uid {
				name = r.red.name
			} else if r.black != nil && r.black.uid == uid {
				name = r.black.name
			}
		}
		if name != "" {
			st.RematchVote = append(st.RematchVote, name)
		}
	}
	if !r.turnStart.IsZero() {
		st.TurnStart = r.turnStart.UnixMilli()
	}
	st.You = r.youInfo(c)
	return st
}

func (r *Room) youInfo(c *Client) *YouInfo {
	if r.gameType == "gomoku" {
		if r.black != nil && r.black.client == c {
			return &YouInfo{Role: "black", Name: c.username}
		}
		if r.white != nil && r.white.client == c {
			return &YouInfo{Role: "white", Name: c.username}
		}
		return &YouInfo{Role: "spectator", Name: c.username}
	}
	if r.red != nil && r.red.client == c {
		return &YouInfo{Role: "red", Name: c.username}
	}
	if r.black != nil && r.black.client == c {
		return &YouInfo{Role: "black", Name: c.username}
	}
	return &YouInfo{Role: "spectator", Name: c.username}
}

func (c *Client) trySend(msg ServerMsg) {
	b, err := json.Marshal(msg)
	if err != nil {
		return
	}
	select {
	case <-c.closed:
	case c.sendCh <- b:
	default:
		go c.shutdown()
	}
}

func (r *Room) sendTo(c *Client) {
	if c == nil {
		return
	}
	c.trySend(ServerMsg{Type: sRoomState, RoomState: r.state(c)})
}

func (r *Room) broadcast() {
	seats := []*seat{r.red, r.black}
	if r.gameType == "gomoku" {
		seats = []*seat{r.black, r.white}
	}
	for _, s := range seats {
		if s != nil && s.client != nil {
			r.sendTo(s.client)
		}
	}
	for _, sp := range r.specs {
		r.sendTo(sp)
	}
}

func (r *Room) fail(c *Client, text string) {
	if c != nil {
		c.trySend(ServerMsg{Type: sError, Err: text})
	}
}

func (r *Room) notify(c *Client, text string) {
	if c != nil {
		c.trySend(ServerMsg{Type: sNotify, Notify: text})
	}
}

func (r *Room) sendRequest(req RequestMsg, to *seat) {
	if to != nil && to.client != nil {
		to.client.trySend(ServerMsg{Type: sRequest, Request: &req})
	}
}

func (r *Room) broadcastChat(ch ChatMsg) {
	msg := ServerMsg{Type: sChat, Chat: &ch}
	seats := []*seat{r.red, r.black}
	if r.gameType == "gomoku" {
		seats = []*seat{r.black, r.white}
	}
	for _, s := range seats {
		if s != nil && s.client != nil {
			s.client.trySend(msg)
		}
	}
	for _, sp := range r.specs {
		sp.trySend(msg)
	}
}
