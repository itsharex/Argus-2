package continuity

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"argus-desktop/internal/common"
	"argus-desktop/internal/llm"
	"argus-desktop/internal/session"
	"argus-desktop/internal/session/claude"
)

// Engine 会话连续性引擎
type Engine struct {
	extractor   *Extractor
	validator   *Validator
	handoffGen  *HandoffGenerator
	homeDir     string
	llmEnhancer *LLMEnhancer // 可选：LLM 增强器
	llmEnabled  bool
}

// NewEngine 创建新的连续性引擎。cfg 为 nil 表示不启用 LLM 增强。
func NewEngine(cfg *llm.ProviderConfig) (*Engine, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户目录失败: %w", err)
	}

	validator, err := NewValidator()
	if err != nil {
		return nil, fmt.Errorf("创建验证器失败: %w", err)
	}

	handoffGen, err := NewHandoffGenerator()
	if err != nil {
		return nil, fmt.Errorf("创建手交生成器失败: %w", err)
	}

	e := &Engine{
		extractor:  NewExtractor(),
		validator:  validator,
		handoffGen: handoffGen,
		homeDir:    homeDir,
	}

	if cfg != nil && cfg.Enabled && cfg.APIKey != "" {
		e.llmEnhancer = NewLLMEnhancer(*cfg)
		e.llmEnabled = true
	}

	return e, nil
}

// GenerateHandoff 生成会话交接摘要
// 需要启用 LLM 增强才能使用
func (e *Engine) GenerateHandoff(projectDir string, sessionCount int) (*HandoffSummary, error) {
	// 检查 LLM 是否启用
	if !e.llmEnabled {
		return nil, fmt.Errorf("生成跨会话摘要需要接入 LLM，请在设置中配置 LLM 后重试")
	}

	// 加载项目的所有会话
	allSessions, err := e.loadProjectSessions(projectDir)
	if err != nil {
		return nil, fmt.Errorf("加载会话失败: %w", err)
	}

	if len(allSessions) == 0 {
		return nil, fmt.Errorf("项目 %s 没有会话数据", projectDir)
	}

	// 按时间排序（最新在前）
	SortSessionsByTime(allSessions)

	// 限制会话数量
	totalSessions := len(allSessions)
	if sessionCount > 0 && sessionCount < totalSessions {
		allSessions = FilterRecentSessions(allSessions, sessionCount)
	}

	// 先执行关键词提取（始终执行，作为 LLM 的参考输入和回退方案）
	completedTasks := e.extractor.ExtractCompletedTasks(allSessions)
	completedTasks = DeduplicateTasks(completedTasks)

	pendingTasks := e.extractor.ExtractPendingTasks(allSessions)

	decisions := e.extractor.ExtractDecisions(allSessions)
	decisions = DeduplicateDecisions(decisions)

	fileSummaries := e.extractor.BuildFileSummaries(allSessions)

	issues := e.extractor.ExtractKnownIssues(allSessions)
	issues = DeduplicateIssues(issues)

	keywordSummaryText := ExtractSessionSummary(allSessions)

	llmUsed := false

	// 尝试 LLM 增强分析
	if e.llmEnhancer != nil && e.llmEnabled {
		llmUsed = e.tryLLMEnhancement(allSessions, &keywordSummaryText,
			&completedTasks, &pendingTasks, &decisions, &issues)
	}

	// Git 交叉验证
	if len(allSessions) > 0 {
		cwd := allSessions[0].CWD
		completedTasks = e.validator.ValidateTasks(completedTasks, cwd)
		completedTasks = e.validator.ValidateAgainstSessions(completedTasks, allSessions)
	}

	// 构建摘要
	summary := &HandoffSummary{
		Project:        projectDir,
		SessionsUsed:   len(allSessions),
		SessionsTotal:  totalSessions,
		Summary:        keywordSummaryText,
		CompletedTasks: completedTasks,
		PendingTasks:   pendingTasks,
		KeyDecisions:   decisions,
		ModifiedFiles:  fileSummaries,
		KnownIssues:    issues,
		GeneratedAt:    time.Now(),
		LLMEnhanced:    llmUsed,
	}

	// 裁剪各维度条目数，防止输出过长
	capSummaryItems(summary)

	// 计算质量评分
	summary.Quality = CalculateSummaryQuality(summary)
	if llmUsed {
		summary.Quality.OverallScore = clampScore(summary.Quality.OverallScore + 0.15)
	}

	return summary, nil
}

