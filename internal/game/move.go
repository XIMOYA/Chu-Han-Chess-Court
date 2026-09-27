/*
internal/game/move.go
模块：棋子着法与伪合法走法生成器
职责：
- 实现 7 种棋子走法：将帅九宫/飞将、仕相走斜塞眼、马走日蹩腿、车炮直射翻山、兵卒过河横行
- 生成指定棋子或全局的伪合法着法集合
*/

package game

// Move 一步棋
type Move struct {
	From     Pos    `json:"from"`
	To       Pos    `json:"to"`
	Piece    int    `json:"piece"`    // 移动的棋子类型
	Color    int    `json:"color"`    // 行棋方
	Captured int    `json:"captured"` // 被吃棋子类型，0 表示无
	Notation string `json:"notation"` // 中文棋谱
}

var (
	orthoDirs = [4][2]int{{-1, 0}, {1, 0}, {0, -1}, {0, 1}}
	horseStep = [8][2]int{
		{-2, -1}, {-2, 1}, {2, -1}, {2, 1},
		{-1, -2}, {-1, 2}, {1, -2}, {1, 2},
	}
	// 与 horseStep 对应的马腿位置（先直走的那格）
	horseLeg = [8][2]int{
		{-1, 0}, {-1, 0}, {1, 0}, {1, 0},
		{0, -1}, {0, 1}, {0, -1}, {0, 1},
	}
	elephantStep = [4][2]int{{-2, -2}, {-2, 2}, {2, -2}, {2, 2}}
	advisorStep  = [4][2]int{{-1, -1}, {-1, 1}, {1, -1}, {1, 1}}
)

// pseudoMoves 生成某方全部伪合法走法（不过滤走完被将军）
func pseudoMoves(b *Board, color int) []Move {
	var moves []Move
	for r := 0; r < Rows; r++ {
		for c := 0; c < Cols; c++ {
			p := Pos{r, c}
			pc := b.get(p)
			if pc.Color() != color {
				continue
			}
			moves = append(moves, pieceMoves(b, p, pc)...)
		}
	}
	return moves
}

// pieceMoves 生成指定位置棋子的伪合法走法
func pieceMoves(b *Board, from Pos, pc Piece) []Move {
	switch pc.Type() {
	case King:
		return kingMoves(b, from, pc)
	case Advisor:
		return fixedStepMoves(b, from, pc, advisorStep[:], func(to Pos) bool {
			return inPalace(to, pc.Color())
		})
	case Elephant:
		return elephantMoves(b, from, pc)
	case Horse:
		return horseMoves(b, from, pc)
	case Rook:
		return sliderMoves(b, from, pc, orthoDirs[:], false)
	case Cannon:
		return cannonMoves(b, from, pc)
	case Pawn:
		return pawnMoves(b, from, pc)
	}
	return nil
}

func addMove(b *Board, from, to Pos, pc Piece, out []Move) []Move {
	target := b.get(to)
	if target != Empty && target.Color() == pc.Color() {
		return out
	}
	m := Move{From: from, To: to, Piece: pc.Type(), Color: pc.Color()}
	if target != Empty {
		m.Captured = target.Type()
	}
	return append(out, m)
}

func kingMoves(b *Board, from Pos, pc Piece) []Move {
	var out []Move
	color := pc.Color()
	for _, d := range orthoDirs {
		to := Pos{from.Row + d[0], from.Col + d[1]}
		if to.Valid() && inPalace(to, color) {
			out = addMove(b, from, to, pc, out)
		}
	}
	// 飞将：同列且中间无子，可直接吃对方将
	if kp, ok := b.findKing(-color); ok && kp.Col == from.Col {
		blocked := false
		lo, hi := from.Row, kp.Row
		if lo > hi {
			lo, hi = hi, lo
		}
		for r := lo + 1; r < hi; r++ {
			if b[r][from.Col] != Empty {
				blocked = true
				break
			}
		}
		if !blocked {
			out = addMove(b, from, kp, pc, out)
		}
	}
	return out
}

func fixedStepMoves(b *Board, from Pos, pc Piece, steps [][2]int, ok func(Pos) bool) []Move {
	var out []Move
	for _, d := range steps {
		to := Pos{from.Row + d[0], from.Col + d[1]}
		if to.Valid() && ok(to) {
			out = addMove(b, from, to, pc, out)
		}
	}
	return out
}

func elephantMoves(b *Board, from Pos, pc Piece) []Move {
	var out []Move
	color := pc.Color()
	for _, d := range elephantStep {
		mid := Pos{from.Row + d[0]/2, from.Col + d[1]/2}
		to := Pos{from.Row + d[0], from.Col + d[1]}
		if !to.Valid() || !mid.Valid() {
			continue
		}
		if crossedRiver(to, color) {
			continue // 象不过河
		}
		if b.get(mid) != Empty {
			continue // 塞象眼
		}
		out = addMove(b, from, to, pc, out)
	}
	return out
}

func horseMoves(b *Board, from Pos, pc Piece) []Move {
	var out []Move
	for i, d := range horseStep {
		leg := Pos{from.Row + horseLeg[i][0], from.Col + horseLeg[i][1]}
		to := Pos{from.Row + d[0], from.Col + d[1]}
		if !to.Valid() {
			continue
		}
		if leg.Valid() && b.get(leg) != Empty {
			continue // 蹩马腿
		}
		out = addMove(b, from, to, pc, out)
	}
	return out
}

// sliderMoves 车的走法；cannon 单独处理炮架
func sliderMoves(b *Board, from Pos, pc Piece, dirs [][2]int, _ bool) []Move {
	var out []Move
	for _, d := range dirs {
		r, c := from.Row+d[0], from.Col+d[1]
		for (Pos{r, c}).Valid() {
			to := Pos{r, c}
			target := b.get(to)
			if target == Empty {
				out = addMove(b, from, to, pc, out)
			} else {
				if target.Color() != pc.Color() {
					out = addMove(b, from, to, pc, out)
				}
				break
			}
			r += d[0]
			c += d[1]
		}
	}
	return out
}

func cannonMoves(b *Board, from Pos, pc Piece) []Move {
	var out []Move
	for _, d := range orthoDirs {
		r, c := from.Row+d[0], from.Col+d[1]
		// 第一阶段：未遇炮架，只走空格
		for (Pos{r, c}).Valid() && b.get(Pos{r, c}) == Empty {
			out = addMove(b, from, Pos{r, c}, pc, out)
			r += d[0]
			c += d[1]
		}
		// 找到炮架，继续向后找第一个子，若为敌子则可吃
		if (Pos{r, c}).Valid() {
			r += d[0]
			c += d[1]
			for (Pos{r, c}).Valid() {
				target := b.get(Pos{r, c})
				if target != Empty {
					if target.Color() != pc.Color() {
						out = addMove(b, from, Pos{r, c}, pc, out)
					}
					break
				}
				r += d[0]
				c += d[1]
			}
		}
	}
	return out
}

func pawnMoves(b *Board, from Pos, pc Piece) []Move {
	var out []Move
	color := pc.Color()
	fwd := -color // 红方向 row 减小，黑方向 row 增大
	candidates := []Pos{{from.Row + fwd, from.Col}}
	if crossedRiver(from, color) {
		candidates = append(candidates, Pos{from.Row, from.Col - 1}, Pos{from.Row, from.Col + 1})
	}
	for _, to := range candidates {
		if to.Valid() {
			out = addMove(b, from, to, pc, out)
		}
	}
	return out
}
