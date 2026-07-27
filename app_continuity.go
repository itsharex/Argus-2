package main

import (
	"fmt"
	"time"

	"argus-desktop/internal/continuity"
)

// ============================================
// Session Continuity API
// ============================================

// ContinuityProjectInfo 项目信息（前端展示用）
type ContinuityProjectInfo struct {
	Name         string    `json:"name"`
	DirName      string    `json:"dirName"`
	SessionCount int       `json:"sessionCount"`
	LastActivity time.Time `json:"lastActivity"`
}

// ContinuityTaskInfo 任务信息（前端展示用）
type ContinuityTaskInfo struct {
	Description   string    `json:"description"`
	SessionID     string    `json:"sessionId"`
	FilesChanged  []string  `json:"filesChanged"`
	VerifiedByGit bool      `json:"verifiedByGit"`
	Timestamp     time.Time `json:"timestamp"`
}

// ContinuityPendingTaskInfo 待办任务信息
type ContinuityPendingTaskInfo struct {
	Description string   `json:"description"`
	Source      string   `json:"source"`
	SessionID   string   `json:"sessionId"`
	FilesHint   []string `json:"filesHint"`
}

// ContinuityDecisionInfo 决策信息
type ContinuityDecisionInfo struct {
	Description string    `json:"description"`
	Context     string    `json:"context"`
	Timestamp   time.Time `json:"timestamp"`
	SessionID   string    `json:"sessionId"`
}

// ContinuityFileInfo 文件信息
type ContinuityFileInfo struct {
	Path         string `json:"path"`
	ChangeCount  int    `json:"changeCount"`
	ActionCount  int    `json:"actionCount"`
	LastAction   string `json:"lastAction"`
	IsTestFile   bool   `json:"isTestFile"`
	IsConfigFile bool   `json:"isConfigFile"`
}

// ContinuitySummary 完整的交接摘要（前端展示用）
type ContinuitySummary struct {
	Project        string                      `json:"project"`
	SessionsUsed   int                         `json:"sessionsUsed"`
	SessionsTotal  int                         `json:"sessionsTotal"`
	Summary        string                      `json:"summary"`
	CompletedTasks []ContinuityTaskInfo        `json:"completedTasks"`
	PendingTasks   []ContinuityPendingTaskInfo `json:"pendingTasks"`
	KeyDecisions   []ContinuityDecisionInfo    `json:"keyDecisions"`
	ModifiedFiles  []ContinuityFileInfo        `json:"modifiedFiles"`
	KnownIssues    []string                    `json:"knownIssues"`
	GeneratedAt    time.Time                   `json:"generatedAt"`
	Quality        ContinuityQualityInfo       `json:"quality"`
	LLMEnhanced    bool                        `json:"llmEnhanced"`
}

// ContinuityQualityInfo 质量评分信息
type ContinuityQualityInfo struct {
	Completeness float64 `json:"completeness"`
	Accuracy     float64 `json:"accuracy"`
	Freshness    float64 `json:"freshness"`
	OverallScore float64 `json:"overallScore"`
}

// GetContinuityProjects 获取所有有会话的项目列表
func (a *App) GetContinuityProjects() ([]ContinuityProjectInfo, error) {
	a.continuityMu.RLock()
	if a.continuity == nil {
		a.continuityMu.RUnlock()
		return nil, fmt.Errorf("会话连续性引擎未初始化")
	}
	continuityEngine := a.continuity
	a.continuityMu.RUnlock()

	projects, err := continuityEngine.GetAvailableProjects()
	if err != nil {
		return nil, err
	}

	result := make([]ContinuityProjectInfo, len(projects))
	for i, p := range projects {
		result[i] = ContinuityProjectInfo{
			Name:         p.Name,
			DirName:      p.DirName,
			SessionCount: p.SessionCount,
			LastActivity: p.LastActivity,
		}
	}

	return result, nil
}

// IsContinuityLLMEnabled 检查跨会话摘要功能是否启用了 LLM 增强
func (a *App) IsContinuityLLMEnabled() bool {
	a.continuityMu.RLock()
	defer a.continuityMu.RUnlock()

	if a.continuity == nil {
		return false
	}
	return a.continuity.IsLLMEnabled()
}

