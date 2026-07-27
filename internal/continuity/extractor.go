package continuity

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"argus-desktop/internal/session"
)

// Extractor 从会话中提取任务信息
type Extractor struct{}

// NewExtractor 创建新的提取器
func NewExtractor() *Extractor {
	return &Extractor{}
}

// filePatterns 匹配文件路径的正则
var filePathPatterns = []*regexp.Regexp{
	regexp.MustCompile(`[\w\-]+(?:/[\w\-]+)*\.[a-zA-Z]{1,10}`), // 文件路径，如 src/utils/helper.go
}

// taskTypeKeywords 任务类型关键词映射
var taskTypeKeywords = map[string][]string{
	"feature":    {"添加", "新增", "实现", "创建", "add", "implement", "create", "build", "develop"},
	"bugfix":     {"修复", "修正", "解决", "fix", "repair", "resolve", "debug", "patch"},
	"refactor":   {"重构", "优化", "清理", "refactor", "optimize", "cleanup", "reorganize"},
	"docs":       {"文档", "注释", "说明", "document", "comment", "readme", "doc"},
	"test":       {"测试", "用例", "验证", "test", "spec", "assert", "verify"},
	"config":     {"配置", "设置", "环境", "config", "setting", "env", "setup"},
	"dependency": {"依赖", "升级", "安装", "depend", "upgrade", "install", "package"},
}

// decisionKeywords 决策相关的关键词
var decisionKeywords = []string{
	// 中文关键词
	"决定", "选择", "方案", "决定用", "最终",
	"采用", "选用", "确定用", "最终选择",
	"技术选型", "架构", "设计模式", "最佳实践",
	"权衡", "对比", "评估", "分析后",
	// 英文关键词
	"decide", "choose", "decision", "approach", "strategy",
	"adopt", "select", "go with", "settle on",
	"convention", "standard", "pattern", "architecture",
	"trade-off", "comparison", "evaluation",
}

// issueKeywords 已知问题/陷阱的关键词
var issueKeywords = []string{
	"注意", "警告", "陷阱", "坑", "避免", "不要", "不能",
	"warn", "caution", "pitfall", "avoid", "don't", "must not", "caveat",
	"issue", "problem", "bug", "workaround",
}

// ExtractTasks 从多个会话中提取任务
func (e *Extractor) ExtractTasks(sessions []*session.Session) []PromptAnalysis {
	var analyses []PromptAnalysis

	for _, sess := range sessions {
		// 提取所有用户 prompt
		prompts := e.extractPrompts(sess)
		analyses = append(analyses, prompts...)
	}

	return analyses
}

// extractPrompts 从单个会话中提取用户 prompt
func (e *Extractor) extractPrompts(sess *session.Session) []PromptAnalysis {
	var analyses []PromptAnalysis
	seen := make(map[string]bool)

	for _, msg := range sess.Messages {
		if msg.Type != session.MessageTypeUser {
			continue
		}

		for _, block := range msg.Content {
			if block.Type != session.ContentTypeText || block.Text == "" {
				continue
			}

			text := strings.TrimSpace(block.Text)
			if text == "" || seen[text] {
				continue
			}
			seen[text] = true

			// 跳过太短的 prompt（通常是确认性回复）
			if len(text) < 5 {
				continue
			}

			// 跳过系统消息模式（以 [ 开头的通常是工具结果回显）
			if strings.HasPrefix(text, "[") {
				continue
			}

			analyses = append(analyses, PromptAnalysis{
				Text:        text,
				Timestamp:   msg.Timestamp,
				SessionID:   sess.ID,
				TaskType:    classifyTaskType(text),
				HasFilePath: containsFilePath(text),
			})
		}
	}

	return analyses
}

// ClassifyTaskType 分类任务类型
func classifyTaskType(text string) string {
	lower := strings.ToLower(text)
	for taskType, keywords := range taskTypeKeywords {
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				return taskType
			}
		}
	}
	return "general"
}

// containsFilePath 检查文本是否包含文件路径
func containsFilePath(text string) bool {
	for _, re := range filePathPatterns {
		if re.MatchString(text) {
			return true
		}
	}
	return false
}

