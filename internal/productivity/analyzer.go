package productivity

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"argus-desktop/internal/session/claude"
)

// Engine 生产力分析引擎
type Engine struct {
	homeDir string
	mu      sync.RWMutex
	cache   *ProductivityReport
	cacheAt time.Time
}

// NewEngine 创建生产力分析引擎
func NewEngine() (*Engine, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户目录失败: %w", err)
	}
	return &Engine{homeDir: homeDir}, nil
}

// Analyze 分析指定天数的生产力数据
func (e *Engine) Analyze(days int) (*ProductivityReport, error) {
	if days <= 0 {
		days = 30
	}

	// 检查缓存（5 分钟有效）
	e.mu.RLock()
	if e.cache != nil && e.cache.Period == fmt.Sprintf("%dd", days) && time.Since(e.cacheAt) < 5*time.Minute {
		result := e.cache
		e.mu.RUnlock()
		return result, nil
	}
	e.mu.RUnlock()

	// 扫描所有会话
	metrics, err := e.scanSessions(days)
	if err != nil {
		return nil, fmt.Errorf("扫描会话失败: %w", err)
	}

	// 生成报告
	report := e.buildReport(metrics, days)

	// 更新缓存
	e.mu.Lock()
	e.cache = report
	e.cacheAt = time.Now()
	e.mu.Unlock()

	return report, nil
}

// GetTrend 获取周趋势数据
func (e *Engine) GetTrend(weeks int) ([]WeeklyProductivity, error) {
	if weeks <= 0 {
		weeks = 12
	}

	report, err := e.Analyze(weeks * 7)
	if err != nil {
		return nil, err
	}

	if weeks > len(report.WeeklyTrend) {
		weeks = len(report.WeeklyTrend)
	}
	return report.WeeklyTrend[len(report.WeeklyTrend)-weeks:], nil
}

// scanSessions 扫描指定天数内的所有会话，提取生产力指标
func (e *Engine) scanSessions(days int) ([]sessionMetrics, error) {
	claudeDir := filepath.Join(e.homeDir, ".claude", "projects")
	if _, err := os.Stat(claudeDir); os.IsNotExist(err) {
		return nil, nil
	}

	cutoff := time.Now().AddDate(0, 0, -days)
	var allMetrics []sessionMetrics

	entries, err := os.ReadDir(claudeDir)
	if err != nil {
		return nil, fmt.Errorf("读取项目目录失败: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projectDir := filepath.Join(claudeDir, entry.Name())
		jsonlFiles, _ := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
		if len(jsonlFiles) == 0 {
			continue
		}

		for _, jsonlPath := range jsonlFiles {
			// 快速检查文件修改时间，跳过太旧的文件
			fi, err := os.Stat(jsonlPath)
			if err != nil {
				continue
			}
			if fi.ModTime().Before(cutoff) {
				continue
			}

			m, err := e.parseSessionMetrics(jsonlPath, entry.Name())
			if err != nil || m == nil {
				continue
			}

			// 检查会话时间是否在范围内
			if m.StartedAt > 0 && m.StartedAt < cutoff.UnixMilli() {
				continue
			}

			allMetrics = append(allMetrics, *m)
		}
	}

	return allMetrics, nil
}

// parseSessionMetrics 从 JSONL 文件解析单个会话的生产力指标
func (e *Engine) parseSessionMetrics(jsonlPath, projectDir string) (*sessionMetrics, error) {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fi, err := file.Stat()
	if err != nil {
		return nil, err
	}

	m := &sessionMetrics{
		ProjectDir: projectDir,
		ModDay:     fi.ModTime().Format("2006-01-02"),
	}

	var firstTs, lastTs int64
	seenUUIDs := make(map[string]bool)
	toolCallCount := 0
	actionCount := 0
	filePaths := make(map[string]bool)
	totalTokens := 0

	scanner := bufio.NewScanner(file)
	const maxBuf = 4 * 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxBuf)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// 快速跳过不可能包含目标字段的行
		if !strings.Contains(line, `"type"`) {
			continue
		}

		var event claude.JSONLLine
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		// 提取 sessionID
		if event.SessionID != "" && m.SessionID == "" {
			m.SessionID = event.SessionID
		}

		// 解析时间戳
		ts := parseTimestamp(event.Timestamp)
		if ts > 0 {
			if firstTs == 0 || ts < firstTs {
				firstTs = ts
			}
			if ts > lastTs {
				lastTs = ts
			}
		}

		if event.Message == nil {
			continue
		}

		// 提取 token 使用
		if event.Type == "assistant" && event.Message.Usage != nil {
			key := event.UUID
			if key == "" {
				key = event.Timestamp
			}
			if !seenUUIDs[key] {
				seenUUIDs[key] = true
				totalTokens += event.Message.Usage.InputTokens + event.Message.Usage.OutputTokens
			}
		}

		// 提取工具调用和文件路径
		if event.Type == "assistant" {
			actionCount++
			if blocks, ok := event.Message.Content.([]any); ok {
				for _, block := range blocks {
					blockMap, ok := block.(map[string]any)
					if !ok {
						continue
					}
					if blockMap["type"] == "tool_use" {
						toolCallCount++
						// 提取文件路径
						if input, ok := blockMap["input"].(map[string]any); ok {
							if fp, ok := input["file_path"].(string); ok && fp != "" {
								filePaths[fp] = true
							}
						}
					}
				}
			}
		}
	}

	if m.SessionID == "" || (firstTs == 0 && lastTs == 0) {
		return nil, nil
	}

	m.StartedAt = firstTs
	if lastTs > firstTs {
		m.DurationMs = lastTs - firstTs
	}
	m.ToolCalls = toolCallCount
	m.Actions = actionCount
	m.FilesChanged = len(filePaths)
	m.TokensUsed = totalTokens

	return m, nil
}

