package hookmonitor

import (
	"bufio"
	"context"
	"crypto/sha256"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	// maxExecutions 内存中最多保留的执行记录数
	maxExecutions = 1000
	// defaultAlertThreshold 连续失败告警阈值
	defaultAlertThreshold = 3
)

// Engine 是 Hook 执行日志监控引擎。
type Engine struct {
	claudeDir      string // ~/.claude 路径
	logFilePath    string // ~/.claude/hooks/hooks.log 路径
	executions     []HookExecution
	mu             sync.RWMutex
	watcher        *fsnotify.Watcher
	watchCtx       context.Context
	watchCancel    context.CancelFunc
	watchDone      chan struct{} // watchLoop 退出时关闭
	alertThreshold int
	onAlert        func(AlertEvent)
	logger         *log.Logger
}

// NewEngine 创建新的 Hook 监控引擎。
func NewEngine(onAlert func(AlertEvent)) (*Engine, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户主目录失败: %w", err)
	}

	claudeDir := filepath.Join(homeDir, ".claude")
	logDir := filepath.Join(claudeDir, "hooks")
	logFilePath := filepath.Join(logDir, "hooks.log")

	// 确保 hooks 目录存在
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, fmt.Errorf("创建 hooks 目录失败: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	e := &Engine{
		claudeDir:      claudeDir,
		logFilePath:    logFilePath,
		executions:     make([]HookExecution, 0, 128),
		watchCtx:       ctx,
		watchCancel:    cancel,
		watchDone:      make(chan struct{}),
		alertThreshold: defaultAlertThreshold,
		onAlert:        onAlert,
		logger:         log.Default(),
	}

	// 初始化 fsnotify watcher
	if err := e.initWatcher(); err != nil {
		cancel()
		if e.watcher != nil {
			e.watcher.Close()
		}
		return nil, fmt.Errorf("初始化文件监听失败: %w", err)
	}

	// 初始扫描
	if _, err := e.ScanLogs(); err != nil {
		e.logger.Printf("WARN: 初始扫描 Hook 日志失败: %v", err)
	}

	return e, nil
}

// initWatcher 初始化 fsnotify 文件监听器。
func (e *Engine) initWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("创建 watcher 失败: %w", err)
	}
	e.watcher = watcher

	// 监听 hooks 目录
	logDir := filepath.Dir(e.logFilePath)
	if err := watcher.Add(logDir); err != nil {
		// 目录可能不存在，忽略
		e.logger.Printf("WARN: 无法监听 hooks 目录 %s: %v", logDir, err)
	}

	// 启动监听协程
	go e.watchLoop()

	return nil
}

// watchLoop 后台监听 hooks.log 文件变更。
func (e *Engine) watchLoop() {
	defer close(e.watchDone)

	debounceTimer := time.NewTimer(0)
	if !debounceTimer.Stop() {
		<-debounceTimer.C
	}

	for {
		select {
		case <-e.watchCtx.Done():
			debounceTimer.Stop()
			return
		case event, ok := <-e.watcher.Events:
			if !ok {
				return
			}
			// 只关心 hooks.log 文件的写入事件
			if filepath.Clean(event.Name) == filepath.Clean(e.logFilePath) {
				if event.Op&(fsnotify.Write|fsnotify.Create) != 0 {
					// 防抖：500ms 内的多次写入合并处理
					debounceTimer.Reset(500 * time.Millisecond)
				}
			}
		case err, ok := <-e.watcher.Errors:
			if !ok {
				return
			}
			e.logger.Printf("WARN: Hook 日志监听错误: %v", err)
		case <-debounceTimer.C:
			e.handleLogUpdate()
		}
	}
}