// ExtractCompletedTasks 从会话的 actions 中提取已完成的任务
// 支持多任务提取：按时间窗口分组actions，每个组视为一个独立任务
func (e *Extractor) ExtractCompletedTasks(sessions []*session.Session) []CompletedTask {
	var tasks []CompletedTask

	for _, sess := range sessions {
		if len(sess.Actions) == 0 {
			continue
		}

		// 按时间窗口分组actions（5分钟窗口）
		actionGroups := groupActionsByTimeWindow(sess.Actions, 5*time.Minute)

		for _, group := range actionGroups {
			// 从每组中提取文件操作
			fileActions := make(map[string][]session.Action)
			for _, action := range group {
				if action.FilePath != "" {
					fileActions[action.FilePath] = append(fileActions[action.FilePath], action)
				}
			}

			// 收集该组涉及的文件
			var filesChanged []string
			for filePath := range fileActions {
				filesChanged = append(filesChanged, filePath)
			}

			// 推断任务描述
			taskDesc := e.inferTaskDescriptionFromGroup(group, sess)

			if taskDesc != "" && len(filesChanged) > 0 {
				tasks = append(tasks, CompletedTask{
					Description:  taskDesc,
					SessionID:    sess.ID,
					FilesChanged: filesChanged,
					Timestamp:    group[0].Timestamp,
				})
			}
		}

		// 如果分组后没有提取到任务，回退到原有逻辑
		if len(tasks) == 0 || (len(tasks) > 0 && tasks[len(tasks)-1].SessionID != sess.ID) {
			taskDesc := e.inferTaskDescription(sess)
			if taskDesc != "" {
				var filesChanged []string
				seen := make(map[string]bool)
				for _, action := range sess.Actions {
					if action.FilePath != "" && !seen[action.FilePath] {
						filesChanged = append(filesChanged, action.FilePath)
						seen[action.FilePath] = true
					}
				}
				if len(filesChanged) > 0 {
					tasks = append(tasks, CompletedTask{
						Description:  taskDesc,
						SessionID:    sess.ID,
						FilesChanged: filesChanged,
						Timestamp:    sess.StartedAt,
					})
				}
			}
		}
	}

	return tasks
}

// groupActionsByTimeWindow 按时间窗口分组actions
func groupActionsByTimeWindow(actions []session.Action, window time.Duration) [][]session.Action {
	if len(actions) == 0 {
		return nil
	}

	// 按时间排序
	sorted := make([]session.Action, len(actions))
	copy(sorted, actions)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Timestamp.Before(sorted[j].Timestamp)
	})

	var groups [][]session.Action
	currentGroup := []session.Action{sorted[0]}

	for i := 1; i < len(sorted); i++ {
		// 如果时间间隔超过窗口，开始新组
		if sorted[i].Timestamp.Sub(sorted[i-1].Timestamp) > window {
			groups = append(groups, currentGroup)
			currentGroup = []session.Action{sorted[i]}
		} else {
			currentGroup = append(currentGroup, sorted[i])
		}
	}
	groups = append(groups, currentGroup)

	return groups
}

// inferTaskDescriptionFromGroup 从action组中推断任务描述
func (e *Extractor) inferTaskDescriptionFromGroup(group []session.Action, sess *session.Session) string {
	// 优先使用session.Prompt（如果时间匹配）
	if sess.Prompt != "" {
		// 提取前3行作为描述
		lines := strings.SplitN(sess.Prompt, "\n", 4)
		var promptLines []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				promptLines = append(promptLines, trimmed)
			}
			if len(promptLines) >= 3 {
				break
			}
		}
		if len(promptLines) > 0 {
			desc := strings.Join(promptLines, " ")
			desc = truncateUTF8(desc, 400)
			return desc
		}
	}

	// 从action的Description中提取
	for _, action := range group {
		if action.Description != "" {
			desc := strings.TrimSpace(action.Description)
			if len(desc) > 10 {
				desc = truncateUTF8(desc, 400)
				return desc
			}
		}
	}

	// 从用户消息中查找包含文件路径的句子
	for _, msg := range sess.Messages {
		if msg.Type != session.MessageTypeUser {
			continue
		}
		for _, block := range msg.Content {
			if block.Type != session.ContentTypeText || block.Text == "" {
				continue
			}
			// 查找包含文件路径的句子
			sentences := strings.Split(block.Text, "。")
			for _, sentence := range sentences {
				sentence = strings.TrimSpace(sentence)
				if len(sentence) > 10 && containsFilePath(sentence) {
					sentence = truncateUTF8(sentence, 400)
					return sentence
				}
			}
		}
	}

	// 回退到第一个有内容的用户消息
	for _, msg := range sess.Messages {
		if msg.Type != session.MessageTypeUser {
			continue
		}
		for _, block := range msg.Content {
			if block.Type == session.ContentTypeText && block.Text != "" {
				text := strings.TrimSpace(block.Text)
				if len(text) > 10 {
					// 提取前3行作为描述
					lines := strings.SplitN(text, "\n", 4)
					var promptLines []string
					for _, line := range lines {
						trimmed := strings.TrimSpace(line)
						if trimmed != "" {
							promptLines = append(promptLines, trimmed)
						}
						if len(promptLines) >= 3 {
							break
						}
					}
					if len(promptLines) > 0 {
						desc := strings.Join(promptLines, " ")
						desc = truncateUTF8(desc, 400)
						return desc
					}
				}
			}
		}
	}

	return ""
}

