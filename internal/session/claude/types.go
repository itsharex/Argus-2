package claude

// JSONLLine 表示 JSONL 文件中的一行事件。
// 作为跨包共享的类型，被 analytics、contexthealth、productivity 等包复用。
type JSONLLine struct {
	Type       string        `json:"type"`
	UUID       string        `json:"uuid"`
	ParentUUID string        `json:"parentUuid"`
	AgentID    string        `json:"agentId"`
	SessionID  string        `json:"sessionId"`
	Timestamp  string        `json:"timestamp"`
	CWD        string        `json:"cwd"`
	GitBranch  string        `json:"gitBranch"`
	Message    *JSONLMessage `json:"message"`
}

// JSONLMessage 表示 JSONL 事件中的 message 字段。
type JSONLMessage struct {
	Role    string      `json:"role"`
	Content any         `json:"content"`
	Model   string      `json:"model"`
	Usage   *JSONLUsage `json:"usage"`
}

// JSONLUsage 表示 token 使用统计。
type JSONLUsage struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	CacheReadTokens     int `json:"cache_read_input_tokens"`
	CacheCreationTokens int `json:"cache_creation_input_tokens"`
}