// handleLogUpdate 处理日志文件更新，追加新记录到内存缓冲。
func (e *Engine) handleLogUpdate() {
	newExecutions, err := e.parseLogFile()
	if err != nil {
		e.logger.Printf("WARN: 解析 Hook 日志失败: %v", err)
		return
	}

	if len(newExecutions) == 0 {
		return
	}

	e.mu.Lock()
	// 追加新记录（跳过已存在的）
	existingIDs := make(map[string]struct{}, len(e.executions))
	for _, ex := range e.executions {
		existingIDs[ex.ID] = struct{}{}
	}
	for _, ex := range newExecutions {
		if _, exists := existingIDs[ex.ID]; !exists {
			e.executions = append(e.executions, ex)
		}
	}
	// 按时间排序
	sort.Slice(e.executions, func(i, j int) bool {
		return e.executions[i].StartTime.After(e.executions[j].StartTime)
	})
	// 裁剪到最大数量
	if len(e.executions) > maxExecutions {
		e.executions = e.executions[:maxExecutions]
	}
	e.mu.Unlock()

	// 检查连续失败告警
	e.checkAlert()
}

// ScanLogs 扫描所有日志来源，返回合并后的执行记录。
func (e *Engine) ScanLogs() ([]HookExecution, error) {
	var all []HookExecution

	// 1. 解析 hooks.log 文件
	logExecs, err := e.parseLogFile()
	if err != nil {
		e.logger.Printf("WARN: 解析 hooks.log 失败: %v", err)
	} else {
		all = append(all, logExecs...)
	}

	// 2. 扫描 JSONL 会话文件中的 hook 事件
	jsonlExecs, err := e.scanJSONLHookEvents()
	if err != nil {
		e.logger.Printf("WARN: 扫描 JSONL hook 事件失败: %v", err)
	} else {
		all = append(all, jsonlExecs...)
	}

	// 去重（基于 ID）
	seen := make(map[string]struct{}, len(all))
	unique := make([]HookExecution, 0, len(all))
	for _, ex := range all {
		if _, exists := seen[ex.ID]; !exists {
			seen[ex.ID] = struct{}{}
			unique = append(unique, ex)
		}
	}

	// 按时间倒序排列
	sort.Slice(unique, func(i, j int) bool {
		return unique[i].StartTime.After(unique[j].StartTime)
	})

	// 裁剪到最大数量
	if len(unique) > maxExecutions {
		unique = unique[:maxExecutions]
	}

	// 更新内存缓冲
	e.mu.Lock()
	e.executions = unique
	e.mu.Unlock()

	return unique, nil
}

// parseLogFile 解析 ~/.claude/hooks/hooks.log 文件。
// 日志格式：[TIMESTAMP] TYPE(MATCHER) STATUS duration=Xms exit=N command="..." [stderr="..."]
func (e *Engine) parseLogFile() ([]HookExecution, error) {
	file, err := os.Open(e.logFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 文件不存在不是错误
		}
		return nil, fmt.Errorf("打开 hooks.log 失败: %w", err)
	}
	defer file.Close()

	var executions []HookExecution
	scanner := bufio.NewScanner(file)

	// 增大缓冲区以支持长行
	const maxScanTokenSize = 1024 * 1024 // 1MB
	buf := make([]byte, 0, maxScanTokenSize)
	scanner.Buffer(buf, maxScanTokenSize)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		ex, ok := e.parseLogLine(line)
		if ok {
			executions = append(executions, ex)
		}
	}

	if err := scanner.Err(); err != nil {
		return executions, fmt.Errorf("读取 hooks.log 失败: %w", err)
	}

	return executions, nil
}

// logPattern 匹配日志行格式：
// [2026-08-04T10:30:00Z] PreToolUse(Bash) SUCCESS duration=150ms exit=0 command="npm test"
var logPattern = regexp.MustCompile(
	`^\[([^\]]+)\]\s+(\w+)\(([^)]*)\)\s+(\w+)(?:\s+duration=(\d+)(ms|s))?(?:\s+exit=(-?\d+))?(?:\s+command="([^"]*)")?(?:\s+stderr="([^"]*)")?`,
)