// inferTaskDescription 从会话中推断任务描述
func (e *Extractor) inferTaskDescription(sess *session.Session) string {
	// 优先使用用户 prompt
	if sess.Prompt != "" {
		// 提取前3行作为描述
		lines := strings.SplitN(sess.Prompt, "\n", 4)
		var promptLines []string
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				promptLines = append(promptLines, trimmed)
			}
			if len(promptLines) >= 3 {
				break
			}
		}
		if len(promptLines) > 0 {
			desc := strings.Join(promptLines, " ")
			desc = truncateUTF8(desc, 400)
			return desc
		}
	}

	// 回退到第一个有内容的用户消息
	for _, msg := range sess.Messages {
		if msg.Type != session.MessageTypeUser {
			continue
		}
		for _, block := range msg.Content {
			if block.Type == session.ContentTypeText && block.Text != "" {
				text := strings.TrimSpace(block.Text)
				if len(text) > 10 {
					// 提取前3行作为描述
					lines := strings.SplitN(text, "\n", 4)
					var promptLines []string
					for _, line := range lines {
						trimmed := strings.TrimSpace(line)
						if trimmed != "" {
							promptLines = append(promptLines, trimmed)
						}
						if len(promptLines) >= 3 {
							break
						}
					}
					if len(promptLines) > 0 {
						desc := strings.Join(promptLines, " ")
						desc = truncateUTF8(desc, 400)
						return desc
					}
				}
			}
		}
	}

	return ""
}

// ExtractDecisions 从会话中提取关键决策
// 优化：只从每个会话的最后几条assistant消息中提取，避免重复
func (e *Extractor) ExtractDecisions(sessions []*session.Session) []Decision {
	var decisions []Decision

	for _, sess := range sessions {
		// 收集所有assistant消息
		var assistantMsgs []struct {
			msg session.Message
			idx int
		}
		for i, msg := range sess.Messages {
			if msg.Type == session.MessageTypeAssistant {
				assistantMsgs = append(assistantMsgs, struct {
					msg session.Message
					idx int
				}{msg, i})
			}
		}

		if len(assistantMsgs) == 0 {
			continue
		}

		// 只从最后3条assistant消息中提取决策
		startIdx := 0
		if len(assistantMsgs) > 3 {
			startIdx = len(assistantMsgs) - 3
		}

		seenDecisions := make(map[string]bool)
		for _, am := range assistantMsgs[startIdx:] {
			for _, block := range am.msg.Content {
				if block.Type != session.ContentTypeText {
					continue
				}

				// 在 assistant 消息中查找决策关键词
				if containsAnyKeyword(block.Text, decisionKeywords) {
					// 提取最终结论（跳过讨论过程）
					decision := extractFinalDecision(block.Text)
					if decision != "" && !seenDecisions[decision] {
						seenDecisions[decision] = true
						decisions = append(decisions, Decision{
							Description: decision,
							Context:     truncate(block.Text, 300),
							Timestamp:   am.msg.Timestamp,
							SessionID:   sess.ID,
						})
					}
				}
			}
		}
	}

	return decisions
}

// extractFinalDecision 从文本中提取最终决策结论
// 优先提取包含"最终"、"决定用"、"选择"等确定性词汇的句子
func extractFinalDecision(text string) string {
	sentences := strings.Split(text, "。")

	// 优先级1：包含确定性词汇的句子
	finalKeywords := []string{
		"最终决定", "决定用", "选择使用", "采用", "选用", "确定用",
		"最终选择", "经评估后", "综合考虑后", "经过对比",
		"decided to", "chose to", "will use", "selected",
	}

	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if len(s) < 10 || len(s) > 300 {
			continue
		}
		lower := strings.ToLower(s)
		for _, kw := range finalKeywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				return truncateUTF8(s, 300)
			}
		}
	}

	// 优先级2：包含决策关键词且长度适中的句子
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if len(s) < 15 || len(s) > 300 {
			continue
		}
		lower := strings.ToLower(s)
		for _, kw := range decisionKeywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				// 排除包含代码、markdown格式的内容
				if !containsCodeOrMarkdown(s) {
					return truncateUTF8(s, 300)
				}
			}
		}
	}

	return ""
}