// GenerateContinuityHandoff 生成会话交接摘要
func (a *App) GenerateContinuityHandoff(project string, sessionCount int) (*ContinuitySummary, error) {
	if project == "" {
		return nil, fmt.Errorf("项目目录名不能为空")
	}
	if sessionCount <= 0 {
		sessionCount = 10 // 默认值
	}

	a.continuityMu.RLock()
	if a.continuity == nil {
		a.continuityMu.RUnlock()
		return nil, fmt.Errorf("会话连续性引擎未初始化")
	}
	continuityEngine := a.continuity
	a.continuityMu.RUnlock()

	summary, err := continuityEngine.GenerateHandoff(project, sessionCount)
	if err != nil {
		return nil, err
	}

	// 转换为前端格式
	completedTasks := make([]ContinuityTaskInfo, len(summary.CompletedTasks))
	for i, t := range summary.CompletedTasks {
		completedTasks[i] = ContinuityTaskInfo{
			Description:   t.Description,
			SessionID:     t.SessionID,
			FilesChanged:  t.FilesChanged,
			VerifiedByGit: t.VerifiedByGit,
			Timestamp:     t.Timestamp,
		}
	}

	pendingTasks := make([]ContinuityPendingTaskInfo, len(summary.PendingTasks))
	for i, t := range summary.PendingTasks {
		pendingTasks[i] = ContinuityPendingTaskInfo{
			Description: t.Description,
			Source:      t.Source,
			SessionID:   t.SessionID,
			FilesHint:   t.FilesHint,
		}
	}

	keyDecisions := make([]ContinuityDecisionInfo, len(summary.KeyDecisions))
	for i, d := range summary.KeyDecisions {
		keyDecisions[i] = ContinuityDecisionInfo{
			Description: d.Description,
			Context:     d.Context,
			Timestamp:   d.Timestamp,
			SessionID:   d.SessionID,
		}
	}

	modifiedFiles := make([]ContinuityFileInfo, len(summary.ModifiedFiles))
	for i, f := range summary.ModifiedFiles {
		modifiedFiles[i] = ContinuityFileInfo{
			Path:         f.Path,
			ChangeCount:  f.ChangeCount,
			ActionCount:  f.ActionCount,
			LastAction:   f.LastAction,
			IsTestFile:   f.IsTestFile,
			IsConfigFile: f.IsConfigFile,
		}
	}

	return &ContinuitySummary{
		Project:        summary.Project,
		SessionsUsed:   summary.SessionsUsed,
		SessionsTotal:  summary.SessionsTotal,
		Summary:        summary.Summary,
		CompletedTasks: completedTasks,
		PendingTasks:   pendingTasks,
		KeyDecisions:   keyDecisions,
		ModifiedFiles:  modifiedFiles,
		KnownIssues:    summary.KnownIssues,
		GeneratedAt:    summary.GeneratedAt,
		Quality: ContinuityQualityInfo{
			Completeness: summary.Quality.Completeness,
			Accuracy:     summary.Quality.Accuracy,
			Freshness:    summary.Quality.Freshness,
			OverallScore: summary.Quality.OverallScore,
		},
		LLMEnhanced: summary.LLMEnhanced,
	}, nil
}

// ExportContinuityToMemory 导出交接摘要到 memory 目录
func (a *App) ExportContinuityToMemory(project string, sessionCount int) (string, error) {
	if project == "" {
		return "", fmt.Errorf("项目目录名不能为空")
	}
	if sessionCount <= 0 {
		sessionCount = 10 // 默认值
	}

	a.continuityMu.RLock()
	if a.continuity == nil {
		a.continuityMu.RUnlock()
		return "", fmt.Errorf("会话连续性引擎未初始化")
	}
	continuityEngine := a.continuity
	a.continuityMu.RUnlock()

	return continuityEngine.ExportToMemory(project, sessionCount)
}

// GenerateContinuityMarkdown 生成 Markdown 格式的交接摘要
func (a *App) GenerateContinuityMarkdown(project string, sessionCount int) (string, error) {
	if project == "" {
		return "", fmt.Errorf("项目目录名不能为空")
	}
	if sessionCount <= 0 {
		sessionCount = 10 // 默认值
	}

	a.continuityMu.RLock()
	if a.continuity == nil {
		a.continuityMu.RUnlock()
		return "", fmt.Errorf("会话连续性引擎未初始化")
	}
	continuityEngine := a.continuity
	a.continuityMu.RUnlock()

	markdown, _, err := continuityEngine.GenerateHandoffMarkdown(project, sessionCount)
	return markdown, err
}

// GenerateContinuityPrompt 生成可粘贴的 prompt 片段
func (a *App) GenerateContinuityPrompt(project string, sessionCount int) (string, error) {
	if project == "" {
		return "", fmt.Errorf("项目目录名不能为空")
	}
	if sessionCount <= 0 {
		sessionCount = 10 // 默认值
	}

	a.continuityMu.RLock()
	if a.continuity == nil {
		a.continuityMu.RUnlock()
		return "", fmt.Errorf("会话连续性引擎未初始化")
	}
	continuityEngine := a.continuity
	a.continuityMu.RUnlock()

	prompt, _, err := continuityEngine.GenerateHandoffPrompt(project, sessionCount)
	return prompt, err
}