// tryLLMEnhancement 尝试使用 LLM 增强摘要，失败时优雅降级。
// 返回 true 表示 LLM 增强成功，false 表示回退到关键词结果。
func (e *Engine) tryLLMEnhancement(
	allSessions []*session.Session,
	summaryText *string,
	completedTasks *[]CompletedTask,
	pendingTasks *[]PendingTask,
	decisions *[]Decision,
	issues *[]string,
) bool {
	// 构建临时的关键词结果作为 LLM 的参考
	keywordResult := &HandoffSummary{
		CompletedTasks: *completedTasks,
		PendingTasks:   *pendingTasks,
		KeyDecisions:   *decisions,
		KnownIssues:    *issues,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	llmResult, err := e.llmEnhancer.EnhanceSummary(ctx, allSessions, keywordResult)
	if err != nil {
		log.Printf("WARN: LLM增强失败，回退到关键词提取: %v", err)
		return false
	}

	// 合并 LLM 结果
	e.mergeLLMResult(llmResult, summaryText, completedTasks, pendingTasks, decisions, issues)
	return true
}

// mergeLLMResult 将 LLM 结果合并到关键词结果中。LLM 结果优先。
func (e *Engine) mergeLLMResult(
	llmResult *LLMEnhancedResult,
	summaryText *string,
	completedTasks *[]CompletedTask,
	pendingTasks *[]PendingTask,
	decisions *[]Decision,
	issues *[]string,
) {
	// 叙事摘要：直接替换
	if llmResult.NarrativeSummary != "" {
		*summaryText = llmResult.NarrativeSummary
	}

	// 已完成任务：LLM 结果优先，关键词结果去重后追加
	llmCompleted := make([]CompletedTask, 0, len(llmResult.CompletedTasks))
	for _, t := range llmResult.CompletedTasks {
		if strings.TrimSpace(t.Description) != "" {
			llmCompleted = append(llmCompleted, CompletedTask{
				Description:  t.Description,
				FilesChanged: t.FilesHint,
			})
		}
	}
	*completedTasks = mergeTaskLists(llmCompleted, *completedTasks)

	// 待办任务
	llmPending := make([]PendingTask, 0, len(llmResult.PendingTasks))
	for _, t := range llmResult.PendingTasks {
		if strings.TrimSpace(t.Description) != "" {
			llmPending = append(llmPending, PendingTask{
				Description: t.Description,
				Source:      t.Source,
			})
		}
	}
	*pendingTasks = mergePendingLists(llmPending, *pendingTasks)

	// 关键决策
	llmDecisions := make([]Decision, 0, len(llmResult.KeyDecisions))
	for _, d := range llmResult.KeyDecisions {
		if strings.TrimSpace(d.Description) != "" {
			llmDecisions = append(llmDecisions, Decision{
				Description: d.Description,
				Context:     d.Rationale,
			})
		}
	}
	*decisions = mergeDecisionLists(llmDecisions, *decisions)

	// 已知问题
	if len(llmResult.KnownIssues) > 0 {
		*issues = mergeStringLists(llmResult.KnownIssues, *issues)
	}
}

// mergeTaskLists 合并任务列表，LLM 结果优先，关键词结果去重后追加。
func mergeTaskLists(llmTasks, keywordTasks []CompletedTask) []CompletedTask {
	result := make([]CompletedTask, len(llmTasks))
	copy(result, llmTasks)

	for _, kt := range keywordTasks {
		isDup := false
		for _, lt := range llmTasks {
			if calculateStringSimilarity(lt.Description, kt.Description) > 0.6 {
				isDup = true
				break
			}
		}
		if !isDup {
			result = append(result, kt)
		}
	}
	return result
}

// mergePendingLists 合并待办列表。
func mergePendingLists(llmPending, keywordPending []PendingTask) []PendingTask {
	result := make([]PendingTask, len(llmPending))
	copy(result, llmPending)

	for _, kp := range keywordPending {
		isDup := false
		for _, lp := range llmPending {
			if calculateStringSimilarity(lp.Description, kp.Description) > 0.5 {
				isDup = true
				break
			}
		}
		if !isDup {
			result = append(result, kp)
		}
	}
	return result
}

// mergeDecisionLists 合并决策列表。
func mergeDecisionLists(llmDecisions, keywordDecisions []Decision) []Decision {
	result := make([]Decision, len(llmDecisions))
	copy(result, llmDecisions)

	for _, kd := range keywordDecisions {
		isDup := false
		for _, ld := range llmDecisions {
			if calculateStringSimilarity(ld.Description, kd.Description) > 0.5 {
				isDup = true
				break
			}
		}
		if !isDup {
			result = append(result, kd)
		}
	}
	return result
}

// mergeStringLists 合并字符串列表。
func mergeStringLists(llmItems, keywordItems []string) []string {
	result := make([]string, len(llmItems))
	copy(result, llmItems)

	for _, ki := range keywordItems {
		isDup := false
		for _, li := range llmItems {
			if calculateStringSimilarity(li, ki) > 0.5 {
				isDup = true
				break
			}
		}
		if !isDup {
			result = append(result, ki)
		}
	}
	return result
}

// 各维度条目数上限，确保最终 Markdown 不超过 MaxSummaryLines 行
const (
	maxCompletedTasks = 8
	maxPendingTasks   = 5
	maxKeyDecisions   = 5
	maxModifiedFiles  = 10
	maxKnownIssues    = 5
)

// capSummaryItems 裁剪摘要各维度的条目数，防止输出过长。
func capSummaryItems(s *HandoffSummary) {
	if len(s.CompletedTasks) > maxCompletedTasks {
		s.CompletedTasks = s.CompletedTasks[:maxCompletedTasks]
	}
	if len(s.PendingTasks) > maxPendingTasks {
		s.PendingTasks = s.PendingTasks[:maxPendingTasks]
	}
	if len(s.KeyDecisions) > maxKeyDecisions {
		s.KeyDecisions = s.KeyDecisions[:maxKeyDecisions]
	}
	if len(s.ModifiedFiles) > maxModifiedFiles {
		s.ModifiedFiles = s.ModifiedFiles[:maxModifiedFiles]
	}
	if len(s.KnownIssues) > maxKnownIssues {
		s.KnownIssues = s.KnownIssues[:maxKnownIssues]
	}
}

// clampScore 将评分限制在 [0, 1] 范围内。
func clampScore(score float64) float64 {
	if score > 1.0 {
		return 1.0
	}
	if score < 0 {
		return 0
	}
	return score
}

// GenerateHandoffMarkdown 生成 Markdown 格式的交接摘要
func (e *Engine) GenerateHandoffMarkdown(projectDir string, sessionCount int) (string, *HandoffSummary, error) {
	summary, err := e.GenerateHandoff(projectDir, sessionCount)
	if err != nil {
		return "", nil, err
	}

	markdown := e.handoffGen.GenerateMarkdown(summary)
	return markdown, summary, nil
}

// GenerateHandoffPrompt 生成可粘贴的 prompt 片段
func (e *Engine) GenerateHandoffPrompt(projectDir string, sessionCount int) (string, *HandoffSummary, error) {
	summary, err := e.GenerateHandoff(projectDir, sessionCount)
	if err != nil {
		return "", nil, err
	}

	prompt := e.handoffGen.GeneratePrompt(summary)
	return prompt, summary, nil
}

// ExportToMemory 导出交接摘要到 memory 目录
func (e *Engine) ExportToMemory(projectDir string, sessionCount int) (string, error) {
	summary, err := e.GenerateHandoff(projectDir, sessionCount)
	if err != nil {
		return "", err
	}

	markdown := e.handoffGen.GenerateMarkdown(summary)
	return e.handoffGen.SaveToMemory(summary, markdown)
}

// GetHandoffGenerator 获取手交生成器
func (e *Engine) GetHandoffGenerator() *HandoffGenerator {
	return e.handoffGen
}

// IsLLMEnabled 返回 LLM 增强是否已启用
func (e *Engine) IsLLMEnabled() bool {
	return e.llmEnabled
}

// GetAvailableProjects 获取所有有会话的项目列表
func (e *Engine) GetAvailableProjects() ([]ProjectInfo, error) {
	claudeDir := filepath.Join(e.homeDir, ".claude", "projects")
	if _, err := os.Stat(claudeDir); os.IsNotExist(err) {
		return []ProjectInfo{}, nil
	}

	entries, err := os.ReadDir(claudeDir)
	if err != nil {
		return nil, fmt.Errorf("读取 Claude 项目目录失败: %w", err)
	}

	var projects []ProjectInfo
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectDir := filepath.Join(claudeDir, entry.Name())
		jsonlFiles, _ := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
		if len(jsonlFiles) == 0 {
			continue
		}

		// 获取最近的会话时间
		var lastActivity time.Time
		for _, f := range jsonlFiles {
			info, err := os.Stat(f)
			if err == nil && info.ModTime().After(lastActivity) {
				lastActivity = info.ModTime()
			}
		}

		projects = append(projects, ProjectInfo{
			Name:         formatProjectDirName(entry.Name()),
			DirName:      entry.Name(),
			SessionCount: len(jsonlFiles),
			LastActivity: lastActivity,
		})
	}

	// 按最后活动时间排序
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].LastActivity.After(projects[j].LastActivity)
	})

	return projects, nil
}