// containsCodeOrMarkdown 检查文本是否包含代码或markdown格式
func containsCodeOrMarkdown(text string) bool {
	codeIndicators := []string{
		"```", "func ", "var ", "import ", "package ",
		"if ", "for ", "switch ", "case ",
		"//", "/*", "*/",
	}
	for _, indicator := range codeIndicators {
		if strings.Contains(text, indicator) {
			return true
		}
	}
	return false
}

// ExtractKnownIssues 从会话中提取已知问题
func (e *Extractor) ExtractKnownIssues(sessions []*session.Session) []string {
	issueSet := make(map[string]bool)
	var issues []string

	for _, sess := range sessions {
		for _, msg := range sess.Messages {
			// 检查 assistant 消息
			if msg.Type == session.MessageTypeAssistant {
				for _, block := range msg.Content {
					if block.Type == session.ContentTypeText && containsAnyKeyword(block.Text, issueKeywords) {
						sentences := extractRelevantSentences(block.Text, issueKeywords)
						for _, s := range sentences {
							if !issueSet[s] {
								issueSet[s] = true
								issues = append(issues, s)
							}
						}
					}
				}
			}

			// 也检查用户消息中的注意事项
			if msg.Type == session.MessageTypeUser {
				for _, block := range msg.Content {
					if block.Type == session.ContentTypeText && containsAnyKeyword(block.Text, issueKeywords) {
						sentences := extractRelevantSentences(block.Text, issueKeywords)
						for _, s := range sentences {
							if !issueSet[s] {
								issueSet[s] = true
								issues = append(issues, s)
							}
						}
					}
				}
			}
		}
	}

	return issues
}

// ExtractPendingTasks 从会话中提取可能的待办任务
// 优化：过滤模板文本、代码块、占位符等无效内容
func (e *Extractor) ExtractPendingTasks(sessions []*session.Session) []PendingTask {
	var tasks []PendingTask
	seen := make(map[string]bool)

	for _, sess := range sessions {
		// 从用户 prompt 中检测 TODO/FIXME/HACK 等模式
		for _, msg := range sess.Messages {
			if msg.Type != session.MessageTypeUser {
				continue
			}
			for _, block := range msg.Content {
				if block.Type != session.ContentTypeText {
					continue
				}
				text := block.Text
				if containsTODOPattern(text) {
					desc := extractTODODescription(text)
					// 过滤无效内容
					if desc != "" && !seen[desc] && isValidPendingTask(desc) {
						seen[desc] = true
						tasks = append(tasks, PendingTask{
							Description: desc,
							Source:      "user_prompt",
							SessionID:   sess.ID,
						})
					}
				}
			}
		}

		// 检测 assistant 消息中的未完成承诺
		for _, msg := range sess.Messages {
			if msg.Type != session.MessageTypeAssistant {
				continue
			}
			for _, block := range msg.Content {
				if block.Type != session.ContentTypeText {
					continue
				}
				if containsUnfinishedPromise(block.Text) {
					desc := extractPromiseDescription(block.Text)
					// 过滤无效内容
					if desc != "" && !seen[desc] && isValidPendingTask(desc) {
						seen[desc] = true
						tasks = append(tasks, PendingTask{
							Description: desc,
							Source:      "assistant_promise",
							SessionID:   sess.ID,
						})
					}
				}
			}
		}
	}

	return tasks
}

// isValidPendingTask 验证待办任务描述是否有效
func isValidPendingTask(desc string) bool {
	// 过滤太短的内容
	if len(desc) < 5 {
		return false
	}

	// 过滤模板文本和占位符
	invalidPatterns := []string{
		"待办任务（用户的未完成需求）",
		"待办事项",
		"TODO",
		"FIXME",
		"### 下一步",
		"### 待办",
		"## TODO",
		"## FIXME",
		"用户消息",
		"AI回复",
	}

	lower := strings.ToLower(desc)
	for _, pattern := range invalidPatterns {
		if strings.Contains(lower, strings.ToLower(pattern)) {
			return false
		}
	}

	// 过滤包含代码的内容
	if containsCodeOrMarkdown(desc) {
		return false
	}

	// 过滤纯数字或标点
	trimmed := strings.TrimSpace(desc)
	if len(trimmed) == 0 {
		return false
	}
	// 检查是否大部分是标点符号
	punctCount := 0
	for _, r := range trimmed {
		if strings.ContainsRune(".,;:!?()[]{}`'\"-–—/#*_", r) {
			punctCount++
		}
	}
	if float64(punctCount)/float64(len([]rune(trimmed))) > 0.5 {
		return false
	}

	return true
}

