/*
internal/gomoku/gomoku_test.go
模块：五子棋规则引擎单元测试
职责：
- 全面验证落子合法性、横纵斜四个维度的五连珠胜负判定
- 验证越界拦截、同位重叠拦截、悔棋回溯与终局保护
*/

package gomoku

import (
	"testing"
)

func TestGomokuBasicFlow(t *testing.T) {
	g := NewGame()
	if g.Side != Black {
		t.Fatalf("expected Black start, got %d", g.Side)
	}

	// 正常落子
	ok, err := g.MakeMove(Pos{7, 7})
	if !ok || err != nil {
		t.Fatalf("MakeMove failed: %v", err)
	}
	if g.Side != White {
		t.Fatalf("expected turn toggle to White, got %d", g.Side)
	}

	// 重复落子拦截
	ok, err = g.MakeMove(Pos{7, 7})
	if ok || err == nil {
		t.Fatalf("expected error for duplicate move, got ok")
	}

	// 越界拦截
	ok, err = g.MakeMove(Pos{15, 0})
	if ok || err == nil {
		t.Fatalf("expected error for out of bounds, got ok")
	}
}

func TestGomokuHorizontalWin(t *testing.T) {
	g := NewGame()
	// 黑: (7, 0), (7, 1), (7, 2), (7, 3), (7, 4)
	// 白: (8, 0), (8, 1), (8, 2), (8, 3)
	moves := []Pos{
		{7, 0}, {8, 0},
		{7, 1}, {8, 1},
		{7, 2}, {8, 2},
		{7, 3}, {8, 3},
		{7, 4}, // 达成五连
	}

	for _, m := range moves {
		if ok, err := g.MakeMove(m); !ok || err != nil {
			t.Fatalf("MakeMove failed at %v: %v", m, err)
		}
	}

	if !g.Over || g.Winner != Black || g.FinishReason != ReasonFiveInARow {
		t.Fatalf("expected Black win by five_in_a_row, got over=%v, winner=%d, reason=%s", g.Over, g.Winner, g.FinishReason)
	}

	// 胜后禁止继续走棋
	if ok, _ := g.MakeMove(Pos{8, 4}); ok {
		t.Fatalf("expected cannot move after game over")
	}
}

func TestGomokuVerticalWin(t *testing.T) {
	g := NewGame()
	// 白方垂直五连胜
	moves := []Pos{
		{0, 0}, {1, 5},
		{0, 1}, {2, 5},
		{0, 2}, {3, 5},
		{0, 3}, {4, 5},
		{1, 0}, {5, 5}, // 白胜
	}
	for _, m := range moves {
		if ok, err := g.MakeMove(m); !ok || err != nil {
			t.Fatalf("MakeMove failed: %v", err)
		}
	}

	if !g.Over || g.Winner != White {
		t.Fatalf("expected White win, got over=%v winner=%d", g.Over, g.Winner)
	}
}

func TestGomokuDiagonalWin(t *testing.T) {
	g := NewGame()
	// 黑方主对角线五连胜: (3,3), (4,4), (5,5), (6,6), (7,7)
	moves := []Pos{
		{3, 3}, {0, 0},
		{4, 4}, {0, 1},
		{5, 5}, {0, 2},
		{6, 6}, {0, 3},
		{7, 7},
	}
	for _, m := range moves {
		if ok, err := g.MakeMove(m); !ok || err != nil {
			t.Fatalf("MakeMove failed: %v", err)
		}
	}
	if !g.Over || g.Winner != Black {
		t.Fatalf("expected Black diagonal win")
	}
}

func TestGomokuAntiDiagonalWin(t *testing.T) {
	g := NewGame()
	// 黑方副对角线五连胜: (3,7), (4,6), (5,5), (6,4), (7,3)
	moves := []Pos{
		{3, 7}, {0, 0},
		{4, 6}, {0, 1},
		{5, 5}, {0, 2},
		{6, 4}, {0, 3},
		{7, 3},
	}
	for _, m := range moves {
		if ok, err := g.MakeMove(m); !ok || err != nil {
			t.Fatalf("MakeMove failed: %v", err)
		}
	}
	if !g.Over || g.Winner != Black {
		t.Fatalf("expected Black anti-diagonal win")
	}
}

func TestGomokuUndo(t *testing.T) {
	g := NewGame()
	g.MakeMove(Pos{7, 7}) // 黑
	g.MakeMove(Pos{7, 8}) // 白

	if len(g.History) != 2 || g.Side != Black {
		t.Fatalf("pre-condition failed")
	}

	// 回退 1 步
	if !g.Undo(1) {
		t.Fatalf("Undo(1) failed")
	}
	if len(g.History) != 1 || g.Side != White || g.Board[7][8] != Empty {
		t.Fatalf("undo 1 step mismatch: history=%d, side=%d", len(g.History), g.Side)
	}

	// 再回退 1 步回到初始
	if !g.Undo(1) {
		t.Fatalf("Undo(1) failed")
	}
	if len(g.History) != 0 || g.Side != Black || g.Board[7][7] != Empty {
		t.Fatalf("undo to initial mismatch")
	}

	// 无法继续回退
	if g.Undo(1) {
		t.Fatalf("expected Undo to return false on empty history")
	}
}

func TestPosJSONSerialization(t *testing.T) {
	// 测试解析 {row, col}
	var p1 Pos
	if err := p1.UnmarshalJSON([]byte(`{"row":7,"col":8}`)); err != nil || p1.Row != 7 || p1.Col != 8 {
		t.Fatalf("failed to unmarshal {row, col}: %v, got %v", err, p1)
	}

	// 测试解析 {r, c}
	var p2 Pos
	if err := p2.UnmarshalJSON([]byte(`{"r":3,"c":4}`)); err != nil || p2.Row != 3 || p2.Col != 4 {
		t.Fatalf("failed to unmarshal {r, c}: %v, got %v", err, p2)
	}
}