// buildReport 从会话指标构建生产力报告
func (e *Engine) buildReport(metrics []sessionMetrics, days int) *ProductivityReport {
	report := &ProductivityReport{
		Period: fmt.Sprintf("%dd", days),
	}

	if len(metrics) == 0 {
		report.WeeklyTrend = []WeeklyProductivity{}
		return report
	}

	report.SessionsTotal = len(metrics)

	// 计算日均会话数
	// 使用实际时间跨度而非固定天数，更准确
	var minDay, maxDay time.Time
	for _, m := range metrics {
		if m.StartedAt > 0 {
			t := time.UnixMilli(m.StartedAt)
			if minDay.IsZero() || t.Before(minDay) {
				minDay = t
			}
			if t.After(maxDay) {
				maxDay = t
			}
		}
	}

	spanDays := days
	if !minDay.IsZero() && !maxDay.IsZero() {
		d := int(maxDay.Sub(minDay).Hours()/24) + 1
		if d > 0 && d < spanDays {
			spanDays = d
		}
	}
	report.SessionsPerDay = float64(report.SessionsTotal) / float64(spanDays)

	// 累加指标
	var totalDuration int64
	durationCount := 0
	for _, m := range metrics {
		totalDuration += m.DurationMs
		if m.DurationMs > 0 {
			durationCount++
		}
		report.TotalToolCalls += m.ToolCalls
		report.TotalFilesChanged += m.FilesChanged
		report.TotalActions += m.Actions
	}

	if durationCount > 0 {
		report.AvgSessionDurationMs = totalDuration / int64(durationCount)
	}
	report.AvgFilesPerSession = float64(report.TotalFilesChanged) / float64(report.SessionsTotal)
	report.AvgActionsPerSession = float64(report.TotalActions) / float64(report.SessionsTotal)
	report.AvgToolCallsPerSession = float64(report.TotalToolCalls) / float64(report.SessionsTotal)

	// 按周聚合趋势
	report.WeeklyTrend = e.buildWeeklyTrend(metrics, days)

	return report
}

// buildWeeklyTrend 构建周趋势数据
func (e *Engine) buildWeeklyTrend(metrics []sessionMetrics, days int) []WeeklyProductivity {
	// 确定起始周（从 days 天前的周一开始）
	now := time.Now()
	startDate := now.AddDate(0, 0, -days)

	// 按 ISO 周分组
	weekMap := make(map[string]*WeeklyProductivity)

	for _, m := range metrics {
		var t time.Time
		if m.StartedAt > 0 {
			t = time.UnixMilli(m.StartedAt)
		} else {
			// 使用文件修改日期
			parsed, err := time.Parse("2006-01-02", m.ModDay)
			if err != nil {
				continue
			}
			t = parsed
		}

		if t.Before(startDate) {
			continue
		}

		year, week := t.ISOWeek()
		weekKey := fmt.Sprintf("%d-W%02d", year, week)

		wp, ok := weekMap[weekKey]
		if !ok {
			wp = &WeeklyProductivity{Week: weekKey}
			weekMap[weekKey] = wp
		}
		wp.Sessions++
		wp.FilesChanged += m.FilesChanged
		wp.TokensUsed += m.TokensUsed
		wp.AvgDuration += m.DurationMs
	}

	// 计算每周平均时长
	for _, wp := range weekMap {
		if wp.Sessions > 0 {
			wp.AvgDuration = wp.AvgDuration / int64(wp.Sessions)
		}
	}

	// 排序（按周标识）
	keys := make([]string, 0, len(weekMap))
	for k := range weekMap {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	result := make([]WeeklyProductivity, 0, len(keys))
	for _, k := range keys {
		result = append(result, *weekMap[k])
	}

	return result
}

// parseTimestamp 解析时间戳，支持 RFC3339 和 Unix 纳秒
func parseTimestamp(s string) int64 {
	if s == "" {
		return 0
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UnixMilli()
	}
	var ns int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			ns = ns*10 + int64(c-'0')
		} else {
			return 0
		}
	}
	if ns > 0 {
		if ns > 1e12 {
			return ns / 1e6
		}
		return ns
	}
	return 0
}