// parseLogLine 解析单行日志。
func (e *Engine) parseLogLine(line string) (HookExecution, bool) {
	matches := logPattern.FindStringSubmatch(line)
	if matches == nil {
		return HookExecution{}, false
	}

	// 解析时间戳
	startTime, err := time.Parse(time.RFC3339, matches[1])
	if err != nil {
		// 尝试其他格式
		startTime, err = time.Parse("2006-01-02T15:04:05", matches[1])
		if err != nil {
			startTime = time.Now()
		}
	}

	hookType := matches[2]
	matcher := matches[3]
	statusStr := strings.ToUpper(matches[4])
	durationMs, _ := strconv.ParseInt(matches[5], 10, 64)
	durationUnit := matches[6]
	exitCode, _ := strconv.Atoi(matches[7])
	command := matches[8]
	stderr := matches[9]

	// 计算耗时
	var duration time.Duration
	if durationMs > 0 {
		if durationUnit == "s" {
			duration = time.Duration(durationMs) * time.Second
		} else {
			duration = time.Duration(durationMs) * time.Millisecond
		}
	}

	// 映射状态
	var status HookEventType
	switch statusStr {
	case "SUCCESS", "OK":
		status = HookEventSuccess
	case "FAILURE", "FAIL", "ERROR":
		status = HookEventFailure
	case "TIMEOUT":
		status = HookEventTimeout
	case "START":
		status = HookEventStart
	default:
		status = HookEventSuccess
	}

	// 生成稳定 ID
	id := e.generateID(hookType, matcher, command, startTime)

	return HookExecution{
		ID:        id,
		HookType:  hookType,
		Matcher:   matcher,
		Command:   command,
		StartTime: startTime,
		EndTime:   startTime.Add(duration),
		Duration:  duration,
		ExitCode:  exitCode,
		Stderr:    stderr,
		Status:    status,
		Source:    "log",
	}, true
}

// scanJSONLHookEvents 扫描 JSONL 会话文件中的 hook 相关事件。
func (e *Engine) scanJSONLHookEvents() ([]HookExecution, error) {
	projectsDir := filepath.Join(e.claudeDir, "projects")

	if _, err := os.Stat(projectsDir); os.IsNotExist(err) {
		return nil, nil
	}

	var executions []HookExecution

	err := filepath.Walk(projectsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if filepath.Ext(path) != ".jsonl" {
			return nil
		}

		jsonlExecs, err := e.parseJSONLForHookEvents(path)
		if err != nil {
			e.logger.Printf("WARN: 解析 JSONL %s 失败: %v", path, err)
			return nil // 单个文件失败不影响其他文件
		}
		executions = append(executions, jsonlExecs...)
		return nil
	})

	return executions, err
}

// parseJSONLForHookEvents 从单个 JSONL 文件中提取 hook 事件。
func (e *Engine) parseJSONLForHookEvents(filePath string) ([]HookExecution, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("打开 JSONL 文件失败: %w", err)
	}
	defer file.Close()

	// 从文件路径提取项目目录和会话 ID
	// 路径格式：~/.claude/projects/<project-hash>/<session-id>.jsonl
	dir := filepath.Base(filepath.Dir(filePath))
	sessionID := strings.TrimSuffix(filepath.Base(filePath), ".jsonl")

	var executions []HookExecution
	scanner := bufio.NewScanner(file)

	// 使用 64KB 初始缓冲区，最大 4MB，避免 JSONL 行过长导致 token too long 错误
	const maxScanTokenSize = 4 * 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxScanTokenSize)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		// 简单的 JSON 字段提取（避免完整 JSON 解析的开销）
		ex := e.extractHookEventFromJSONL(line, dir, sessionID)
		if ex != nil {
			executions = append(executions, *ex)
		}
	}

	return executions, scanner.Err()
}