// BuildFileSummaries 构建文件概览
func (e *Extractor) BuildFileSummaries(sessions []*session.Session) []FileSummary {
	fileMap := make(map[string]*FileSummary)

	for _, sess := range sessions {
		for _, action := range sess.Actions {
			if action.FilePath == "" {
				continue
			}

			fs, ok := fileMap[action.FilePath]
			if !ok {
				fs = &FileSummary{
					Path:         action.FilePath,
					IsTestFile:   isTestFile(action.FilePath),
					IsConfigFile: isConfigFile(action.FilePath),
				}
				fileMap[action.FilePath] = fs
			}

			fs.ChangeCount++
			fs.ActionCount++
			fs.LastAction = string(action.Type)
		}
	}

	// 转换为切片并按操作次数排序
	summaries := make([]FileSummary, 0, len(fileMap))
	for _, fs := range fileMap {
		summaries = append(summaries, *fs)
	}

	// 使用标准库排序，O(n log n)，按 ActionCount 降序
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].ActionCount > summaries[j].ActionCount
	})

	return summaries
}

// 辅助函数

func containsAnyKeyword(text string, keywords []string) bool {
	lower := strings.ToLower(text)
	for _, kw := range keywords {
		if strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

func extractRelevantSentences(text string, keywords []string) []string {
	sentences := strings.Split(text, "。")
	sentences = append(sentences, strings.Split(text, ". ")...)

	var relevant []string
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if s == "" || len(s) < 10 {
			continue
		}
		lower := strings.ToLower(s)
		for _, kw := range keywords {
			if strings.Contains(lower, strings.ToLower(kw)) {
				s = truncateUTF8(s, 400)
				relevant = append(relevant, s)
				break
			}
		}
	}

	return relevant
}

func truncate(text string, maxLen int) string {
	return truncateUTF8(text, maxLen)
}

// truncateUTF8 安全截断字符串，按 rune 而非字节截断
func truncateUTF8(text string, maxLen int) string {
	if utf8.RuneCountInString(text) <= maxLen {
		return text
	}
	runes := []rune(text)
	return string(runes[:maxLen]) + "..."
}

func containsTODOPattern(text string) bool {
	lower := strings.ToLower(text)

	// 显式TODO标记
	explicitPatterns := []string{"todo", "fixme", "hack", "xxx"}
	for _, p := range explicitPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}

	// 中文自然语言模式
	chinesePatterns := []string{
		"需要完成", "待完成", "后续需要", "还需要", "待处理",
		"需要实现", "需要优化", "需要修复", "需要添加", "需要处理",
		"待实现", "待优化", "待修复", "待添加", "待处理",
		"还没有", "尚未", "未完成", "未实现",
	}
	for _, p := range chinesePatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}

	// 英文自然语言模式
	englishPatterns := []string{
		"need to", "should", "must", "have to",
		"pending", "remaining", "unfinished", "incomplete",
	}
	for _, p := range englishPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}

	return false
}

func extractTODODescription(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))

		// 显式TODO标记
		if strings.Contains(lower, "todo") || strings.Contains(lower, "fixme") || strings.Contains(lower, "hack") {
			line = strings.TrimSpace(line)
			line = truncateUTF8(line, 400)
			return line
		}

		// 中文自然语言模式
		chinesePatterns := []string{
			"待完成", "需要完成", "还需要", "待处理",
			"需要实现", "需要优化", "需要修复", "需要添加",
			"待实现", "待优化", "待修复", "待添加",
			"还没有", "尚未", "未完成", "未实现",
		}
		for _, p := range chinesePatterns {
			if strings.Contains(lower, p) {
				line = strings.TrimSpace(line)
				line = truncateUTF8(line, 400)
				return line
			}
		}

		// 英文自然语言模式
		englishPatterns := []string{
			"need to", "should", "must", "have to",
			"pending", "remaining", "unfinished",
		}
		for _, p := range englishPatterns {
			if strings.Contains(lower, p) {
				line = strings.TrimSpace(line)
				line = truncateUTF8(line, 400)
				return line
			}
		}
	}
	return ""
}

