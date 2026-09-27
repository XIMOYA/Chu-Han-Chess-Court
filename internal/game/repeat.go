/*
internal/game/repeat.go
模块：重复局面与长将长捉裁判
职责：
- 追踪历史局面哈希频次（FEN 快照对比）
- 依据子力价值与吃子危险性评估着法性质（将/捉/闲）
- 三次重复时判定单方长将判负、长捉判负或双方不变作和
*/

package game

// 重复局面（三次重复局面）裁判：
//   - 长将：循环内一方每步着法均将军，判负；
//   - 长捉：循环内一方每步都产生新的"捉"（可白吃或以小吃大），对方均为闲着，判负；
//   - 其余三次重复（双方均有闲着，或双方皆为打）按"双方不变作和"处理。
//
// 说明：此处"捉"采用竞赛规则精神的工程化简化（将帅、兵卒允许长捉；兑、献、拦归为闲），
// 覆盖绝大多数实战循环，极端判例（联合捉子、借捉还捉等复杂棋例）可能与正式裁判细则有出入。

type keySnap struct {
	key     string
	histLen int
}

type chaseKey struct {
	attacker Pos
	target   Pos
}

func (g *Game) positionKey() string {
	return g.FEN()
}

func (g *Game) recordKey() {
	k := g.positionKey()
	g.keyCount[k]++
	g.keySnaps = append(g.keySnaps, keySnap{k, len(g.History)})
}

// chaseSet 计算 color 方当前局面下全部"捉"（吃子得便宜）的关系集合
func chaseSet(b *Board, color int) map[chaseKey]bool {
	set := map[chaseKey]bool{}
	for r := 0; r < Rows; r++ {
		for c := 0; c < Cols; c++ {
			from := Pos{r, c}
			pc := b.get(from)
			if pc.Color() != color {
				continue
			}
			// 将帅、兵卒允许长捉，其捉子不记
			if pc.Type() == King || pc.Type() == Pawn {
				continue
			}
			for _, m := range pieceMoves(b, from, pc) {
				if m.Captured == Empty || m.Captured == King {
					continue
				}
				nb := apply(b, m)
				if isAttacked(&nb, m.To, -color) {
					// 吃子后会被反吃：只有以小吃大（得子）才算捉，等价兑/亏献为闲
					if pieceValue[m.Piece] >= pieceValue[m.Captured] {
						continue
					}
				}
				set[chaseKey{from, m.To}] = true
			}
		}
	}
	return set
}

// moveNature 判定一步棋的性质：是否将军、是否产生新捉
func moveNature(b *Board, m Move) (isCheck, newChase bool) {
	nb := apply(b, m)
	if IsInCheck(&nb, -m.Color) {
		return true, false
	}
	before := chaseSet(b, m.Color)
	after := chaseSet(&nb, m.Color)
	for k := range after {
		if !before[k] {
			newChase = true
			break
		}
	}
	return false, newChase
}

func (g *Game) replayBoard(history []Move) Board {
	b, _, _ := ParseFEN(g.StartFEN)
	for _, m := range history {
		b = apply(&b, m)
	}
	return b
}

func (g *Game) finish(winner int, reason string) {
	g.Over = true
	g.Winner = winner
	g.Reason = reason
}

// judgeRepeat 走子后若同一局面第三次出现，进行循环着法裁判
func (g *Game) judgeRepeat() {
	k := g.positionKey()
	if g.keyCount[k] < 3 {
		return
	}
	cur := g.keySnaps[len(g.keySnaps)-1]
	var prev keySnap
	found := false
	for i := len(g.keySnaps) - 2; i >= 0; i-- {
		if g.keySnaps[i].key == k {
			prev = g.keySnaps[i]
			found = true
			break
		}
	}
	if !found {
		return
	}
	loop := g.History[prev.histLen:cur.histLen]
	if len(loop) < 2 {
		return
	}

	allAttack := map[int]bool{Red: true, Black: true}
	allCheck := map[int]bool{Red: true, Black: true}
	hasMove := map[int]bool{}
	b := g.replayBoard(g.History[:prev.histLen])
	for _, m := range loop {
		hasMove[m.Color] = true
		chk, chase := moveNature(&b, m)
		if !chk && !chase {
			allAttack[m.Color] = false
		}
		if !chk {
			allCheck[m.Color] = false
		}
		b = apply(&b, m)
	}

	redA, blackA := allAttack[Red] && hasMove[Red], allAttack[Black] && hasMove[Black]
	redC, blackC := allCheck[Red] && hasMove[Red], allCheck[Black] && hasMove[Black]

	switch {
	case redA && !blackA:
		if redC {
			g.finish(Black, ReasonPerpetualCheck)
		} else {
			g.finish(Black, ReasonPerpetualChase)
		}
	case blackA && !redA:
		if blackC {
			g.finish(Red, ReasonPerpetualCheck)
		} else {
			g.finish(Red, ReasonPerpetualChase)
		}
	default:
		// 双方皆打或皆闲：长将方仍负，否则双方不变作和
		if redC && !blackC {
			g.finish(Black, ReasonPerpetualCheck)
		} else if blackC && !redC {
			g.finish(Red, ReasonPerpetualCheck)
		} else {
			g.finish(0, ReasonRepeatDraw)
		}
	}
}