// maxLoadSessions 单次加载会话的最大数量，防止内存压力过大
const maxLoadSessions = 50

// loadProjectSessions 加载项目的最近会话（按修改时间排序，最多 maxLoadSessions 个）
func (e *Engine) loadProjectSessions(projectDir string) ([]*session.Session, error) {
	claudeDir := filepath.Join(e.homeDir, ".claude", "projects", projectDir)
	if _, err := os.Stat(claudeDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("项目目录不存在: %s", projectDir)
	}

	jsonlFiles, err := filepath.Glob(filepath.Join(claudeDir, "*.jsonl"))
	if err != nil {
		return nil, fmt.Errorf("查找会话文件失败: %w", err)
	}

	// 按修改时间排序，最新的在前，限制加载数量
	type fileWithTime struct {
		path    string
		modTime time.Time
	}
	var sortedFiles []fileWithTime
	for _, f := range jsonlFiles {
		fi, err := os.Stat(f)
		if err != nil {
			continue
		}
		sortedFiles = append(sortedFiles, fileWithTime{path: f, modTime: fi.ModTime()})
	}
	sort.Slice(sortedFiles, func(i, j int) bool {
		return sortedFiles[i].modTime.After(sortedFiles[j].modTime)
	})

	if len(sortedFiles) > maxLoadSessions {
		sortedFiles = sortedFiles[:maxLoadSessions]
	}

	var sessions []*session.Session
	reader := claude.NewReader()

	for _, f := range sortedFiles {
		sess, err := reader.Read(f.path)
		if err != nil {
			continue // 跳过解析失败的会话
		}
		sessions = append(sessions, sess)
	}

	return sessions, nil
}