func containsUnfinishedPromise(text string) bool {
	lower := strings.ToLower(text)
	patterns := []string{"接下来", "下一步", "然后我会", "随后", "之后", "next, i will", "then i'll", "will then", "next step"}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func extractPromiseDescription(text string) string {
	lines := strings.Split(text, "\n")
	for _, line := range lines {
		lower := strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(lower, "下一步") || strings.Contains(lower, "接下来") || strings.Contains(lower, "then i") || strings.Contains(lower, "next step") {
			line = strings.TrimSpace(line)
			line = truncateUTF8(line, 400)
			return line
		}
	}
	return ""
}

func isTestFile(path string) bool {
	lower := strings.ToLower(path)
	return strings.Contains(lower, "_test") ||
		strings.Contains(lower, ".test.") ||
		strings.Contains(lower, "_spec") ||
		strings.Contains(lower, ".spec.") ||
		strings.Contains(lower, "/test/") ||
		strings.Contains(lower, "/tests/") ||
		strings.Contains(lower, "/__tests__/")
}

func isConfigFile(path string) bool {
	lower := strings.ToLower(path)
	configPatterns := []string{
		".env", "config.", "setting.", ".json", ".yaml", ".yml", ".toml",
		"dockerfile", "docker-compose", "makefile", ".gitignore",
		"go.mod", "go.sum", "package.json", "package-lock.json",
		"tsconfig", "webpack", "vite.config", ".eslint", ".prettier",
	}
	for _, p := range configPatterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// DeduplicateTasks 去重任务列表（保留原有函数，向后兼容）
func DeduplicateTasks(tasks []CompletedTask) []CompletedTask {
	return DeduplicateTasksAdvanced(tasks, 0.6)
}

// DeduplicateTasksAdvanced 基于相似度的智能去重
func DeduplicateTasksAdvanced(tasks []CompletedTask, threshold float64) []CompletedTask {
	if len(tasks) <= 1 {
		return tasks
	}

	var result []CompletedTask
	used := make(map[int]bool)

	for i := 0; i < len(tasks); i++ {
		if used[i] {
			continue
		}

		// 保留第一个，标记相似的为已使用
		result = append(result, tasks[i])

		for j := i + 1; j < len(tasks); j++ {
			if used[j] {
				continue
			}

			// 计算相似度
			similarity := calculateTaskSimilarity(tasks[i], tasks[j])
			if similarity > threshold {
				used[j] = true
			}
		}
	}

	return result
}

// calculateTaskSimilarity 计算两个任务的相似度
func calculateTaskSimilarity(a, b CompletedTask) float64 {
	// 描述相似度（权重0.7）
	descSim := calculateStringSimilarity(a.Description, b.Description)

	// 文件重叠度（权重0.3）
	filesSim := calculateFilesSimilarity(a.FilesChanged, b.FilesChanged)

	// 如果描述高度相似，即使文件不同也认为是同一任务
	if descSim > 0.8 {
		return descSim*0.7 + filesSim*0.3
	}

	// 如果文件完全相同，即使描述不同也可能相关
	if filesSim > 0.9 {
		return descSim*0.7 + filesSim*0.3
	}

	// 综合评分
	return descSim*0.7 + filesSim*0.3
}

// calculateStringSimilarity 计算两个字符串的相似度（Jaccard系数）
func calculateStringSimilarity(a, b string) float64 {
	setA := toWordSet(a)
	setB := toWordSet(b)

	if len(setA) == 0 || len(setB) == 0 {
		return 0
	}

	intersection := 0
	for word := range setA {
		if setB[word] {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

// toWordSet 将文本转换为单词集合
func toWordSet(text string) map[string]bool {
	words := make(map[string]bool)
	// 简单分词：按空格和标点分割
	tokens := regexp.MustCompile(`[\s\p{P}]+`).Split(strings.ToLower(text), -1)
	for _, token := range tokens {
		if len(token) > 1 {
			words[token] = true
		}
	}
	return words
}

// calculateFilesSimilarity 计算文件列表的相似度
func calculateFilesSimilarity(a, b []string) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0
	}

	setA := make(map[string]bool)
	for _, f := range a {
		setA[filepath.Base(f)] = true
	}

	setB := make(map[string]bool)
	for _, f := range b {
		setB[filepath.Base(f)] = true
	}

	intersection := 0
	for f := range setA {
		if setB[f] {
			intersection++
		}
	}

	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}

	return float64(intersection) / float64(union)
}

// DeduplicateDecisions 去重决策列表（基于相似度）
func DeduplicateDecisions(decisions []Decision) []Decision {
	if len(decisions) <= 1 {
		return decisions
	}

	var result []Decision
	used := make(map[int]bool)

	for i := 0; i < len(decisions); i++ {
		if used[i] {
			continue
		}
		result = append(result, decisions[i])

		for j := i + 1; j < len(decisions); j++ {
			if used[j] {
				continue
			}
			similarity := calculateStringSimilarity(decisions[i].Description, decisions[j].Description)
			if similarity > 0.5 {
				used[j] = true
			}
		}
	}

	return result
}

// DeduplicateIssues 去重已知问题（基于相似度）
func DeduplicateIssues(issues []string) []string {
	if len(issues) <= 1 {
		return issues
	}

	var result []string
	used := make(map[int]bool)

	for i := 0; i < len(issues); i++ {
		if used[i] {
			continue
		}
		result = append(result, issues[i])

		for j := i + 1; j < len(issues); j++ {
			if used[j] {
				continue
			}
			similarity := calculateStringSimilarity(issues[i], issues[j])
			if similarity > 0.5 {
				used[j] = true
			}
		}
	}

	return result
}

// filterSessionsByProject 按项目过滤会话
func FilterSessionsByProject(sessions []*session.Session, projectDir string) []*session.Session {
	var filtered []*session.Session
	for _, sess := range sessions {
		if strings.Contains(sess.CWD, projectDir) || projectDir == "" {
			filtered = append(filtered, sess)
		}
	}
	return filtered
}

// sortSessionsByTime 按时间排序会话（最新的在前）
func SortSessionsByTime(sessions []*session.Session) {
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartedAt.After(sessions[j].StartedAt)
	})
}

