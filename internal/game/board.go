/*
internal/game/board.go
模块：棋盘数据结构与 FEN 串解析
职责：
- 表达 10x9 中国象棋棋盘及格点坐标合法性、九宫/过河边界判定
- 实现标准 FEN 局面串与内存棋盘的互相解析与序列化
*/

package game

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
)

const (
	Rows = 10
	Cols = 9
)

// Pos 棋盘坐标：Row 0 为黑方底线，Row 9 为红方底线；Col 0..8 从左到右。
type Pos struct {
	Row int `json:"r"`
	Col int `json:"c"`
}

// UnmarshalJSON 兼容支持 {r, c} 和 {row, col} 两种入参键名
func (p *Pos) UnmarshalJSON(data []byte) error {
	var raw struct {
		R   *int `json:"r"`
		C   *int `json:"c"`
		Row *int `json:"row"`
		Col *int `json:"col"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.R != nil {
		p.Row = *raw.R
	} else if raw.Row != nil {
		p.Row = *raw.Row
	}
	if raw.C != nil {
		p.Col = *raw.C
	} else if raw.Col != nil {
		p.Col = *raw.Col
	}
	return nil
}

func (p Pos) Valid() bool {
	return p.Row >= 0 && p.Row < Rows && p.Col >= 0 && p.Col < Cols
}

func (p Pos) Eq(o Pos) bool {
	return p.Row == o.Row && p.Col == o.Col
}

// Board 棋盘，[row][col]
type Board [Rows][Cols]Piece

func inPalace(p Pos, color int) bool {
	if p.Col < 3 || p.Col > 5 {
		return false
	}
	if color == Red {
		return p.Row >= 7 && p.Row <= 9
	}
	return p.Row >= 0 && p.Row <= 2
}

// crossedRiver 是否已过河（以 color 视角）
func crossedRiver(p Pos, color int) bool {
	if color == Red {
		return p.Row <= 4
	}
	return p.Row >= 5
}

func (b *Board) get(p Pos) Piece {
	return b[p.Row][p.Col]
}

func (b *Board) set(p Pos, pc Piece) {
	b[p.Row][p.Col] = pc
}

func (b *Board) clone() Board {
	var nb Board
	for r := 0; r < Rows; r++ {
		for c := 0; c < Cols; c++ {
			nb[r][c] = b[r][c]
		}
	}
	return nb
}

// findKing 找到某方将/帅的位置
func (b *Board) findKing(color int) (Pos, bool) {
	for r := 0; r < Rows; r++ {
		for c := 0; c < Cols; c++ {
			if b[r][c].Type() == King && b[r][c].Color() == color {
				return Pos{r, c}, true
			}
		}
	}
	return Pos{}, false
}

const startFEN = "rnbakabnr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RNBAKABNR w"

var fenPiece = map[byte]Piece{
	'K': makePiece(King, Red), 'A': makePiece(Advisor, Red), 'B': makePiece(Elephant, Red),
	'N': makePiece(Horse, Red), 'R': makePiece(Rook, Red), 'C': makePiece(Cannon, Red),
	'P': makePiece(Pawn, Red),
	'k': makePiece(King, Black), 'a': makePiece(Advisor, Black), 'b': makePiece(Elephant, Black),
	'n': makePiece(Horse, Black), 'r': makePiece(Rook, Black), 'c': makePiece(Cannon, Black),
	'p': makePiece(Pawn, Black),
}

// ParseFEN 解析局面部分（棋子放置 + 行棋方），形如
// "rnbakabnr/9/1c5c1/p1p1p1p1p/9/9/P1P1P1P1P/1C5C1/9/RNBAKABNR w"
func ParseFEN(fen string) (Board, int, error) {
	var b Board
	fields := strings.Fields(strings.TrimSpace(fen))
	if len(fields) < 2 {
		return b, 0, errors.New("invalid FEN: missing side")
	}
	ranks := strings.Split(fields[0], "/")
	if len(ranks) != Rows {
		return b, 0, errors.New("invalid FEN: expected 10 ranks")
	}
	for r, rank := range ranks {
		c := 0
		for _, ch := range []byte(rank) {
			if ch >= '1' && ch <= '9' {
				c += int(ch - '0')
				if c > Cols {
					return b, 0, errors.New("invalid FEN: rank too long")
				}
				continue
			}
			pc, ok := fenPiece[ch]
			if !ok {
				return b, 0, errors.New("invalid FEN: unknown piece " + string(ch))
			}
			if c >= Cols {
				return b, 0, errors.New("invalid FEN: rank too long")
			}
			b[r][c] = pc
			c++
		}
		if c != Cols {
			return b, 0, errors.New("invalid FEN: rank length must be 9")
		}
	}
	side := Red
	if fields[1] == "b" {
		side = Black
	} else if fields[1] != "w" {
		return b, 0, errors.New("invalid FEN: bad side")
	}
	return b, side, nil
}

// FEN 序列化局面（棋子放置 + 行棋方）
func (g *Game) FEN() string {
	var sb strings.Builder
	for r := 0; r < Rows; r++ {
		empty := 0
		for c := 0; c < Cols; c++ {
			pc := g.Board[r][c]
			if pc == Empty {
				empty++
				continue
			}
			if empty > 0 {
				sb.WriteString(strconv.Itoa(empty))
				empty = 0
			}
			sb.WriteByte(pc.fenChar())
		}
		if empty > 0 {
			sb.WriteString(strconv.Itoa(empty))
		}
		if r < Rows-1 {
			sb.WriteByte('/')
		}
	}
	if g.Side == Red {
		sb.WriteString(" w")
	} else {
		sb.WriteString(" b")
	}
	return sb.String()
}

func (p Piece) fenChar() byte {
	typ := p.Type()
	ch := byte('?')
	switch typ {
	case King:
		ch = 'k'
	case Advisor:
		ch = 'a'
	case Elephant:
		ch = 'b'
	case Horse:
		ch = 'n'
	case Rook:
		ch = 'r'
	case Cannon:
		ch = 'c'
	case Pawn:
		ch = 'p'
	}
	if p.Color() == Red {
		ch -= 32 // 小写转大写
	}
	return ch
}
