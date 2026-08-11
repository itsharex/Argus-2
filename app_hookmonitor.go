package main

import (
	"fmt"

	"argus-desktop/internal/hookmonitor"
)

// ============================================
// Hook 执行日志监控 API
// ============================================

// HookExecutionDTO Hook 执行记录（前端展示用）
type HookExecutionDTO struct {
	ID         string  `json:"id"`
	HookType   string  `json:"hookType"`
	Matcher    string  `json:"matcher"`
	Command    string  `json:"command"`
	ProjectDir string  `json:"projectDir"`
	SessionID  string  `json:"sessionId,omitempty"`
	StartTime  string  `json:"startTime"`
	EndTime    string  `json:"endTime"`
	Duration   int64   `json:"duration"` // 毫秒
	ExitCode   int     `json:"exitCode"`
	Stdout     string  `json:"stdout,omitempty"`
	Stderr     string  `json:"stderr,omitempty"`
	Status     string  `json:"status"`
	Source     string  `json:"source"`
}

// HookTypeStatsDTO 按 Hook 类型的聚合统计（前端展示用）
type HookTypeStatsDTO struct {
	Executions  int     `json:"executions"`
	Successes   int     `json:"successes"`
	Failures    int     `json:"failures"`
	SuccessRate float64 `json:"successRate"`
	AvgDuration int64   `json:"avgDuration"` // 毫秒
}

// HookStatsDTO Hook 执行聚合统计（前端展示用）
type HookStatsDTO struct {
	TotalExecutions int                       `json:"totalExecutions"`
	SuccessRate     float64                   `json:"successRate"`
	AvgDuration     int64                     `json:"avgDuration"` // 毫秒
	FailureStreak   int                       `json:"failureStreak"`
	ByHookType      map[string]*HookTypeStatsDTO `json:"byHookType"`
	LastExecution   *HookExecutionDTO         `json:"lastExecution,omitempty"`
}

// hookExecutionsToDTO 将内部模型转换为前端 DTO。
func hookExecutionsToDTO(execs []hookmonitor.HookExecution) []HookExecutionDTO {
	result := make([]HookExecutionDTO, len(execs))
	for i, ex := range execs {
		result[i] = hookExecutionToDTO(ex)
	}
	return result
}

// hookExecutionToDTO 将单条内部模型转换为前端 DTO。
func hookExecutionToDTO(ex hookmonitor.HookExecution) HookExecutionDTO {
	return HookExecutionDTO{
		ID:         ex.ID,
		HookType:   ex.HookType,
		Matcher:    ex.Matcher,
		Command:    ex.Command,
		ProjectDir: ex.ProjectDir,
		SessionID:  ex.SessionID,
		StartTime:  ex.StartTime.Format("2006-01-02 15:04:05"),
		EndTime:    ex.EndTime.Format("2006-01-02 15:04:05"),
		Duration:   ex.Duration.Milliseconds(),
		ExitCode:   ex.ExitCode,
		Stdout:     ex.Stdout,
		Stderr:     ex.Stderr,
		Status:     string(ex.Status),
		Source:     ex.Source,
	}
}

// hookStatsToDTO 将内部统计模型转换为前端 DTO。
func hookStatsToDTO(stats *hookmonitor.HookStats) *HookStatsDTO {
	if stats == nil {
		return &HookStatsDTO{
			ByHookType: make(map[string]*HookTypeStatsDTO),
		}
	}

	dto := &HookStatsDTO{
		TotalExecutions: stats.TotalExecutions,
		SuccessRate:     stats.SuccessRate,
		AvgDuration:     stats.AvgDuration.Milliseconds(),
		FailureStreak:   stats.FailureStreak,
		ByHookType:      make(map[string]*HookTypeStatsDTO, len(stats.ByHookType)),
	}

	for hookType, typeStats := range stats.ByHookType {
		dto.ByHookType[hookType] = &HookTypeStatsDTO{
			Executions:  typeStats.Executions,
			Successes:   typeStats.Successes,
			Failures:    typeStats.Failures,
			SuccessRate: typeStats.SuccessRate,
			AvgDuration: typeStats.AvgDuration.Milliseconds(),
		}
	}

	if stats.LastExecution != nil {
		last := hookExecutionToDTO(*stats.LastExecution)
		dto.LastExecution = &last
	}

	return dto
}

// GetHookExecutions 获取 Hook 执行记录列表。
// limit <= 0 返回全部（最多 1000 条），hookType 为空表示不过滤。
func (a *App) GetHookExecutions(limit int, hookType string) ([]HookExecutionDTO, error) {
	if a.hookMonitor == nil {
		return []HookExecutionDTO{}, nil
	}

	executions := a.hookMonitor.GetExecutions(limit, hookType)
	return hookExecutionsToDTO(executions), nil
}

// GetHookStats 获取 Hook 执行的聚合统计数据。
func (a *App) GetHookStats() (*HookStatsDTO, error) {
	if a.hookMonitor == nil {
		return &HookStatsDTO{
			ByHookType: make(map[string]*HookTypeStatsDTO),
		}, nil
	}

	stats := a.hookMonitor.GetStats()
	return hookStatsToDTO(stats), nil
}

// ClearHookLogs 清空 Hook 执行日志。
func (a *App) ClearHookLogs() error {
	if a.hookMonitor == nil {
		return fmt.Errorf("Hook 监控引擎未初始化")
	}

	a.hookMonitor.ClearLogs()
	return nil
}

// RefreshHookLogs 重新扫描所有日志来源。
func (a *App) RefreshHookLogs() ([]HookExecutionDTO, error) {
	if a.hookMonitor == nil {
		return []HookExecutionDTO{}, nil
	}

	executions, err := a.hookMonitor.ScanLogs()
	if err != nil {
		return nil, fmt.Errorf("刷新 Hook 日志失败: %w", err)
	}

	return hookExecutionsToDTO(executions), nil
}
