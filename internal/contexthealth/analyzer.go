package contexthealth

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Analyzer 上下文健康分析器
type Analyzer struct{}

// NewAnalyzer 创建分析器
func NewAnalyzer() *Analyzer {
	return &Analyzer{}
}

// --- JSONL 原始模型（与 analytics 包独立，避免循环依赖）---

type jsonlLine struct {
	Type      string        `json:"type"`
	UUID      string        `json:"uuid"`
	SessionID string        `json:"sessionId"`
	Timestamp string        `json:"timestamp"`
	Message   *jsonlMessage `json:"message"`
}

type jsonlMessage struct {
	Role    string      `json:"role"`
	Model   string      `json:"model"`
	Usage   *jsonlUsage `json:"usage"`
	Content any         `json:"content"`
}

type jsonlUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// --- 上下文窗口限制（token）---

var contextLimits = map[string]int{
	"claude-opus-4":     200000,
	"claude-opus-4-0":   200000,
	"claude-sonnet-4":   200000,
	"claude-sonnet-4-0": 200000,
	"claude-haiku-3-5":  200000,
	"claude-3-5-haiku":  200000,
	"mimo-v2.5":         200000,
	"deepseek-v4-pro":   200000,
}

func getContextLimit(model string) int {
	// 尝试精确匹配
	if limit, ok := contextLimits[model]; ok {
		return limit
	}
	// 尝试前缀匹配
	lower := strings.ToLower(model)
	for k, v := range contextLimits {
		if strings.HasPrefix(lower, k) {
			return v
		}
	}
	// 默认 200K
	return 200000
}

// AnalyzeSession 分析单个会话的上下文健康
func (a *Analyzer) AnalyzeSession(jsonlPath string) (*SessionHealth, error) {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return nil, fmt.Errorf("打开 JSONL 文件失败: %w", err)
	}
	defer file.Close()

	health := &SessionHealth{
		Alerts: []string{},
		Turns:  []TurnMetric{},
	}

	// UUID 去重：同一条消息可能被记录多次（streaming 事件）
	seenUUIDs := make(map[string]bool)
	turnIndex := 0
	var maxInputTokens int

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event jsonlLine
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		// 提取会话 ID 和模型
		if event.SessionID != "" && health.SessionID == "" {
			health.SessionID = event.SessionID
		}

		if event.Type != "assistant" || event.Message == nil {
			continue
		}

		if event.Message.Model != "" && health.Model == "" {
			health.Model = event.Message.Model
		}

		// 提取 token 使用
		if event.Message.Usage == nil {
			continue
		}

		inputTokens := event.Message.Usage.InputTokens
		outputTokens := event.Message.Usage.OutputTokens

		// UUID 去重
		if event.UUID != "" {
			if seenUUIDs[event.UUID] {
				continue
			}
			seenUUIDs[event.UUID] = true
		}

		// 分析内容块
		turn := TurnMetric{
			TurnIndex:    turnIndex,
			InputTokens:  inputTokens,
			OutputTokens: outputTokens,
			Timestamp:    event.Timestamp,
		}

		// 解析 content 块
		if blocks, ok := event.Message.Content.([]any); ok {
			for _, block := range blocks {
				blockMap, ok := block.(map[string]any)
				if !ok {
					continue
				}
				blockType, _ := blockMap["type"].(string)

				switch blockType {
				case "thinking":
					turn.ThinkingCount++
					if thinking, ok := blockMap["thinking"].(string); ok {
						turn.ThinkingChars += len(thinking)
					}
				case "tool_use":
					turn.ToolUseCount++
				}
			}
		}

		health.Turns = append(health.Turns, turn)
		health.ToolCallsTotal += turn.ToolUseCount
		if turn.ThinkingCount > 0 {
			health.ThinkingTurns++
		}

		// 跟踪峰值上下文
		if inputTokens > maxInputTokens {
			maxInputTokens = inputTokens
		}

		turnIndex++
	}

	health.TotalTurns = turnIndex
	health.MaxContextUsed = maxInputTokens
	health.ContextLimit = getContextLimit(health.Model)

	if health.ContextLimit > 0 {
		health.ContextUsagePct = float64(health.MaxContextUsed) / float64(health.ContextLimit) * 100
	}

	// 检测上下文压缩事件（input_tokens 大幅下降 >50%）
	health.CompressionEvents = detectCompressionEvents(health.Turns)

	// 计算平均思考字符数
	if health.ThinkingTurns > 0 {
		totalThinkingChars := 0
		for _, t := range health.Turns {
			totalThinkingChars += t.ThinkingChars
		}
		health.AvgThinkingChars = float64(totalThinkingChars) / float64(health.ThinkingTurns)
	}

	// 检测退化信号
	health.Alerts = detectAlerts(health)

	// 计算健康评分
	health.HealthScore = calculateHealthScore(health)
	health.HealthLevel = getHealthLevel(health.HealthScore)

	return health, nil
}

// detectCompressionEvents 检测上下文压缩事件
// 当 input_tokens 从高值大幅下降（>50%），说明发生了上下文压缩/截断
func detectCompressionEvents(turns []TurnMetric) int {
	if len(turns) < 2 {
		return 0
	}
	count := 0
	for i := 1; i < len(turns); i++ {
		prev := turns[i-1].InputTokens
		curr := turns[i].InputTokens
		if prev > 1000 && curr < prev/2 {
			count++
		}
	}
	return count
}