// extractHookEventFromJSONL 从 JSONL 行中提取 hook 事件。
// 查找包含 "type":"hook_start" 或 "type":"hook_end" 的行。
func (e *Engine) extractHookEventFromJSONL(line, projectDir, sessionID string) *HookExecution {
	// 快速检查是否包含 hook 相关内容
	if !strings.Contains(line, `"hook"`) && !strings.Contains(line, `"hookType"`) {
		return nil
	}

	// 提取 type 字段
	hookEventType := e.extractJSONString(line, "type")
	if hookEventType == "" {
		return nil
	}

	// 只处理 hook 相关的事件类型
	var status HookEventType
	switch {
	case strings.Contains(hookEventType, "hook_start") || strings.Contains(hookEventType, "hook.start"):
		status = HookEventStart
	case strings.Contains(hookEventType, "hook_end") || strings.Contains(hookEventType, "hook.end"):
		status = HookEventSuccess
	case strings.Contains(hookEventType, "hook_error") || strings.Contains(hookEventType, "hook.error"):
		status = HookEventFailure
	default:
		return nil
	}

	hookType := e.extractJSONString(line, "hookType")
	matcher := e.extractJSONString(line, "matcher")
	command := e.extractJSONString(line, "command")
	stderr := e.extractJSONString(line, "error")

	// 提取时间戳
	timestamp := e.extractJSONString(line, "timestamp")
	startTime, err := time.Parse(time.RFC3339, timestamp)
	if err != nil {
		startTime = time.Now()
	}

	// 提取耗时
	durationMs := e.extractJSONNumber(line, "duration")
	duration := time.Duration(durationMs) * time.Millisecond

	// 提取退出码
	exitCode := e.extractJSONInt(line, "exitCode")

	id := e.generateID(hookType, matcher, command, startTime)

	return &HookExecution{
		ID:         id,
		HookType:   hookType,
		Matcher:    matcher,
		Command:    command,
		ProjectDir: projectDir,
		SessionID:  sessionID,
		StartTime:  startTime,
		EndTime:    startTime.Add(duration),
		Duration:   duration,
		ExitCode:   exitCode,
		Stderr:     stderr,
		Status:     status,
		Source:     "jsonl",
	}
}

// extractJSONString 从 JSON 行中快速提取字符串字段值。
func (e *Engine) extractJSONString(line, field string) string {
	key := `"` + field + `":`
	idx := strings.Index(line, key)
	if idx < 0 {
		return ""
	}
	rest := line[idx+len(key):]
	rest = strings.TrimSpace(rest)

	if len(rest) < 2 || rest[0] != '"' {
		return ""
	}
	// 找到结束引号（跳过转义的引号）
	end := 1
	for end < len(rest) {
		if rest[end] == '\\' && end+1 < len(rest) {
			end += 2
			continue
		}
		if rest[end] == '"' {
			break
		}
		end++
	}
	if end >= len(rest) {
		return ""
	}
	return rest[1:end]
}

// extractJSONNumber 从 JSON 行中快速提取数字字段值。
func (e *Engine) extractJSONNumber(line, field string) int64 {
	key := `"` + field + `":`
	idx := strings.Index(line, key)
	if idx < 0 {
		return 0
	}
	rest := strings.TrimSpace(line[idx+len(key):])
	end := 0
	// 支持负数
	if end < len(rest) && rest[end] == '-' {
		end++
	}
	for end < len(rest) && (rest[end] >= '0' && rest[end] <= '9') {
		end++
	}
	if end == 0 || (end == 1 && rest[0] == '-') {
		return 0
	}
	val, _ := strconv.ParseInt(rest[:end], 10, 64)
	return val
}

// extractJSONInt 从 JSON 行中快速提取整数字段值。
func (e *Engine) extractJSONInt(line, field string) int {
	return int(e.extractJSONNumber(line, field))
}

// generateID 基于内容和时间生成稳定的唯一 ID。
func (e *Engine) generateID(hookType, matcher, command string, t time.Time) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s:%s:%d", hookType, matcher, command, t.UnixNano())))
	return fmt.Sprintf("%x", h[:8])
}