// ExportContinuityToMemoryWithData 使用已生成的摘要数据导出到 memory 目录
// 避免重新调用 LLM，直接使用前端传递的摘要数据
func (a *App) ExportContinuityToMemoryWithData(project string, summary *ContinuitySummary) (string, error) {
	a.continuityMu.RLock()
	if a.continuity == nil {
		a.continuityMu.RUnlock()
		return "", fmt.Errorf("会话连续性引擎未初始化")
	}
	continuityEngine := a.continuity
	a.continuityMu.RUnlock()

	// 将前端格式的摘要转换为内部格式
	handoffSummary := convertToHandoffSummary(summary)

	// 生成 Markdown 并保存
	markdown := continuityEngine.GetHandoffGenerator().GenerateMarkdown(handoffSummary)
	return continuityEngine.GetHandoffGenerator().SaveToMemory(handoffSummary, markdown)
}

// GenerateContinuityMarkdownWithData 使用已生成的摘要数据生成 Markdown
// 避免重新调用 LLM，直接使用前端传递的摘要数据
func (a *App) GenerateContinuityMarkdownWithData(project string, summary *ContinuitySummary) (string, error) {
	a.continuityMu.RLock()
	if a.continuity == nil {
		a.continuityMu.RUnlock()
		return "", fmt.Errorf("会话连续性引擎未初始化")
	}
	continuityEngine := a.continuity
	a.continuityMu.RUnlock()

	// 将前端格式的摘要转换为内部格式
	handoffSummary := convertToHandoffSummary(summary)

	// 生成 Markdown
	return continuityEngine.GetHandoffGenerator().GenerateMarkdown(handoffSummary), nil
}

// GenerateContinuityPromptWithData 使用已生成的摘要数据生成 Prompt
// 避免重新调用 LLM，直接使用前端传递的摘要数据
func (a *App) GenerateContinuityPromptWithData(project string, summary *ContinuitySummary) (string, error) {
	a.continuityMu.RLock()
	if a.continuity == nil {
		a.continuityMu.RUnlock()
		return "", fmt.Errorf("会话连续性引擎未初始化")
	}
	continuityEngine := a.continuity
	a.continuityMu.RUnlock()

	// 将前端格式的摘要转换为内部格式
	handoffSummary := convertToHandoffSummary(summary)

	// 生成 Prompt
	return continuityEngine.GetHandoffGenerator().GeneratePrompt(handoffSummary), nil
}

// convertToHandoffSummary 将前端格式的摘要转换为内部格式
func convertToHandoffSummary(summary *ContinuitySummary) *continuity.HandoffSummary {
	// 转换已完成任务
	completedTasks := make([]continuity.CompletedTask, len(summary.CompletedTasks))
	for i, t := range summary.CompletedTasks {
		completedTasks[i] = continuity.CompletedTask{
			Description:   t.Description,
			SessionID:     t.SessionID,
			FilesChanged:  t.FilesChanged,
			VerifiedByGit: t.VerifiedByGit,
			Timestamp:     t.Timestamp,
		}
	}

	// 转换待办任务
	pendingTasks := make([]continuity.PendingTask, len(summary.PendingTasks))
	for i, t := range summary.PendingTasks {
		pendingTasks[i] = continuity.PendingTask{
			Description: t.Description,
			Source:      t.Source,
			SessionID:   t.SessionID,
			FilesHint:   t.FilesHint,
		}
	}

	// 转换关键决策
	keyDecisions := make([]continuity.Decision, len(summary.KeyDecisions))
	for i, d := range summary.KeyDecisions {
		keyDecisions[i] = continuity.Decision{
			Description: d.Description,
			Context:     d.Context,
			Timestamp:   d.Timestamp,
			SessionID:   d.SessionID,
		}
	}

	// 转换修改的文件
	modifiedFiles := make([]continuity.FileSummary, len(summary.ModifiedFiles))
	for i, f := range summary.ModifiedFiles {
		modifiedFiles[i] = continuity.FileSummary{
			Path:         f.Path,
			ChangeCount:  f.ChangeCount,
			ActionCount:  f.ActionCount,
			LastAction:   f.LastAction,
			IsTestFile:   f.IsTestFile,
			IsConfigFile: f.IsConfigFile,
		}
	}

	return &continuity.HandoffSummary{
		Project:        summary.Project,
		SessionsUsed:   summary.SessionsUsed,
		SessionsTotal:  summary.SessionsTotal,
		Summary:        summary.Summary,
		CompletedTasks: completedTasks,
		PendingTasks:   pendingTasks,
		KeyDecisions:   keyDecisions,
		ModifiedFiles:  modifiedFiles,
		KnownIssues:    summary.KnownIssues,
		GeneratedAt:    summary.GeneratedAt,
		Quality: continuity.SummaryQuality{
			Completeness: summary.Quality.Completeness,
			Accuracy:     summary.Quality.Accuracy,
			Freshness:    summary.Quality.Freshness,
			OverallScore: summary.Quality.OverallScore,
		},
		LLMEnhanced: summary.LLMEnhanced,
	}
}
