// Package productivity 提供开发者生产力分析，包括 DORA 风格指标和周趋势统计。
package productivity

// ProductivityReport 生产力分析报告
type ProductivityReport struct {
	Period                 string               `json:"period"`                 // 统计周期 "7d" / "30d" / "90d"
	SessionsTotal          int                  `json:"sessionsTotal"`          // 会话总数
	SessionsPerDay         float64              `json:"sessionsPerDay"`         // 日均会话数
	AvgSessionDurationMs   int64                `json:"avgSessionDurationMs"`   // 平均会话时长（毫秒）
	AvgFilesPerSession     float64              `json:"avgFilesPerSession"`     // 平均每次会话改动文件数
	AvgActionsPerSession   float64              `json:"avgActionsPerSession"`   // 平均每次会话操作数
	AvgToolCallsPerSession float64              `json:"avgToolCallsPerSession"` // 平均每次会话工具调用数
	TotalToolCalls         int                  `json:"totalToolCalls"`         // 工具调用总数
	TotalFilesChanged      int                  `json:"totalFilesChanged"`      // 文件改动总数
	TotalActions           int                  `json:"totalActions"`           // 操作总数
	WeeklyTrend            []WeeklyProductivity `json:"weeklyTrend"`            // 周趋势
}

// WeeklyProductivity 每周生产力数据
type WeeklyProductivity struct {
	Week         string `json:"week"`         // ISO 周标识 "2026-W28"
	Sessions     int    `json:"sessions"`     // 会话数
	FilesChanged int    `json:"filesChanged"` // 文件改动数
	TokensUsed   int    `json:"tokensUsed"`   // Token 使用量
	AvgDuration  int64  `json:"avgDurationMs"` // 平均会话时长（毫秒）
}

// sessionMetrics 单个会话的生产力指标（内部使用）
type sessionMetrics struct {
	SessionID    string
	ProjectDir   string
	StartedAt    int64 // Unix 毫秒
	DurationMs   int64
	ToolCalls    int
	Actions      int
	FilesChanged int
	TokensUsed   int
	ModDay       string // 修改日期 "2006-01-02"
}