// FilterRecentSessions 只保留最近 N 个会话
func FilterRecentSessions(sessions []*session.Session, count int) []*session.Session {
	if count <= 0 || count >= len(sessions) {
		return sessions
	}
	return sessions[:count]
}

// filterByTimeRange 按时间范围过滤会话
func FilterByTimeRange(sessions []*session.Session, since time.Time) []*session.Session {
	var filtered []*session.Session
	for _, sess := range sessions {
		if sess.StartedAt.After(since) || sess.StartedAt.Equal(since) {
			filtered = append(filtered, sess)
		}
	}
	return filtered
}

// ExtractSessionSummary 从会话中提取核心内容摘要（增强版）
// 提取用户需求、AI执行操作、文件变更、技术决策、完成状态等多维度信息
func ExtractSessionSummary(sessions []*session.Session) string {
	if len(sessions) == 0 {
		return ""
	}

	var sessionSummaries []string

	for _, sess := range sessions {
		summary := extractSingleSessionSummary(sess)
		if summary != "" {
			sessionSummaries = append(sessionSummaries, summary)
		}
	}

	if len(sessionSummaries) == 0 {
		return "暂无会话摘要"
	}

	// 限制会话数量
	if len(sessionSummaries) > 5 {
		sessionSummaries = sessionSummaries[:5]
	}

	return strings.Join(sessionSummaries, "\n\n")
}

// extractSingleSessionSummary 提取单个会话的完整摘要
func extractSingleSessionSummary(sess *session.Session) string {
	var parts []string

	// 1. 提取核心任务描述
	taskDesc := extractTaskDescription(sess)
	if taskDesc != "" {
		parts = append(parts, "**任务**: "+taskDesc)
	}

	// 2. 提取完成状态
	completionStatus := extractCompletionStatus(sess)
	if completionStatus != "" {
		parts = append(parts, "**状态**: "+completionStatus)
	}

	// 3. 提取文件变更摘要
	fileSummary := extractFileChangeSummary(sess)
	if fileSummary != "" {
		parts = append(parts, "**文件变更**: "+fileSummary)
	}

	// 4. 提取技术决策
	decision := extractKeyDecision(sess)
	if decision != "" {
		parts = append(parts, "**技术决策**: "+decision)
	}

	// 5. 提取遇到的问题
	issue := extractKeyIssue(sess)
	if issue != "" {
		parts = append(parts, "**遇到问题**: "+issue)
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, "\n")
}

// extractTaskDescription 提取任务描述（从用户 prompt 和消息中）
func extractTaskDescription(sess *session.Session) string {
	// 优先使用 sess.Prompt
	if sess.Prompt != "" {
		desc := truncateUTF8(strings.TrimSpace(sess.Prompt), 200)
		if len(desc) > 10 {
			return desc
		}
	}

	// 从用户消息中提取
	for _, msg := range sess.Messages {
		if msg.Type != session.MessageTypeUser {
			continue
		}
		for _, block := range msg.Content {
			if block.Type != session.ContentTypeText && block.Type != session.ContentTypeToolResult {
				continue
			}
			text := strings.TrimSpace(block.Text)
			if text == "" || len(text) < 10 {
				continue
			}
			// 跳过系统消息
			if strings.HasPrefix(text, "[") {
				continue
			}
			// 提取第一行作为任务描述
			lines := strings.SplitN(text, "\n", 2)
			desc := strings.TrimSpace(lines[0])
			if len(desc) > 10 {
				return truncateUTF8(desc, 200)
			}
		}
	}

	return ""
}

