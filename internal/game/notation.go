/*
internal/game/notation.go
模块：中国象棋标准中文记谱器
职责：
- 转换红黑双方列编号（红方一至九，黑方1至9）
- 生成进/退/平标准着法描述（如“炮二平五”、“马8进7”）
- 支持同列双子“前/后”歧义消解（如“前车平六”）
*/

package game

import "strconv"

var redNumCN = []string{"零", "一", "二", "三", "四", "五", "六", "七", "八", "九"}

// colName 列编号：红方从右起一~九，黑方从右起 1~9
func colName(col, color int) string {
	if color == Red {
		return redNumCN[9-col]
	}
	return strconv.Itoa(col + 1)
}

func numName(n, color int) string {
	if color == Red {
		return redNumCN[n]
	}
	return strconv.Itoa(n)
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// Notation 生成一步棋的中文记谱
func (g *Game) Notation(m Move) string {
	name := pieceName(m.Piece, m.Color)

	// 查找同列同类同色棋子（处理"前车平六""后马进三"）
	var twins []Pos
	for r := 0; r < Rows; r++ {
		pc := g.Board[r][m.From.Col]
		if pc.Type() == m.Piece && pc.Color() == m.Color {
			twins = append(twins, Pos{r, m.From.Col})
		}
	}

	var head string
	if len(twins) == 2 {
		var isFront bool
		if m.Color == Red {
			isFront = m.From.Row == twins[0].Row // 红方 row 小者为前
		} else {
			isFront = m.From.Row == twins[1].Row // 黑方 row 大者为前
		}
		if isFront {
			head = "前" + name
		} else {
			head = "后" + name
		}
	} else {
		head = name + colName(m.From.Col, m.Color)
	}

	var action, tail string
	switch {
	case m.To.Col != m.From.Col && m.To.Row == m.From.Row:
		action = "平"
		tail = colName(m.To.Col, m.Color)
	default:
		forward := false
		if m.Color == Red {
			forward = m.To.Row < m.From.Row
		} else {
			forward = m.To.Row > m.From.Row
		}
		if forward {
			action = "进"
		} else {
			action = "退"
		}
		switch m.Piece {
		case Horse, Elephant, Advisor:
			tail = colName(m.To.Col, m.Color)
		default:
			tail = numName(absInt(m.To.Row-m.From.Row), m.Color)
		}
	}
	return head + action + tail
}
