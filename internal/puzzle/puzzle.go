/*
internal/puzzle/puzzle.go
模块：象棋经典残局步骤校验与防守应手引擎
职责：
- 解析历代名谱正解走法序列 JSON
- 校验红方玩家破局走法的精准性，并自动驱动黑方对应防守着法
- 判定终局绝杀点与通关资格
*/

package puzzle

import (
	"encoding/json"
	"errors"
	"fmt"

	"xiangqi/internal/game"
)

// MoveStep 单步正解（含红方走步与黑方对应防守步）
type MoveStep struct {
	From     game.Pos  `json:"from"`
	To       game.Pos  `json:"to"`
	Notation string    `json:"notation,omitempty"`
	Reply    *MoveStep `json:"reply,omitempty"` // 黑方应手防守走法
}

// ParseSolution 解析正解走法序列
func ParseSolution(solutionJSON string) ([]MoveStep, error) {
	var steps []MoveStep
	if err := json.Unmarshal([]byte(solutionJSON), &steps); err != nil {
		return nil, fmt.Errorf("解析残局正解步骤失败: %w", err)
	}
	if len(steps) == 0 {
		return nil, errors.New("正解步骤为空")
	}
	return steps, nil
}

// CheckMove 校验玩家走法
func CheckMove(solutionJSON string, stepIndex int, from, to game.Pos) (valid bool, reply *MoveStep, finished bool, msg string) {
	steps, err := ParseSolution(solutionJSON)
	if err != nil {
		return false, nil, false, "残局配置错误"
	}

	if stepIndex < 0 || stepIndex >= len(steps) {
		return false, nil, false, "步骤越界"
	}

	expected := steps[stepIndex]
	if expected.From.Row != from.Row || expected.From.Col != from.Col ||
		expected.To.Row != to.Row || expected.To.Col != to.Col {
		return false, nil, false, "此招攻势已缓，未中要害，不妨再思量一番～"
	}

	// 走法正确！
	if expected.Reply != nil {
		// 还有黑方防守应手，继续下一步
		return true, expected.Reply, false, "妙着！黑方被迫应对！"
	}

	// 无应手且为最后一步，达成连照绝杀！
	if stepIndex == len(steps)-1 {
		return true, nil, true, "神之一手！恭喜参透玄机，破局成功！"
	}

	return true, nil, false, "妙着！请继续推进！"
}
