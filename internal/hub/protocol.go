package hub

import "xiangqi/internal/game"

// 客户端 -> 服务端 消息类型
const (
	cCreateRoom = "create_room"
	cJoinRoom   = "join_room"
	cMove       = "move"
	cUndoReq    = "undo_request"
	cUndoAccept = "undo_accept"
	cUndoReject = "undo_reject"
	cDrawReq    = "draw_request"
	cDrawAccept = "draw_accept"
	cDrawReject = "draw_reject"
	cResign     = "resign"
	cChat       = "chat"
	cRematch    = "rematch"
)

// ClientMsg 客户端消息
type ClientMsg struct {
	Type         string     `json:"type"`
	GameType     string     `json:"gameType"` // xiangqi / gomoku
	TimeMode     string     `json:"timeMode"`
	TimeSeconds  int        `json:"timeSeconds"`
	Side         string     `json:"side"` // xiangqi: red / black / random; gomoku: black / white / random
	Code         string     `json:"code"`
	From         game.Pos   `json:"from"`
	To           game.Pos   `json:"to"`
	Text         string     `json:"text"`
	Spectate     bool       `json:"spectate"`     // 是否仅观战（不自动入座）
	IsAI         bool       `json:"isAi"`         // 是否为单人人机对战
	AIDifficulty string     `json:"aiDifficulty"` // easy / medium / hard
	Visibility   string     `json:"visibility"`   // public / private
}

// PublicRoomInfo 公共等待大厅展示的擂台信息
type PublicRoomInfo struct {
	Code        string `json:"code"`
	GameType    string `json:"gameType"`
	OwnerName   string `json:"ownerName"`
	Side        string `json:"side"` // 擂主所选执子
	TimeMode    string `json:"timeMode"`
	TimeSeconds int    `json:"timeSeconds"`
	CreatedAt   string `json:"createdAt"`
}

// 服务端 -> 客户端 消息类型
const (
	sRoomState = "room_state"
	sChat      = "chat"
	sRequest   = "request"
	sNotify    = "notify"
	sError     = "error"
)

// SeatInfo 座位信息
type SeatInfo struct {
	UserID   int64  `json:"userId"`
	Name     string `json:"name"`
	TimeLeft int64  `json:"timeLeft"` // 毫秒
	Online   bool   `json:"online"`
	IsAI     bool   `json:"isAi,omitempty"`
}

// SpectatorInfo 观战者信息
type SpectatorInfo struct {
	ConnID int64  `json:"connId"`
	Name   string `json:"name"`
	You    bool   `json:"you"`
}

// ChatMsg 聊天消息
type ChatMsg struct {
	Name string `json:"name"`
	Text string `json:"text"`
	TS   int64  `json:"ts"`
}

// RequestMsg 待处理请求
type RequestMsg struct {
	Kind string `json:"kind"` // undo / draw
	From string `json:"from"`
}

// RoomState 房间全量状态
type RoomState struct {
	GameType     string             `json:"gameType"` // xiangqi / gomoku
	Code         string             `json:"code"`
	Status       string             `json:"status"` // waiting / playing / finished
	Red          *SeatInfo          `json:"red,omitempty"`
	Black        *SeatInfo          `json:"black,omitempty"`
	White        *SeatInfo          `json:"white,omitempty"`
	Spectators   []SpectatorInfo    `json:"spectators"`
	TimeMode     string             `json:"timeMode"`
	TimeSeconds  int                `json:"timeSeconds"`
	FEN          string             `json:"fen,omitempty"`
	GomokuBoard  [][]int            `json:"gomokuBoard,omitempty"`
	Moves        []any              `json:"moves"`
	Side         string             `json:"side"`
	InCheck      bool               `json:"inCheck"`
	Over         bool               `json:"over"`
	Winner       string             `json:"winner"`
	Reason       string             `json:"reason"`
	Pending      *RequestMsg        `json:"pending"`
	Chats        []ChatMsg          `json:"chats"`
	RematchVote  []string           `json:"rematchVote"`
	ServerTime   int64              `json:"serverTime"`
	TurnStart    int64              `json:"turnStart"`
	You          *YouInfo           `json:"you"`
	IsAI         bool               `json:"isAi,omitempty"`
	AIDifficulty string             `json:"aiDifficulty,omitempty"`
}

// YouInfo 当前连接在房间中的身份
type YouInfo struct {
	Role string `json:"role"` // red / black / spectator
	Name string `json:"name"`
}

// ServerMsg 服务端消息
type ServerMsg struct {
	Type string `json:"type"`
	*RoomState
	Chat    *ChatMsg    `json:"chat,omitempty"`
	Request *RequestMsg `json:"request,omitempty"`
	Notify  string      `json:"notify,omitempty"`
	Err     string      `json:"err,omitempty"`
}