// AnalyzeOverview 分析所有会话的全局健康概览
func (a *Analyzer) AnalyzeOverview(claudeDir string) (*OverviewHealth, error) {
	overview := &OverviewHealth{
		TopSessions: []SessionHealth{},
	}

	// 遍历所有项目目录
	projectDirs, err := filepath.Glob(filepath.Join(claudeDir, "projects", "*"))
	if err != nil {
		return nil, fmt.Errorf("遍历项目目录失败: %w", err)
	}

	var allSessions []SessionHealth

	for _, projectDir := range projectDirs {
		// 跳过非目录
		info, err := os.Stat(projectDir)
		if err != nil || !info.IsDir() {
			continue
		}

		// 查找 JSONL 文件
		jsonlFiles, err := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
		if err != nil {
			continue
		}

		for _, jsonlPath := range jsonlFiles {
			health, err := a.AnalyzeSession(jsonlPath)
			if err != nil || health == nil || health.TotalTurns == 0 {
				continue
			}
			allSessions = append(allSessions, *health)
		}
	}

	overview.TotalSessions = len(allSessions)
	if len(allSessions) == 0 {
		return overview, nil
	}

	// 汇总统计
	totalCtxUsage := 0.0
	maxCtxUsage := 0.0
	totalScore := 0.0

	for _, s := range allSessions {
		totalCtxUsage += s.ContextUsagePct
		totalScore += float64(s.HealthScore)
		if s.ContextUsagePct > maxCtxUsage {
			maxCtxUsage = s.ContextUsagePct
		}
		if s.ContextUsagePct > 50 {
			overview.WarningCount++
		}
		if s.ContextUsagePct > 80 {
			overview.CriticalCount++
		}
	}

	overview.AvgContextUsage = totalCtxUsage / float64(len(allSessions))
	overview.MaxContextUsage = maxCtxUsage
	overview.AvgHealthScore = totalScore / float64(len(allSessions))

	// 按健康评分升序排列，取最需关注的前 10 个
	sortSessionsByHealth(allSessions)
	if len(allSessions) > 10 {
		overview.TopSessions = allSessions[:10]
	} else {
		overview.TopSessions = allSessions
	}

	return overview, nil
}

// detectAlerts 检测退化信号
func detectAlerts(health *SessionHealth) []string {
	var alerts []string

	// 上下文使用率告警（基于峰值）
	if health.ContextUsagePct > 80 {
		alerts = append(alerts, "⚠️ 峰值上下文超过 80%，接近退化阈值")
	} else if health.ContextUsagePct > 50 {
		alerts = append(alerts, "⚡ 峰值上下文超过 50%")
	}

	// 压缩事件告警
	if health.CompressionEvents > 5 {
		alerts = append(alerts, fmt.Sprintf("🔄 发生 %d 次上下文压缩，对话较长", health.CompressionEvents))
	} else if health.CompressionEvents > 0 {
		alerts = append(alerts, fmt.Sprintf("🔄 发生 %d 次上下文压缩", health.CompressionEvents))
	}

	// 思考深度告警
	if health.TotalTurns > 5 && health.ThinkingTurns == 0 {
		alerts = append(alerts, "🧠 全程无思考，可能影响推理质量")
	} else if health.TotalTurns > 0 {
		thinkingRatio := float64(health.ThinkingTurns) / float64(health.TotalTurns)
		if thinkingRatio < 0.2 && health.TotalTurns > 10 {
			alerts = append(alerts, "🧠 思考深度偏低（<20% 轮次有思考）")
		}
	}

	// 单轮输出过大
	for _, t := range health.Turns {
		if t.OutputTokens > 20000 {
			alerts = append(alerts, fmt.Sprintf("📏 第 %d 轮输出过大（%d tokens）", t.TurnIndex+1, t.OutputTokens))
			break // 只报一次
		}
	}

	return alerts
}

// calculateHealthScore 计算综合健康评分 (0-100)
func calculateHealthScore(health *SessionHealth) int {
	score := 100

	// 峰值上下文使用率扣分（主要因素）
	switch {
	case health.ContextUsagePct > 80:
		score -= 40
	case health.ContextUsagePct > 60:
		score -= 20
	case health.ContextUsagePct > 40:
		score -= 10
	case health.ContextUsagePct > 20:
		score -= 5
	}

	// 压缩事件扣分（频繁压缩说明对话很长）
	if health.CompressionEvents > 10 {
		score -= 15
	} else if health.CompressionEvents > 5 {
		score -= 10
	} else if health.CompressionEvents > 0 {
		score -= 3
	}

	// 思考深度扣分
	if health.TotalTurns > 10 {
		thinkingRatio := float64(health.ThinkingTurns) / float64(health.TotalTurns)
		if thinkingRatio < 0.1 {
			score -= 15
		} else if thinkingRatio < 0.2 {
			score -= 8
		}
	}

	// 每个退化信号扣分（最多扣 20）
	signalPenalty := len(health.Alerts) * 5
	if signalPenalty > 20 {
		signalPenalty = 20
	}
	score -= signalPenalty

	// 限制范围
	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	return score
}

// getHealthLevel 根据评分返回健康等级
func getHealthLevel(score int) string {
	switch {
	case score >= 80:
		return "excellent"
	case score >= 60:
		return "good"
	case score >= 40:
		return "warning"
	default:
		return "critical"
	}
}

// sortSessionsByHealth 按健康评分升序排列（最差的在前）
func sortSessionsByHealth(sessions []SessionHealth) {
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].HealthScore < sessions[j].HealthScore
	})
}
