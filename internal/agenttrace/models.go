package agenttrace

import "time"

// AgentNode 表示 Agent 树中的一个节点（一个 Agent，而非单条消息）
type AgentNode struct {
	AgentID      string        `json:"agentId"`
	ParentID     string        `json:"parentId,omitempty"`
	UUID         string        `json:"uuid"` // 该 Agent 第一条消息的 UUID
	Depth        int           `json:"depth"`
	InputTokens  int           `json:"inputTokens"`
	OutputTokens int           `json:"outputTokens"`
	ToolCalls    int           `json:"toolCalls"`
	MessageCount int           `json:"messageCount"` // 该 Agent 产生的消息数
	StartTime    time.Time     `json:"startTime"`
	EndTime      time.Time     `json:"endTime"`
	Duration     time.Duration `json:"duration"`
	Status       string        `json:"status"`      // running, completed, error
	Type         string        `json:"type"`         // user, assistant
	Name         string        `json:"name"`         // 显示名称：主 Agent / 子 Agent: xxx
	IsSubAgent   bool          `json:"isSubAgent"`   // 是否为子 Agent
	Children     []*AgentNode  `json:"children"`
}

// AgentTree 表示一次会话的完整 Agent 调用树
type AgentTree struct {
	SessionID   string       `json:"sessionId"`
	Root        *AgentNode   `json:"root"`         // 第一个根节点（向后兼容）
	Roots       []*AgentNode `json:"roots"`        // 所有根节点（支持多棵独立 Agent 树）
	TotalAgents int          `json:"totalAgents"`
	MaxDepth    int          `json:"maxDepth"`
}
