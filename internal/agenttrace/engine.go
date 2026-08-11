package agenttrace

import (
	"fmt"
	"sync"
	"time"

	"argus-desktop/internal/session"
	"argus-desktop/internal/session/claude"
)

// cacheEntry 缓存条目
type cacheEntry struct {
	tree      *AgentTree
	buildTime time.Time
	modTime   time.Time
}

// Engine Agent 追踪引擎
type Engine struct {
	mu    sync.RWMutex
	cache map[string]*cacheEntry // sessionID → cacheEntry
}

// NewEngine 创建新的 Agent 追踪引擎
func NewEngine() *Engine {
	return &Engine{
		cache: make(map[string]*cacheEntry),
	}
}

// agentGroup 内部使用：同一 AgentID 的消息聚合
type agentGroup struct {
	AgentID      string
	Messages     []session.AgentInfo
	InputTokens  int
	OutputTokens int
	ToolCalls    int
	StartTime    time.Time
	EndTime      time.Time
	ParentUUID   string // 该 Agent 第一条消息的 ParentUUID
	FirstUUID    string // 该 Agent 第一条消息的 UUID
	Type         string // user / assistant（取数量最多的类型）
}

// BuildTree 从会话数据构建 Agent 树（基于 AgentID 聚合，而非逐消息）
func (e *Engine) BuildTree(sess *session.Session) *AgentTree {
	if sess == nil || len(sess.Agents) == 0 {
		sessionID := ""
		if sess != nil {
			sessionID = sess.ID
		}
		return &AgentTree{
			SessionID:   sessionID,
			Roots:       []*AgentNode{},
			TotalAgents: 0,
			MaxDepth:    0,
		}
	}

	// 步骤1：按 AgentID 分组消息
	// empty AgentID = 主 Agent
	groups := make(map[string]*agentGroup)
	uuidToAgentID := make(map[string]string) // 消息UUID → AgentID

	for i := range sess.Agents {
		a := &sess.Agents[i]
		agentKey := a.AgentID
		if agentKey == "" || agentKey == "null" {
			agentKey = "main"
		}

		// 记录 UUID → AgentID 映射，用于后续确定父子关系
		uuidToAgentID[a.UUID] = agentKey

		group, exists := groups[agentKey]
		if !exists {
			group = &agentGroup{
				AgentID:    a.AgentID,
				ParentUUID: a.ParentUUID,
				FirstUUID:  a.UUID,
				StartTime:  a.Timestamp,
			}
			groups[agentKey] = group
		}

		group.Messages = append(group.Messages, *a)
		group.InputTokens += a.InputTokens
		group.OutputTokens += a.OutputTokens
		group.ToolCalls += a.ToolCalls

		// 跟踪最早/最晚时间
		if a.Timestamp.Before(group.StartTime) {
			group.StartTime = a.Timestamp
			group.ParentUUID = a.ParentUUID
			group.FirstUUID = a.UUID
		}
		if a.Timestamp.After(group.EndTime) {
			group.EndTime = a.Timestamp
		}

		// 统计类型
		if a.Type == "assistant" {
			group.Type = "assistant" // 优先使用 assistant
		} else if group.Type == "" {
			group.Type = a.Type
		}
	}

	// 步骤2：为每个 Agent 组确定父 Agent
	// 子Agent的第一条消息的ParentUUID → 找到该UUID所属的AgentID → 即父Agent
	agentParent := make(map[string]string) // agentKey → parentAgentKey
	for agentKey, group := range groups {
		if agentKey == "main" {
			continue // 主 Agent 没有父
		}
		parentUUID := group.ParentUUID
		if parentUUID == "" || parentUUID == "null" {
			// 没有父UUID，归到主Agent下
			agentParent[agentKey] = "main"
			continue
		}
		if parentAgentKey, ok := uuidToAgentID[parentUUID]; ok {
			agentParent[agentKey] = parentAgentKey
		} else {
			// 父UUID不属于任何已知Agent，归到主Agent下
			agentParent[agentKey] = "main"
		}
	}

	// 步骤3：构建 AgentNode 节点
	agentNodes := make(map[string]*AgentNode) // agentKey → AgentNode
	for agentKey, group := range groups {
		isSub := agentKey != "main"
		name := "主 Agent"
		if isSub {
			// 截取 AgentID 的前8位作为名称
			shortID := group.AgentID
			if len(shortID) > 8 {
				shortID = shortID[:8]
			}
			name = "子 Agent: " + shortID
		}

		node := &AgentNode{
			AgentID:      group.AgentID,
			UUID:         group.FirstUUID,
			Depth:        0,
			InputTokens:  group.InputTokens,
			OutputTokens: group.OutputTokens,
			ToolCalls:    group.ToolCalls,
			MessageCount: len(group.Messages),
			StartTime:    group.StartTime,
			EndTime:      group.EndTime,
			Duration:     group.EndTime.Sub(group.StartTime),
			Type:         group.Type,
			Status:       "completed",
			Name:         name,
			IsSubAgent:   isSub,
			Children:     []*AgentNode{},
		}
		agentNodes[agentKey] = node
	}

	// 步骤4：建立父子关系
	for childKey, parentKey := range agentParent {
		childNode := agentNodes[childKey]
		parentNode := agentNodes[parentKey]

		// 防止循环引用
		if childNode == nil || parentNode == nil || childKey == parentKey {
			continue
		}

		childNode.ParentID = parentNode.AgentID
		parentNode.Children = append(parentNode.Children, childNode)
	}

	// 步骤5：计算深度（从根开始递归）
	var roots []*AgentNode
	var rootForCompat *AgentNode
	for agentKey, node := range agentNodes {
		if _, hasParent := agentParent[agentKey]; !hasParent {
			// 这是根节点
			computeDepth(node, 0)
			roots = append(roots, node)
		}
	}

	// 如果没有根节点（所有节点都有父？不太可能，但兜底）
	if len(roots) == 0 && len(agentNodes) > 0 {
		// 取第一个节点作为根
		for _, node := range agentNodes {
			roots = append(roots, node)
			break
		}
		if len(roots) > 0 {
			computeDepth(roots[0], 0)
		}
	}

	// 向后兼容：Root 指向第一个根节点
	if len(roots) > 0 {
		rootForCompat = roots[0]
	}

	// 步骤6：统计
	totalAgents, maxDepth := countForestStats(roots)

	tree := &AgentTree{
		SessionID:   sess.ID,
		Root:        rootForCompat,
		Roots:       roots,
		TotalAgents: totalAgents,
		MaxDepth:    maxDepth,
	}

	return tree
}