// extractCompletionStatus 提取任务完成状态
func extractCompletionStatus(sess *session.Session) string {
	// 从 assistant 最后几条消息中提取完成状态
	var assistantMsgs []session.Message
	for _, msg := range sess.Messages {
		if msg.Type == session.MessageTypeAssistant {
			assistantMsgs = append(assistantMsgs, msg)
		}
	}

	if len(assistantMsgs) == 0 {
		return "进行中"
	}

	// 检查最后几条 assistant 消息
	checkCount := 3
	if len(assistantMsgs) < checkCount {
		checkCount = len(assistantMsgs)
	}

	for i := len(assistantMsgs) - checkCount; i < len(assistantMsgs); i++ {
		msg := assistantMsgs[i]
		for _, block := range msg.Content {
			if block.Type != session.ContentTypeText {
				continue
			}
			text := strings.ToLower(block.Text)

			// 检查完成状态关键词
			if containsCompletionKeywords(text) {
				return extractCompletionPhrase(block.Text)
			}
		}
	}

	// 检查是否有文件变更
	if len(sess.Actions) > 0 {
		return "已完成操作"
	}

	return "进行中"
}

// containsCompletionKeywords 检查是否包含完成状态关键词
func containsCompletionKeywords(text string) bool {
	successPatterns := []string{
		"完成", "成功", "已修复", "已实现", "已添加", "已完成",
		"done", "completed", "fixed", "implemented", "resolved",
	}
	for _, p := range successPatterns {
		if strings.Contains(text, p) {
			return true
		}
	}
	return false
}

// extractCompletionPhrase 提取完成状态短语
func extractCompletionPhrase(text string) string {
	// 尝试提取包含完成关键词的句子
	sentences := strings.Split(text, "。")
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if len(s) < 5 || len(s) > 100 {
			continue
		}
		lower := strings.ToLower(s)
		if containsCompletionKeywords(lower) {
			return truncateUTF8(s, 100)
		}
	}
	return "已完成"
}

// extractFileChangeSummary 提取文件变更摘要
func extractFileChangeSummary(sess *session.Session) string {
	if len(sess.Actions) == 0 {
		return ""
	}

	// 统计文件操作
	fileActions := make(map[string]int)
	for _, action := range sess.Actions {
		if action.FilePath != "" {
			fileActions[action.FilePath]++
		}
	}

	if len(fileActions) == 0 {
		return ""
	}

	// 按操作次数排序
	type fileInfo struct {
		path  string
		count int
	}
	var files []fileInfo
	for path, count := range fileActions {
		files = append(files, fileInfo{path, count})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].count > files[j].count
	})

	// 生成摘要
	var parts []string
	maxFiles := 5
	if len(files) < maxFiles {
		maxFiles = len(files)
	}

	for i := 0; i < maxFiles; i++ {
		f := files[i]
		// 只显示文件名
		fileName := filepath.Base(f.path)
		if f.count > 1 {
			parts = append(parts, fmt.Sprintf("%s(x%d)", fileName, f.count))
		} else {
			parts = append(parts, fileName)
		}
	}

	if len(files) > maxFiles {
		parts = append(parts, fmt.Sprintf("等%d个文件", len(files)))
	}

	return strings.Join(parts, ", ")
}

// extractKeyDecision 提取关键技术决策
func extractKeyDecision(sess *session.Session) string {
	// 从 assistant 消息中提取决策
	for _, msg := range sess.Messages {
		if msg.Type != session.MessageTypeAssistant {
			continue
		}
		for _, block := range msg.Content {
			if block.Type != session.ContentTypeText {
				continue
			}
			// 查找决策关键词
			if containsAnyKeyword(block.Text, decisionKeywords) {
				decision := extractFinalDecision(block.Text)
				if decision != "" {
					return truncateUTF8(decision, 150)
				}
			}
		}
	}
	return ""
}

// extractKeyIssue 提取关键问题
func extractKeyIssue(sess *session.Session) string {
	// 从 assistant 消息中提取问题
	for _, msg := range sess.Messages {
		if msg.Type != session.MessageTypeAssistant {
			continue
		}
		for _, block := range msg.Content {
			if block.Type != session.ContentTypeText {
				continue
			}
			// 查找问题关键词
			if containsAnyKeyword(block.Text, issueKeywords) {
				sentences := extractRelevantSentences(block.Text, issueKeywords)
				if len(sentences) > 0 {
					return truncateUTF8(sentences[0], 150)
				}
			}
		}
	}
	return ""
}
