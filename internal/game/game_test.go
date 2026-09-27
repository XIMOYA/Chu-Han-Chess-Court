package game

import "testing"

func (p Pos) fenTestKey() string {
	return string(rune('0'+p.Row)) + "-" + string(rune('0'+p.Col))
}

func gameFromFEN(fen string) *Game {
	return newFromFEN(fen)
}

func TestOpeningMoves(t *testing.T) {
	g := NewGame()
	ms := g.LegalMoves()
	if len(ms) != 44 {
		t.Fatalf("opening legal moves = %d, want 44", len(ms))
	}
}

func TestOpeningNotation(t *testing.T) {
	g := NewGame()
	// 炮二平五：右炮 (7,7) -> (7,4)
	if ok, err := g.MakeMove(Pos{7, 7}, Pos{7, 4}); !ok {
		t.Fatalf("炮二平五 failed: %v", err)
	}
	if g.History[0].Notation != "炮二平五" {
		t.Fatalf("notation = %q, want 炮二平五", g.History[0].Notation)
	}
	if g.Side != Black {
		t.Fatal("side should be black after red move")
	}
	// 马8进7：黑马 (0,7) -> (2,6)
	if ok, err := g.MakeMove(Pos{0, 7}, Pos{2, 6}); !ok {
		t.Fatalf("马8进7 failed: %v", err)
	}
	if g.History[1].Notation != "马8进7" {
		t.Fatalf("notation = %q, want 马8进7", g.History[1].Notation)
	}
}

func TestHorseLeg(t *testing.T) {
	// 红马 (4,4)，马腿 (3,4) 有红兵，则 (2,3)(2,5) 不可达
	g := gameFromFEN("4k4/9/9/4P4/4N4/9/9/9/9/4K4 w")
	tos := g.LegalMovesFrom(Pos{4, 4})
	for _, p := range tos {
		if p.Eq(Pos{2, 3}) || p.Eq(Pos{2, 5}) {
			t.Fatalf("horse should be blocked to %v", p)
		}
	}
	// 向后的 (6,3)(6,5) 马腿 (5,4) 空，应可达
	found := map[string]bool{}
	for _, p := range tos {
		found[p.fenTestKey()] = true
	}
	if !found[Pos{6, 3}.fenTestKey()] || !found[Pos{6, 5}.fenTestKey()] {
		t.Fatalf("backward horse moves missing: %v", tos)
	}
}

func TestElephantEye(t *testing.T) {
	// 红相 (9,2) 走 (7,4) 的象眼为 (8,3)；仕 (8,3) 堵住则 (7,4) 不可达
	g := gameFromFEN("4k4/9/9/9/9/9/9/9/3A5/2B1K4 w")
	tos := g.LegalMovesFrom(Pos{9, 2})
	for _, p := range tos {
		if p.Eq(Pos{7, 4}) {
			t.Fatal("elephant eye should block (7,4)")
		}
	}
}

func TestFlyingGeneral(t *testing.T) {
	// 两将同列中路无子遮挡；红兵 (8,3) 前进 (7,3) 后仍照面，该着法非法
	g := gameFromFEN("4k4/9/9/9/9/9/9/9/3P5/4K4 w")
	for _, m := range g.LegalMoves() {
		if m.From.Eq(Pos{8, 3}) && m.To.Eq(Pos{7, 3}) {
			t.Fatal("moving pawn while generals face each other must be illegal")
		}
	}
}

func TestCheckmate(t *testing.T) {
	// 红车 (1,3) 贴将将军；黑将 (0,3) 五个走点：
	// (0,2) 被红兵 (1,2) 控制；(0,4) 被红帅飞将控制；(1,2) 兵占且车控；
	// (1,3) 吃车则被红兵反将；(2,3) 被车控制。无解。
	g := gameFromFEN("3k5/9/2PR5/9/9/9/9/9/9/4K4 b")
	if !IsInCheck(&g.Board, Black) {
		t.Fatal("black should be in check")
	}
	if len(legalMoves(&g.Board, Black)) != 0 {
		t.Fatal("black should have no legal moves (checkmate)")
	}
}

func TestStalemate(t *testing.T) {
	// 黑孤将当前不被将军：红车 (1,3)(1,5) 控制 (0,3)(0,5)，
	// 红马 (3,3) 与红车共同控制 (1,4)，红兵 (5,4) 挡住红帅飞将射线。
	g := gameFromFEN("4k4/3R1R3/9/3N4/9/4P4/9/9/9/4K4 b")
	if IsInCheck(&g.Board, Black) {
		t.Fatal("black should NOT be in check (stalemate setup)")
	}
	if len(legalMoves(&g.Board, Black)) != 0 {
		t.Fatal("black should have no legal moves (stalemate)")
	}
}

func TestPerpetualCheck(t *testing.T) {
	// 黑车在 (9,0)/(8,0) 来回将军，红帅在 (9,4)/(8,4) 上下躲避；
	// 黑卒 (4,4) 挡住将帅中路照面。
	g := gameFromFEN("4k4/9/9/9/4p4/9/9/9/9/r3K4 w")
	seq := []struct{ from, to Pos }{
		{Pos{9, 4}, Pos{8, 4}}, // 红帅上
		{Pos{9, 0}, Pos{8, 0}}, // 黑车抬
		{Pos{8, 4}, Pos{9, 4}}, // 红帅下
		{Pos{8, 0}, Pos{9, 0}}, // 黑车落（第 2 次同局面）
		{Pos{9, 4}, Pos{8, 4}},
		{Pos{9, 0}, Pos{8, 0}},
		{Pos{8, 4}, Pos{9, 4}},
		{Pos{8, 0}, Pos{9, 0}}, // 第 3 次
	}
	for i, s := range seq {
		ok, err := g.MakeMove(s.from, s.to)
		if !ok {
			t.Fatalf("move %d failed: %v", i+1, err)
		}
	}
	if !g.Over {
		t.Fatal("game should be over after perpetual check loop")
	}
	if g.Winner != Red {
		t.Fatalf("winner = %d, want red (long-check side loses)", g.Winner)
	}
	if g.Reason != ReasonPerpetualCheck {
		t.Fatalf("reason = %q, want perpetual_check", g.Reason)
	}
}

func TestUndo(t *testing.T) {
	g := NewGame()
	if ok, _ := g.MakeMove(Pos{7, 7}, Pos{7, 4}); !ok {
		t.Fatal("move failed")
	}
	if !g.Undo(1) {
		t.Fatal("undo failed")
	}
	if len(g.History) != 0 || g.Side != Red || g.Over {
		t.Fatal("state not restored after undo")
	}
	if len(g.LegalMoves()) != 44 {
		t.Fatal("opening moves after undo should be 44")
	}
}

func TestTwinNotation(t *testing.T) {
	// 双红车同列 col4，前车 (3,4) 平 (3,3) 应记"前车平六"
	g := gameFromFEN("4k4/9/9/4R4/9/4R4/9/9/9/4K4 w")
	if ok, err := g.MakeMove(Pos{3, 4}, Pos{3, 3}); !ok {
		t.Fatalf("move failed: %v", err)
	}
	if g.History[0].Notation != "前车平六" {
		t.Fatalf("notation = %q, want 前车平六", g.History[0].Notation)
	}
}
