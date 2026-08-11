package session

import "time"

// ActionType 表示 Agent 使用的工具类型
type ActionType string

const (
	ActionWrite  ActionType = "Write"
	ActionEdit   ActionType = "Edit"
	ActionBash   ActionType = "Bash"
	ActionRead   ActionType = "Read"
	ActionGrep   ActionType = "Grep"
	ActionGlob   ActionType = "Glob"
	ActionOther  ActionType = "Other"
)

// RiskLevel 表示改动的风险等级
type RiskLevel string

const (
	RiskSafe   RiskLevel = "Safe"
	RiskReview RiskLevel = "Review"
	RiskDanger RiskLevel = "Danger"
)

// ChangeType 表示文件改动类型
type ChangeType string

const (
	ChangeCreated  ChangeType = "Created"
	ChangeModified ChangeType = "Modified"
	ChangeDeleted  ChangeType = "Deleted"
)

// Action 表示 Agent 的一次操作
type Action struct {
	ID          string         // tool_use id
	Type        ActionType     // Write, Edit, Bash, Read, Grep...
	Description string         // Agent 的描述：我要干什么
	FilePath    string         // 涉及的文件路径（对于文件操作）
	Input       map[string]any // 工具调用的原始参数
	Timestamp   time.Time
}

// FileChange 一个文件的改动
type FileChange struct {
	Path        string
	ChangeType  ChangeType  // Created, Modified, Deleted
	Actions     []Action    // 导致这个改动的 Agent 操作
	Diff        string      // unified diff 内容
	Risk        RiskLevel   // Safe / Review / Danger
	RiskReason  string      // 为什么标注这个风险等级
	ActionCount int         // 操作次数（用于导出）
}

// TokenUsage 记录 token 消耗
type TokenUsage struct {
	InputTokens         int
	OutputTokens        int
	TotalTokens         int
	CacheReadTokens     int // 缓存读取 token（prompt caching）
	CacheCreationTokens int // 缓存创建 token（prompt caching）
}

// AgentInfo 记录单个 Agent 的元数据（从 JSONL 中提取）
type AgentInfo struct {
	UUID         string    // 消息 UUID
	AgentID      string    // Agent 标识符（可能为空）
	ParentUUID   string    // 父消息 UUID
	Type         string    // 消息类型（user / assistant）
	InputTokens  int       // 输入 token
	OutputTokens int       // 输出 token
	ToolCalls    int       // 工具调用次数
	Timestamp    time.Time // 时间戳
}

// Session 一次 Agent 会话
type Session struct {
	ID          string
	AgentType   string       // "claude-code", "codex-cli"...
	Model       string       // "claude-sonnet-4-6"
	Prompt      string       // 用户的原始需求
	CWD         string
	GitBranch   string
	StartedAt   time.Time
	Duration    time.Duration
	Actions     []Action
	FileChanges []FileChange
	TokenUsage  TokenUsage
	Messages    []Message    // 完整消息历史
	Agents      []AgentInfo  // 所有 Agent 消息（用于构建 Agent 树）
}
