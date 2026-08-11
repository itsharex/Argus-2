// Package hookmonitor 提供 Hook 执行日志的解析、统计和实时监控能力。
package hookmonitor

import (
	"time"
)

// HookEventType 表示 Hook 执行事件的类型。
type HookEventType string

const (
	HookEventStart   HookEventType = "start"
	HookEventSuccess HookEventType = "success"
	HookEventFailure HookEventType = "failure"
	HookEventTimeout HookEventType = "timeout"
)

// IsValid 检查事件类型是否有效。
func (t HookEventType) IsValid() bool {
	switch t {
	case HookEventStart, HookEventSuccess, HookEventFailure, HookEventTimeout:
		return true
	}
	return false
}

// String 返回事件类型的字符串表示。
func (t HookEventType) String() string {
	return string(t)
}

// HookExecution 表示一次 Hook 执行记录。
type HookExecution struct {
	ID         string        `json:"id"`
	HookType   string        `json:"hookType"`              // PreToolUse, PostToolUse, Stop, Notification
	Matcher    string        `json:"matcher"`               // 匹配模式
	Command    string        `json:"command"`               // 执行的命令
	ProjectDir string        `json:"projectDir"`            // 项目目录
	SessionID  string        `json:"sessionId,omitempty"`   // 关联的会话 ID
	StartTime  time.Time     `json:"startTime"`             // 开始时间
	EndTime    time.Time     `json:"endTime"`               // 结束时间
	Duration   time.Duration `json:"duration"`              // 执行耗时
	ExitCode   int           `json:"exitCode"`              // 退出码
	Stdout     string        `json:"stdout,omitempty"`      // 标准输出
	Stderr     string        `json:"stderr,omitempty"`      // 标准错误
	Status     HookEventType `json:"status"`                // 执行状态
	Source     string        `json:"source"`                // 日志来源："log" | "jsonl"
}

// HookTypeStats 按 Hook 类型的聚合统计。
type HookTypeStats struct {
	Executions  int           `json:"executions"`
	Successes   int           `json:"successes"`
	Failures    int           `json:"failures"`
	SuccessRate float64       `json:"successRate"`
	AvgDuration time.Duration `json:"avgDuration"`
}

// HookStats Hook 执行的聚合统计数据。
type HookStats struct {
	TotalExecutions int                        `json:"totalExecutions"`
	SuccessRate     float64                    `json:"successRate"`
	AvgDuration     time.Duration              `json:"avgDuration"`
	FailureStreak   int                        `json:"failureStreak"` // 当前连续失败次数
	ByHookType      map[string]*HookTypeStats  `json:"byHookType"`
	LastExecution   *HookExecution             `json:"lastExecution,omitempty"`
}

// AlertEvent 告警事件，当连续失败达到阈值时触发。
type AlertEvent struct {
	Type       string `json:"type"`       // "consecutive_failure"
	Count      int    `json:"count"`      // 连续失败次数
	HookType   string `json:"hookType"`   // 触发告警的 Hook 类型
	LastErrors []string `json:"lastErrors"` // 最近的错误信息
}
