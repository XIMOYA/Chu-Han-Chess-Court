/*
internal/ai/gomoku.go
模块：五子棋棋型评估与攻防平衡搜索引擎
职责：
- 识别活四、冲四、活三、眠三、活二等经典五子连珠攻防棋型
- 动态平衡自主进攻与对手截堵权重，提供多品阶智能落子推演
*/

package ai

import (
	"math/rand"
	"sort"
	"time"

	"xiangqi/internal/gomoku"
)

// 棋型权重常量
const (
	ScoreFive      = 100000
	ScoreOpenFour  = 12000
	ScoreRushFour  = 2500
	ScoreOpenThree = 2000
	ScoreRushThree = 300
	ScoreOpenTwo   = 200
	ScoreRushTwo   = 30
)

type candidatePoint struct {
	p     gomoku.Pos
	score int
}

// DecideGomokuMove 计算最佳五子棋落子点
func DecideGomokuMove(board [15][15]int, myColor int, difficulty string) gomoku.Pos {
	// 如果棋盘完全为空，先手直落天元 (7, 7)
	emptyCount := 0
	for r := 0; r < 15; r++ {
		for c := 0; c < 15; c++ {
			if board[r][c] == gomoku.Empty {
				emptyCount++
			}
		}
	}
	if emptyCount == 15*15 {
		return gomoku.Pos{Row: 7, Col: 7}
	}

	oppColor := -myColor
	candidates := getCandidates(board)
	if len(candidates) == 0 {
		return gomoku.Pos{Row: 7, Col: 7}
	}

	// 评估每个候选落子点的攻防复合分值
	scored := make([]candidatePoint, len(candidates))
	for i, p := range candidates {
		myScore := evaluatePoint(board, p, myColor)
		oppScore := evaluatePoint(board, p, oppColor)

		// 进攻分 + 防守阻截分（防守权重略高，敏锐堵截活三与冲四）
		total := myScore*11/10 + oppScore

		// 简单难度加入轻度随机干扰
		if difficulty == "easy" {
			total += rand.Intn(400) - 200
		}
		scored[i] = candidatePoint{p: p, score: total}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// 若最高分具有绝对优势（如自己连五或阻止对方连五），直接执行
	if scored[0].score >= ScoreOpenFour {
		return scored[0].p
	}

	// 难度为 hard 且候选充足时，对前 4 个最优候选进行简易深度反推
	if difficulty == "hard" && len(scored) >= 3 {
		bestP := scored[0].p
		bestVal := -99999999
		topCount := 4
		if len(scored) < topCount {
			topCount = len(scored)
		}

		for i := 0; i < topCount; i++ {
			p := scored[i].p
			board[p.Row][p.Col] = myColor
			// 对手下一步的最强反击分
			oppBest := evaluateBestReply(board, oppColor)
			board[p.Row][p.Col] = gomoku.Empty

			val := scored[i].score - oppBest
			if val > bestVal {
				bestVal = val
				bestP = p
			}
		}
		return bestP
	}

	// 取前若干个相近分值中的随机最优，使下棋灵动自然
	topThreshold := scored[0].score - 100
	var pool []gomoku.Pos
	for _, sc := range scored {
		if sc.score >= topThreshold {
			pool = append(pool, sc.p)
		} else {
			break
		}
	}

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return pool[r.Intn(len(pool))]
}

func evaluateBestReply(board [15][15]int, color int) int {
	cands := getCandidates(board)
	maxSc := 0
	for _, p := range cands {
		sc := evaluatePoint(board, p, color)
		if sc > maxSc {
			maxSc = sc
		}
	}
	return maxSc
}

// evaluatePoint 评估在点 p 处落 color 子所能构成的四向棋型分值
func evaluatePoint(board [15][15]int, p gomoku.Pos, color int) int {
	board[p.Row][p.Col] = color
	defer func() {
		board[p.Row][p.Col] = gomoku.Empty
	}()

	dirs := [4][2]int{
		{0, 1},  // 水平
		{1, 0},  // 垂直
		{1, 1},  // 正对角
		{1, -1}, // 反对角
	}

	total := 0
	for _, d := range dirs {
		total += scoreDirection(board, p, d[0], d[1], color)
	}
	return total
}

func scoreDirection(board [15][15]int, p gomoku.Pos, dr, dc, color int) int {
	count := 1
	openEnds := 0

	// 正向延伸
	r, c := p.Row+dr, p.Col+dc
	for r >= 0 && r < 15 && c >= 0 && c < 15 && board[r][c] == color {
		count++
		r += dr
		c += dc
	}
	if r >= 0 && r < 15 && c >= 0 && c < 15 && board[r][c] == gomoku.Empty {
		openEnds++
	}

	// 反向延伸
	r, c = p.Row-dr, p.Col-dc
	for r >= 0 && r < 15 && c >= 0 && c < 15 && board[r][c] == color {
		count++
		r -= dr
		c -= dc
	}
	if r >= 0 && r < 15 && c >= 0 && c < 15 && board[r][c] == gomoku.Empty {
		openEnds++
	}

	if count >= 5 {
		return ScoreFive
	}
	if count == 4 {
		if openEnds == 2 {
			return ScoreOpenFour
		}
		if openEnds == 1 {
			return ScoreRushFour
		}
	}
	if count == 3 {
		if openEnds == 2 {
			return ScoreOpenThree
		}
		if openEnds == 1 {
			return ScoreRushThree
		}
	}
	if count == 2 {
		if openEnds == 2 {
			return ScoreOpenTwo
		}
		if openEnds == 1 {
			return ScoreRushTwo
		}
	}
	return 0
}

// getCandidates 收集已有棋子周围邻近距离 <= 2 的全部空位作为候选点（大幅减少搜索广度）
func getCandidates(board [15][15]int) []gomoku.Pos {
	visited := [15][15]bool{}
	var out []gomoku.Pos

	for r := 0; r < 15; r++ {
		for c := 0; c < 15; c++ {
			if board[r][c] != gomoku.Empty {
				for dr := -2; dr <= 2; dr++ {
					for dc := -2; dc <= 2; dc++ {
						nr, nc := r+dr, c+dc
						if nr >= 0 && nr < 15 && nc >= 0 && nc < 15 && board[nr][nc] == gomoku.Empty && !visited[nr][nc] {
							visited[nr][nc] = true
							out = append(out, gomoku.Pos{Row: nr, Col: nc})
						}
					}
				}
			}
		}
	}
	return out
}
