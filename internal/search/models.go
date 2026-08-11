// Package search 提供跨项目全文搜索功能，基于倒排索引实现高效检索。
package search

// FullTextResult 全文搜索结果
type FullTextResult struct {
	SessionID   string  `json:"sessionId"`            // 会话 ID
	ProjectDir  string  `json:"projectDir"`           // 项目目录名
	MatchType   string  `json:"matchType"`            // 匹配类型：prompt, message, command, filepath
	MatchContent string `json:"matchContent"`         // 匹配的原文片段（截取前后上下文）
	Score       float64 `json:"score"`                // 匹配分数（越高越相关）
	Timestamp   int64   `json:"timestamp"`            // 消息时间戳（Unix 毫秒）
}

// SearchRequest 搜索请求参数
type SearchRequest struct {
	Keyword    string   `json:"keyword"`                         // 搜索关键词（必填）
	MatchTypes []string `json:"matchTypes,omitempty"`            // 限定匹配类型（空=搜索所有）
	Projects   []string `json:"projects,omitempty"`              // 限定项目范围（空=搜索所有项目）
	Limit      int      `json:"limit"`                          // 返回结果上限（默认 50）
}

// SearchFilter 内部搜索过滤器
type SearchFilter struct {
	matchTypes map[string]bool
	projects   map[string]bool
	limit      int
}

// IndexStatus 索引状态信息
type IndexStatus struct {
	TotalDocuments int   `json:"totalDocuments"` // 索引的文档（JSONL 文件）数
	TotalEntries   int   `json:"totalEntries"`   // 索引条目总数
	LastBuiltAt    int64 `json:"lastBuiltAt"`    // 最后构建时间（Unix 毫秒）
	IndexSizeBytes int64 `json:"indexSizeBytes"` // 索引文件大小（字节）
}

// matchTypeWeights 各匹配类型的评分权重
var matchTypeWeights = map[string]float64{
	"prompt":   15.0,
	"command":  12.0,
	"filepath": 10.0,
	"message":  8.0,
}

// defaultLimit 默认返回结果上限
const defaultLimit = 50