// GetExecutions 返回最近的执行记录。
// limit <= 0 表示返回全部。hookType 为空表示不过滤。
func (e *Engine) GetExecutions(limit int, hookType string) []HookExecution {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []HookExecution
	for _, ex := range e.executions {
		if hookType != "" && !strings.EqualFold(ex.HookType, hookType) {
			continue
		}
		result = append(result, ex)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// GetStats 计算并返回聚合统计数据。
func (e *Engine) GetStats() *HookStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := &HookStats{
		ByHookType: make(map[string]*HookTypeStats),
	}

	if len(e.executions) == 0 {
		return stats
	}

	var totalDuration time.Duration
	successCount := 0
	failureStreak := 0
	maxStreak := 0

	// 用于计算连续失败（从最新的记录开始倒序）
	for i := range e.executions {
		ex := &e.executions[i]
		stats.TotalExecutions++
		totalDuration += ex.Duration

		// 按类型统计
		typeStats, exists := stats.ByHookType[ex.HookType]
		if !exists {
			typeStats = &HookTypeStats{}
			stats.ByHookType[ex.HookType] = typeStats
		}
		typeStats.Executions++
		typeStats.AvgDuration = (typeStats.AvgDuration*time.Duration(typeStats.Executions-1) + ex.Duration) / time.Duration(typeStats.Executions)

		if ex.Status == HookEventSuccess || ex.Status == HookEventStart {
			successCount++
			typeStats.Successes++
		} else if ex.Status == HookEventFailure || ex.Status == HookEventTimeout {
			typeStats.Failures++
		}
	}

	// 计算总体成功率
	if stats.TotalExecutions > 0 {
		stats.SuccessRate = float64(successCount) / float64(stats.TotalExecutions) * 100
		stats.AvgDuration = totalDuration / time.Duration(stats.TotalExecutions)
	}

	// 计算各类型成功率
	for _, typeStats := range stats.ByHookType {
		if typeStats.Executions > 0 {
			typeStats.SuccessRate = float64(typeStats.Successes) / float64(typeStats.Executions) * 100
		}
	}

	// 计算连续失败次数（从最新记录倒序）
	for _, ex := range e.executions {
		if ex.Status == HookEventFailure || ex.Status == HookEventTimeout {
			failureStreak++
		} else {
			break
		}
	}
	stats.FailureStreak = failureStreak
	_ = maxStreak

	// 最近一次执行
	if len(e.executions) > 0 {
		last := e.executions[0]
		stats.LastExecution = &last
	}

	return stats
}

// ClearLogs 清空内存中的执行记录。
func (e *Engine) ClearLogs() {
	e.mu.Lock()
	e.executions = make([]HookExecution, 0, 128)
	e.mu.Unlock()
}

// checkAlert 检查连续失败是否达到告警阈值。
func (e *Engine) checkAlert() {
	if e.onAlert == nil {
		return
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	// 直接计算连续失败次数，避免调用 GetStats() 导致重复加锁
	failureStreak := 0
	var lastErrors []string
	var lastHookType string
	for _, ex := range e.executions {
		if ex.Status == HookEventFailure || ex.Status == HookEventTimeout {
			failureStreak++
			if ex.Stderr != "" && len(lastErrors) < 5 {
				lastErrors = append(lastErrors, ex.Stderr)
			}
			lastHookType = ex.HookType
		} else {
			break
		}
	}

	if failureStreak >= e.alertThreshold {
		e.onAlert(AlertEvent{
			Type:       "consecutive_failure",
			Count:      failureStreak,
			HookType:   lastHookType,
			LastErrors: lastErrors,
		})
	}
}

// Close 关闭引擎，释放资源。
func (e *Engine) Close() {
	e.watchCancel()
	// 等待 watchLoop goroutine 退出
	<-e.watchDone
	if e.watcher != nil {
		e.watcher.Close()
	}
}

// SetAlertThreshold 设置连续失败告警阈值。
func (e *Engine) SetAlertThreshold(threshold int) {
	if threshold > 0 {
		e.alertThreshold = threshold
	}
}
