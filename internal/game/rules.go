/*
internal/game/rules.go
模块：核心裁判与对局状态驱动
职责：
- 严格走子校验（将军过滤、绝对非法走法拦截）
- 驱动局面转移、终局判定（将死、困毙、和棋）
- 基于纯函数 Replay 机制安全实现悔棋（Undo）
*/

package game

import "fmt"

// 终局原因
const (
	ReasonCheckmate      = "checkmate"       // 将死
	ReasonStalemate      = "stalemate"       // 困毙
	ReasonPerpetualCheck = "perpetual_check" // 长将
	ReasonPerpetualChase = "perpetual_chase" // 长捉
	ReasonRepeatDraw     = "repeat_draw"     // 双方不变作和
)

// Game 一局棋的完整状态
type Game struct {
	Board    Board  `json:"board"`
	Side     int    `json:"side"` // 下一步行棋方
	History  []Move `json:"history"`
	Over     bool   `json:"over"`
	Winner   int    `json:"winner"` // Red / Black / 0(和)
	Reason   string `json:"reason"`
	StartFEN string `json:"startFen"`
	keyCount map[string]int
	keySnaps []keySnap
}

// NewGame 标准开局
func NewGame() *Game {
	return newFromFEN(startFEN)
}

func newFromFEN(fen string) *Game {
	b, side, err := ParseFEN(fen)
	if err != nil {
		panic(err)
	}
	g := &Game{
		Board:    b,
		Side:     side,
		StartFEN: fen,
		keyCount: map[string]int{},
	}
	g.recordKey()
	return g
}

func apply(b *Board, m Move) Board {
	nb := b.clone()
	nb[m.To.Row][m.To.Col] = makePiece(m.Piece, m.Color)
	nb[m.From.Row][m.From.Col] = Empty
	return nb
}

// isAttacked 位置 p 是否被 byColor 方攻击
func isAttacked(b *Board, p Pos, byColor int) bool {
	for r := 0; r < Rows; r++ {
		for c := 0; c < Cols; c++ {
			from := Pos{r, c}
			pc := b.get(from)
			if pc.Color() != byColor {
				continue
			}
			for _, m := range pieceMoves(b, from, pc) {
				if m.To.Eq(p) {
					return true
				}
			}
		}
	}
	return false
}

// IsInCheck color 方是否被将军
func IsInCheck(b *Board, color int) bool {
	kp, ok := b.findKing(color)
	if !ok {
		return false
	}
	return isAttacked(b, kp, -color)
}

func (g *Game) inCheck(color int) bool {
	return IsInCheck(&g.Board, color)
}

// ApplyMove 模拟走一步棋（不改变原盘面，返回新盘面副本）
func ApplyMove(b *Board, m Move) Board {
	return apply(b, m)
}

// LegalMovesFor 返回指定盘面指定颜色的全部合法走法
func LegalMovesFor(b *Board, color int) []Move {
	return legalMoves(b, color)
}

// LegalMoves 返回当前局面全部合法走法
func (g *Game) LegalMoves() []Move {
	return legalMoves(&g.Board, g.Side)
}

func legalMoves(b *Board, color int) []Move {
	var out []Move
	for _, m := range pseudoMoves(b, color) {
		nb := apply(b, m)
		if !IsInCheck(&nb, color) {
			out = append(out, m)
		}
	}
	return out
}

// LegalMovesFrom 返回某位置棋子可走的合法目标
func (g *Game) LegalMovesFrom(p Pos) []Pos {
	var out []Pos
	if g.Over {
		return out
	}
	pc := g.Board.get(p)
	if pc.Color() != g.Side {
		return out
	}
	for _, m := range g.LegalMoves() {
		if m.From.Eq(p) {
			out = append(out, m.To)
		}
	}
	return out
}

// MakeMove 尝试走棋，返回是否合法
func (g *Game) MakeMove(from, to Pos) (bool, error) {
	if g.Over {
		return false, fmt.Errorf("game already over")
	}
	pc := g.Board.get(from)
	if pc == Empty {
		return false, fmt.Errorf("no piece at %v", from)
	}
	if pc.Color() != g.Side {
		return false, fmt.Errorf("not your turn")
	}
	legal := g.LegalMoves()
	for i := range legal {
		if legal[i].From.Eq(from) && legal[i].To.Eq(to) {
			m := legal[i]
			m.Notation = g.Notation(m)
			g.CommitMove(m)
			return true, nil
		}
	}
	return false, fmt.Errorf("illegal move")
}

// CommitMove 提交一步已校验的走法，更新局面并进行终局/重复局面判定
func (g *Game) CommitMove(m Move) {
	g.Board = apply(&g.Board, m)
	g.History = append(g.History, m)
	g.Side = -g.Side

	if ms := legalMoves(&g.Board, g.Side); len(ms) == 0 {
		g.Over = true
		g.Winner = -g.Side
		if g.inCheck(g.Side) {
			g.Reason = ReasonCheckmate
		} else {
			g.Reason = ReasonStalemate
		}
		return
	}
	g.recordKey()
	g.judgeRepeat()
}

// Undo 回退 n 步（用于悔棋）
func (g *Game) Undo(n int) bool {
	if n <= 0 || n > len(g.History) {
		return false
	}
	kept := append([]Move(nil), g.History[:len(g.History)-n]...)
	ng := newFromFEN(g.StartFEN)
	for _, m := range kept {
		ng.Board = apply(&ng.Board, m)
		ng.History = append(ng.History, m)
		ng.Side = -ng.Side
		ng.recordKey()
	}
	g.Board = ng.Board
	g.Side = ng.Side
	g.History = ng.History
	g.keyCount = ng.keyCount
	g.keySnaps = ng.keySnaps
	g.Over = false
	g.Winner = 0
	g.Reason = ""
	return true
}

// LastMove 返回最后一步，无则 nil
func (g *Game) LastMove() *Move {
	if len(g.History) == 0 {
		return nil
	}
	m := g.History[len(g.History)-1]
	return &m
}
