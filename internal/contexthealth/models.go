package contexthealth

// TurnMetric 单轮指标
type TurnMetric struct {
	TurnIndex          int    `json:"turnIndex"`
	InputTokens        int    `json:"inputTokens"`        // 当次 API 调用的输入 token
	OutputTokens       int    `json:"outputTokens"`       // 当次 API 调用的输出 token
	CacheReadTokens    int    `json:"cacheReadTokens"`    // 缓存读取 token
	CacheCreationTokens int   `json:"cacheCreationTokens"` // 缓存创建 token
	ThinkingCount      int    `json:"thinkingCount"`      // 思考块数量
	ThinkingChars      int    `json:"thinkingChars"`      // 思考文本字符数
	ToolUseCount       int    `json:"toolUseCount"`       // 工具调用数
	Timestamp          string `json:"timestamp"`
}

// SessionHealth 单个会话健康报告
type SessionHealth struct {
	SessionID         string       `json:"sessionId"`
	Model             string       `json:"model"`
	ContextLimit      int          `json:"contextLimit"`      // 模型上下文窗口上限
	MaxContextUsed    int          `json:"maxContextUsed"`    // 峰值上下文（所有轮次中最大的 input_tokens）
	ContextUsagePct   float64      `json:"contextUsagePct"`   // 峰值上下文占窗口的百分比
	TotalTurns        int          `json:"totalTurns"`        // 总轮数（去重后的 API 调用数）
	ThinkingTurns     int          `json:"thinkingTurns"`     // 有思考的轮数
	AvgThinkingChars  float64      `json:"avgThinkingChars"`  // 平均思考字符数
	ToolCallsTotal    int          `json:"toolCallsTotal"`    // 工具调用总数
	CompressionEvents int          `json:"compressionEvents"` // 上下文压缩事件次数（input_tokens 大幅下降）
	HealthScore       int          `json:"healthScore"`       // 0-100 健康评分
	HealthLevel       string       `json:"healthLevel"`       // "excellent"/"good"/"warning"/"critical"
	Alerts            []string     `json:"alerts"`            // 退化信号列表
	Turns             []TurnMetric `json:"turns"`
	// 缓存相关
	TotalCacheReadTokens  int     `json:"totalCacheReadTokens"`  // 缓存读取 token 总计
	TotalCacheWriteTokens int     `json:"totalCacheWriteTokens"` // 缓存写入 token 总计
	CacheHitRate          float64 `json:"cacheHitRate"`          // 缓存命中率 0-100
}

// OverviewHealth 全局健康概览
type OverviewHealth struct {
	TotalSessions    int            `json:"totalSessions"`
	AvgContextUsage  float64        `json:"avgContextUsage"`  // 平均峰值上下文使用百分比
	MaxContextUsage  float64        `json:"maxContextUsage"`  // 最高峰值上下文使用百分比
	AvgHealthScore   float64        `json:"avgHealthScore"`   // 平均健康评分
	WarningCount     int            `json:"warningCount"`     // 峰值上下文 >50% 的会话数
	CriticalCount    int            `json:"criticalCount"`    // 峰值上下文 >80% 的会话数
	TopSessions      []SessionHealth `json:"topSessions"`     // 最需关注的会话（按健康评分升序）
	AvgCacheHitRate  float64        `json:"avgCacheHitRate"`  // 全局平均缓存命中率
}
