/*
internal/llm/client.go
模块：通用 OpenAI 兼容协议大语言模型客户端
职责：
- 动态对接 DeepSeek、Qwen、OpenAI、Ollama 等各厂商大模型接口
- 支持模型列表自动化拉取与过滤排序
- 支持会话联通测试、毫秒级延迟诊断与 Token 消耗用量捕获
*/

package llm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

var httpClient = &http.Client{
	Timeout: 15 * time.Second,
}

type modelItem struct {
	ID string `json:"id"`
}

type modelListResponse struct {
	Data  []modelItem `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float32       `json:"temperature"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func normalizeBaseURL(u string) string {
	u = strings.TrimSpace(u)
	u = strings.TrimRight(u, "/")
	return u
}

// GetModels 从大模型接口拉取当前支持的模型列表
func GetModels(baseURL, apiKey string) ([]string, error) {
	baseURL = normalizeBaseURL(baseURL)
	if baseURL == "" {
		return nil, errors.New("API Base URL 不能为空")
	}

	url := baseURL + "/models"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("构造模型请求失败: %w", err)
	}

	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	}
	req.Header.Set("User-Agent", "Xiangqi-Admin/1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求模型列表失败（请检查网络或接口地址）: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取模型响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		var errResp modelListResponse
		if json.Unmarshal(body, &errResp) == nil && errResp.Error != nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("模型接口拒绝 (%d): %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("模型接口返回异常状态码 %d: %s", resp.StatusCode, string(body))
	}

	var res modelListResponse
	if err := json.Unmarshal(body, &res); err != nil {
		return nil, fmt.Errorf("解析模型列表 JSON 格式错误: %w", err)
	}

	out := make([]string, 0, len(res.Data))
	for _, m := range res.Data {
		id := strings.TrimSpace(m.ID)
		if id != "" {
			out = append(out, id)
		}
	}
	sort.Strings(out)

	if len(out) == 0 {
		return nil, errors.New("未拉取到任何可用模型标识，请确认 API Key 权限")
	}
	return out, nil
}

// TestChat 发起单轮测试对话，并统计耗时及 Token 消耗
func TestChat(baseURL, apiKey, model string) (reply string, latencyMs int64, promptTokens, completionTokens, totalTokens int, err error) {
	baseURL = normalizeBaseURL(baseURL)
	if baseURL == "" {
		return "", 0, 0, 0, 0, errors.New("API Base URL 不能为空")
	}
	if model == "" {
		model = "deepseek-chat"
	}

	url := baseURL + "/chat/completions"
	reqData := chatRequest{
		Model: model,
		Messages: []chatMessage{
			{Role: "system", Content: "你是楚汉棋苑的古风AI棋客，言辞雅致脱俗，惜字如金。"},
			{Role: "user", Content: "请用一句话回答：弈棋之道的最高境界是什么？"},
		},
		MaxTokens:   80,
		Temperature: 0.7,
	}

	buf, _ := json.Marshal(reqData)
	req, err := http.NewRequest("POST", url, bytes.NewReader(buf))
	if err != nil {
		return "", 0, 0, 0, 0, fmt.Errorf("创建测试请求失败: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(apiKey))
	}

	start := time.Now()
	resp, err := httpClient.Do(req)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return "", latency, 0, 0, 0, fmt.Errorf("与大模型建立会话失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", latency, 0, 0, 0, fmt.Errorf("读取模型回复失败: %w", err)
	}

	var chatResp chatResponse
	if err := json.Unmarshal(body, &chatResp); err != nil {
		return "", latency, 0, 0, 0, fmt.Errorf("模型回复非标准 JSON (%d): %s", resp.StatusCode, string(body))
	}

	if resp.StatusCode != http.StatusOK {
		if chatResp.Error != nil && chatResp.Error.Message != "" {
			return "", latency, 0, 0, 0, fmt.Errorf("大模型返回错误 (%d): %s", resp.StatusCode, chatResp.Error.Message)
		}
		return "", latency, 0, 0, 0, fmt.Errorf("请求异常响应码 %d", resp.StatusCode)
	}

	if len(chatResp.Choices) == 0 {
		return "", latency, 0, 0, 0, errors.New("模型未返回任何回答内容")
	}

	content := strings.TrimSpace(chatResp.Choices[0].Message.Content)
	pTokens := chatResp.Usage.PromptTokens
	cTokens := chatResp.Usage.CompletionTokens
	tTokens := chatResp.Usage.TotalTokens
	if tTokens == 0 {
		tTokens = pTokens + cTokens
	}

	return content, latency, pTokens, cTokens, tTokens, nil
}
