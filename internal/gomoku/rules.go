/*
internal/gomoku/rules.go
模块：五子棋核心对弈规则引擎
职责：
- 驱动五子棋落子、胜负判定（四方向五连珠及以上判胜）、满盘判和
- 提供悔棋回溯、对局状态重置与盘面数据快照拷贝
*/

package gomoku

import (
	"errors"
	"fmt"
)

const (
	ReasonFiveInARow = "five_in_a_row"
	ReasonBoardFull  = "board_full"
)

// Game 五子棋对局实例
type Game struct {
	Board        [Size][Size]int `json:"board"`
	Side         int             `json:"side"` // 当前行动方：1 黑 / -1 白
	History      []Move          `json:"history"`
	Over         bool            `json:"over"`
	Winner       int             `json:"winner"`       // 1 黑胜 / -1 白胜 / 0 暂无或平局
	FinishReason string          `json:"finishReason"` // 终局原因
}

// NewGame 初始化标准五子棋对局，黑方先手
func NewGame() *Game {
	return &Game{
		Side:    Black,
		History: make([]Move, 0, 64),
	}
}

// MakeMove 当前行动方在坐标 p 处落子
func (g *Game) MakeMove(p Pos) (bool, error) {
	if g.Over {
		return false, errors.New("对局已结束")
	}
	if !p.Valid() {
		return false, fmt.Errorf("坐标越界: (%d, %d)", p.Row, p.Col)
	}
	if g.Board[p.Row][p.Col] != Empty {
		return false, errors.New("该位置已有棋子")
	}

	color := g.Side
	g.Board[p.Row][p.Col] = color

	step := len(g.History) + 1
	m := Move{
		Step:     step,
		Pos:      p,
		To:       p,
		Color:    color,
		Notation: FormatNotation(p, color),
	}
	g.History = append(g.History, m)

	// 胜负判定：是否达成五连及以上
	if g.checkWin(p, color) {
		g.Over = true
		g.Winner = color
		g.FinishReason = ReasonFiveInARow
		return true, nil
	}

	// 和棋判定：225 格全满
	if len(g.History) >= Size*Size {
		g.Over = true
		g.Winner = 0
		g.FinishReason = ReasonBoardFull
		return true, nil
	}

	// 轮转下子方
	g.Side = -g.Side
	return true, nil
}

// checkWin 检查以 (p.Row, p.Col) 为落子点在四个方向上是否有连续 5 颗或更多同色子
func (g *Game) checkWin(p Pos, color int) bool {
	dirs := [4][2]int{
		{0, 1},  // 水平向右
		{1, 0},  // 垂直向下
		{1, 1},  // 主对角线（右下）
		{1, -1}, // 副对角线（左下）
	}

	for _, d := range dirs {
		count := 1
		dr, dc := d[0], d[1]

		// 正向探测
		r, c := p.Row+dr, p.Col+dc
		for r >= 0 && r < Size && c >= 0 && c < Size && g.Board[r][c] == color {
			count++
			r += dr
			c += dc
		}

		// 反向探测
		r, c = p.Row-dr, p.Col-dc
		for r >= 0 && r < Size && c >= 0 && c < Size && g.Board[r][c] == color {
			count++
			r -= dr
			c -= dc
		}

		if count >= 5 {
			return true
		}
	}
	return false
}

// Undo 悔棋回退 n 步（通常为 1 步或 2 步）
func (g *Game) Undo(n int) bool {
	if n <= 0 || len(g.History) < n {
		return false
	}
	for i := 0; i < n; i++ {
		idx := len(g.History) - 1
		last := g.History[idx]
		g.Board[last.Pos.Row][last.Pos.Col] = Empty
		g.History = g.History[:idx]
	}

	// 恢复轮转行动方：偶数步为黑方，奇数步为白方
	if len(g.History)%2 == 0 {
		g.Side = Black
	} else {
		g.Side = White
	}

	g.Over = false
	g.Winner = 0
	g.FinishReason = ""
	return true
}

// LastMove 获取最后一步落子
func (g *Game) LastMove() *Move {
	if len(g.History) == 0 {
		return nil
	}
	return &g.History[len(g.History)-1]
}

// BoardSlice 将定长二维数组转换为切片以便 JSON 传输
func (g *Game) BoardSlice() [][]int {
	out := make([][]int, Size)
	for r := 0; r < Size; r++ {
		out[r] = make([]int, Size)
		for c := 0; c < Size; c++ {
			out[r][c] = g.Board[r][c]
		}
	}
	return out
}
