/*
internal/gomoku/board.go
模块：五子棋棋盘模型与坐标基础
职责：
- 定义 15x15 五子棋棋盘维度与黑白棋子类型常量
- 提供坐标合法性校验与标准棋谱文本生成（如 H8、天元等）
*/

package gomoku

import (
	"encoding/json"
	"fmt"
)

const (
	Size  = 15 // 15x15 标准棋盘
	Empty = 0  // 空位
	Black = 1  // 黑子（先手）
	White = -1 // 白子（后手）
)

// Pos 棋盘坐标 (0..14, 0..14)
type Pos struct {
	Row int `json:"row"`
	Col int `json:"col"`
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
	if raw.Row != nil {
		p.Row = *raw.Row
	} else if raw.R != nil {
		p.Row = *raw.R
	}
	if raw.Col != nil {
		p.Col = *raw.Col
	} else if raw.C != nil {
		p.Col = *raw.C
	}
	return nil
}

// MarshalJSON 同时输出 r/c 与 row/col，确保前端任意组件均能读取
func (p Pos) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[string]int{
		"r":   p.Row,
		"c":   p.Col,
		"row": p.Row,
		"col": p.Col,
	})
}

// Valid 校验坐标是否在 15x15 盘面内
func (p Pos) Valid() bool {
	return p.Row >= 0 && p.Row < Size && p.Col >= 0 && p.Col < Size
}

// Move 一步落子
type Move struct {
	Step     int    `json:"step"`     // 手数序号（从 1 开始）
	Pos      Pos    `json:"pos"`      // 落子位置
	To       Pos    `json:"to"`       // 兼容通用棋盘 lastMove.to 格式
	Color    int    `json:"color"`    // 执子方：1 黑 / -1 白
	Notation string `json:"notation"` // 棋谱标记（例如 "H8" 或 "黑 (8,8)"）
}

// FormatNotation 生成人类可读的标准棋谱记法（列 A-O，行 1-15）
func FormatNotation(p Pos, color int) string {
	colChar := 'A' + rune(p.Col)
	rowNum := 15 - p.Row // 习惯上底部为 1，顶部为 15
	colorName := "黑"
	if color == White {
		colorName = "白"
	}
	return fmt.Sprintf("%s %c%d", colorName, colChar, rowNum)
}
