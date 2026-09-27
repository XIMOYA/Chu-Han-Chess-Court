/*
internal/game/piece.go
模块：棋子类型与子力价值定义
职责：
- 定义中国象棋 7 种棋子类型与红黑双方阵营常量
- 提供棋子类型/阵营反解及子力价值字典（用于打闲长捉评估）
*/

package game

// 棋子类型
const (
	Empty    = 0
	King     = 1 // 帅/将
	Advisor  = 2 // 仕/士
	Elephant = 3 // 相/象
	Horse    = 4 // 马
	Rook     = 5 // 车
	Cannon   = 6 // 炮
	Pawn     = 7 // 兵/卒
)

// 颜色：红方先行
const (
	Red   = 1
	Black = -1
)

// Piece 为带符号的棋子编码：正数红方，负数黑方，0 为空。
type Piece int8

func (p Piece) Type() int {
	if p > 0 {
		return int(p)
	}
	return int(-p)
}

func (p Piece) Color() int {
	if p > 0 {
		return Red
	}
	if p < 0 {
		return Black
	}
	return Empty
}

func makePiece(typ, color int) Piece {
	return Piece(typ * color)
}

// 棋子名称（用于中文棋谱）
var pieceNames = map[int]map[int]string{
	Red: {
		King: "帅", Advisor: "仕", Elephant: "相", Horse: "马",
		Rook: "车", Cannon: "炮", Pawn: "兵",
	},
	Black: {
		King: "将", Advisor: "士", Elephant: "象", Horse: "马",
		Rook: "车", Cannon: "砲", Pawn: "卒",
	},
}

func pieceName(typ, color int) string {
	return pieceNames[color][typ]
}

// 子力价值，用于"捉/兑/献"的简化判定
var pieceValue = map[int]int{
	King: 10000, Rook: 9, Cannon: 5, Horse: 4,
	Elephant: 2, Advisor: 2, Pawn: 1,
}