// formatProjectDirName 将项目目录名转换为可读的名称
func formatProjectDirName(dirName string) string {
	return common.FormatProjectName(dirName)
}

// ProjectInfo 项目信息
type ProjectInfo struct {
	Name         string    `json:"name"`         // 显示名称
	DirName      string    `json:"dirName"`      // 目录名
	SessionCount int       `json:"sessionCount"` // 会话数
	LastActivity time.Time `json:"lastActivity"` // 最后活动时间
}

// CalculateSummaryQuality 计算摘要质量评分
func CalculateSummaryQuality(summary *HandoffSummary) SummaryQuality {
	quality := SummaryQuality{}

	// 1. 完整性评分（40%）：各维度是否有内容
	completenessScore := 0.0
	totalDimensions := 5.0

	if len(summary.CompletedTasks) > 0 {
		completenessScore += 1.0
	}
	if len(summary.PendingTasks) > 0 {
		completenessScore += 1.0
	}
	if len(summary.KeyDecisions) > 0 {
		completenessScore += 1.0
	}
	if len(summary.ModifiedFiles) > 0 {
		completenessScore += 1.0
	}
	if len(summary.KnownIssues) > 0 {
		completenessScore += 1.0
	}
	quality.Completeness = completenessScore / totalDimensions

	// 2. 准确性评分（40%）：Git验证率
	if len(summary.CompletedTasks) > 0 {
		verifiedCount := 0
		for _, task := range summary.CompletedTasks {
			if task.VerifiedByGit {
				verifiedCount++
			}
		}
		quality.Accuracy = float64(verifiedCount) / float64(len(summary.CompletedTasks))
	} else {
		quality.Accuracy = 0
	}

	// 3. 时效性评分（20%）：基于会话时间分布
	if summary.SessionsUsed > 0 {
		// 使用会话使用率作为时效性指标
		// 使用的会话越多，时效性越好
		sessionRatio := float64(summary.SessionsUsed) / float64(summary.SessionsTotal)
		if sessionRatio > 1 {
			sessionRatio = 1
		}
		quality.Freshness = sessionRatio
	} else {
		quality.Freshness = 0
	}

	// 4. 综合评分
	quality.OverallScore = quality.Completeness*0.4 + quality.Accuracy*0.4 + quality.Freshness*0.2

	return quality
}
