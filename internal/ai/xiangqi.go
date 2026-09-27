/*
internal/ai/xiangqi.go
模块：中国象棋博弈评估与 Alpha-Beta 剪枝搜索引擎
职责：
- 构建象棋子力价值与棋子位置权重表（PST 矩阵）
- 提供启发式吃子着法排序、极小化极大搜索与动态难度深度配置
*/

package ai

import (
	"math"
	"math/rand"
	"sort"
	"time"

	"xiangqi/internal/game"
)

var pieceValues = map[int]int{
	game.King:     10000,
	game.Rook:     1000,
	game.Cannon:   480,
	game.Horse:    420,
	game.Advisor:  220,
	game.Elephant: 220,
	game.Pawn:     120,
}

// 兵卒过河奖励位置分（越靠近敌方九宫分值越高）
var pawnTable = [10][9]int{
	{0, 30, 40, 50, 60, 50, 40, 30, 0},
	{0, 40, 60, 80, 90, 80, 60, 40, 0},
	{0, 40, 60, 80, 90, 80, 60, 40, 0},
	{0, 30, 50, 60, 70, 60, 50, 30, 0},
	{0, 20, 30, 40, 50, 40, 30, 20, 0},
	{0, 0, 0, 0, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 0, 0, 0, 0, 0},
	{0, 0, 0, 0, 0, 0, 0, 0, 0},
}

// evaluateXiangqi 评估盘面价值（以 color 视角：正数为优势，负数为劣势）
func evaluateXiangqi(b *game.Board, color int) int {
	score := 0
	for r := 0; r < 10; r++ {
		for c := 0; c < 9; c++ {
			p := b[r][c]
			if p == game.Empty {
				continue
			}
			t := p.Type()
			val := pieceValues[t]

			// 位置附加分
			switch t {
			case game.Pawn:
				if p.Color() == game.Red {
					val += pawnTable[r][c]
				} else {
					val += pawnTable[9-r][c]
				}
			case game.Rook:
				// 车占中路或横扫河沿加分
				if c == 4 {
					val += 30
				}
			case game.Horse:
				// 马进中央或活跃区域加分
				if c >= 2 && c <= 6 && r >= 2 && r <= 7 {
					val += 25
				}
			case game.Cannon:
				if c == 4 {
					val += 35
				}
			}

			if p.Color() == color {
				score += val
			} else {
				score -= val
			}
		}
	}
	return score
}

// orderMoves 启发式走法排序：吃子走法优先搜索，大幅提升 Alpha-Beta 剪枝效率
func orderMoves(moves []game.Move) {
	sort.Slice(moves, func(i, j int) bool {
		vi := 0
		vj := 0
		if moves[i].Captured != 0 {
			vi = pieceValues[moves[i].Captured] - pieceValues[moves[i].Piece]/10
		}
		if moves[j].Captured != 0 {
			vj = pieceValues[moves[j].Captured] - pieceValues[moves[j].Piece]/10
		}
		return vi > vj
	})
}

// alphabetaXiangqi 极小化极大搜索
func alphabetaXiangqi(b *game.Board, depth int, alpha, beta int, maximizing bool, rootColor int) int {
	if depth <= 0 {
		return evaluateXiangqi(b, rootColor)
	}

	curColor := rootColor
	if !maximizing {
		curColor = -rootColor
	}

	moves := game.LegalMovesFor(b, curColor)
	if len(moves) == 0 {
		// 绝杀或困毙
		if game.IsInCheck(b, curColor) {
			if maximizing {
				return -99999 + (10 - depth) // 越快输惩罚越大
			}
			return 99999 - (10 - depth)
		}
		return 0 // 和棋
	}

	orderMoves(moves)

	if maximizing {
		maxEval := -math.MaxInt32
		for _, m := range moves {
			nb := game.ApplyMove(b, m)
			eval := alphabetaXiangqi(&nb, depth-1, alpha, beta, false, rootColor)
			if eval > maxEval {
				maxEval = eval
			}
			if eval > alpha {
				alpha = eval
			}
			if beta <= alpha {
				break
			}
		}
		return maxEval
	}

	minEval := math.MaxInt32
	for _, m := range moves {
		nb := game.ApplyMove(b, m)
		eval := alphabetaXiangqi(&nb, depth-1, alpha, beta, true, rootColor)
		if eval < minEval {
			minEval = eval
		}
		if eval < beta {
			beta = eval
		}
		if beta <= alpha {
			break
		}
	}
	return minEval
}

// DecideXiangqiMove 根据品阶计算最佳象棋走法
func DecideXiangqiMove(b *game.Board, color int, difficulty string) (game.Move, bool) {
	moves := game.LegalMovesFor(b, color)
	if len(moves) == 0 {
		return game.Move{}, false
	}

	depth := 2
	switch difficulty {
	case "easy":
		depth = 1
	case "medium":
		depth = 2
	case "hard":
		depth = 3
	default:
		depth = 2
	}

	orderMoves(moves)

	bestVal := -math.MaxInt32
	var bestMoves []game.Move
	alpha := -math.MaxInt32
	beta := math.MaxInt32

	for _, m := range moves {
		nb := game.ApplyMove(b, m)
		val := alphabetaXiangqi(&nb, depth-1, alpha, beta, false, color)

		// 简单难度加入轻微随机扰动，使棋风更拟人
		if difficulty == "easy" {
			val += rand.Intn(60) - 30
		}

		if val > bestVal {
			bestVal = val
			bestMoves = []game.Move{m}
		} else if val == bestVal {
			bestMoves = append(bestMoves, m)
		}
		if val > alpha {
			alpha = val
		}
	}

	if len(bestMoves) == 0 {
		return moves[0], true
	}

	// 同等最优解中随机挑选一步，避免走法呆板
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return bestMoves[r.Intn(len(bestMoves))], true
}