// computeDepth 递归计算每个节点的深度
func computeDepth(node *AgentNode, depth int) {
	node.Depth = depth
	for _, child := range node.Children {
		computeDepth(child, depth+1)
	}
}

// countForestStats 统计森林的总节点数和最大深度
func countForestStats(roots []*AgentNode) (int, int) {
	total := 0
	maxDepth := 0
	for _, root := range roots {
		t, d := countTreeStats(root)
		total += t
		if d > maxDepth {
			maxDepth = d
		}
	}
	return total, maxDepth
}

// countTreeStats 统计树的总节点数和最大深度
func countTreeStats(node *AgentNode) (int, int) {
	if node == nil {
		return 0, 0
	}
	total := 1
	maxDepth := node.Depth
	for _, child := range node.Children {
		t, d := countTreeStats(child)
		total += t
		if d > maxDepth {
			maxDepth = d
		}
	}
	return total, maxDepth
}

// GetAgentDetail 从树中查找指定 AgentID 的节点
func GetAgentDetail(tree *AgentTree, agentID string) *AgentNode {
	if tree == nil {
		return nil
	}
	for _, root := range tree.Roots {
		if found := findNode(root, agentID); found != nil {
			return found
		}
	}
	return nil
}

func findNode(node *AgentNode, agentID string) *AgentNode {
	if node == nil {
		return nil
	}
	if node.AgentID == agentID || node.UUID == agentID {
		return node
	}
	for _, child := range node.Children {
		if found := findNode(child, agentID); found != nil {
			return found
		}
	}
	return nil
}

// ReadSession 读取会话文件并返回 Session 对象
func ReadSession(sessionPath string) (*session.Session, error) {
	reader := claude.NewReader()
	sess, err := reader.Read(sessionPath)
	if err != nil {
		return nil, fmt.Errorf("读取会话失败: %w", err)
	}
	return sess, nil
}
