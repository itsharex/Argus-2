package claude

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"argus-desktop/internal/session"
)

// Reader 解析 Claude Code 会话日志
type Reader struct{}

// NewReader 创建新的 Claude Code Reader
func NewReader() *Reader {
	return &Reader{}
}

type contentBlock struct {
	Type    string         `json:"type"`
	Text    string         `json:"text,omitempty"`
	ID      string         `json:"id,omitempty"`
	Name    string         `json:"name,omitempty"`
	Input   map[string]any `json:"input,omitempty"`
	Content string         `json:"content,omitempty"` // tool_result 的结果
}

// Read 解析指定路径的 Claude Code JSONL 文件
func (r *Reader) Read(path string) (*session.Session, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	sess := &session.Session{
		AgentType: "claude-code",
	}

	// 追踪最后一条消息的时间戳，用于计算会话持续时间
	var lastTime time.Time

	scanner := bufio.NewScanner(file)
	// 使用 64KB 初始缓冲区，最大 4MB，避免 50MB 内存尖峰
	const maxScanTokenSize = 4 * 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxScanTokenSize)

	seenPrompts := make(map[string]bool)
	seenMessageUUIDs := make(map[string]bool) // 用于去重 token 统计
	skippedUUIDToParent := make(map[string]string) // 被跳过的消息 UUID → 其 parentUUID，用于修复父节点链

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event JSONLLine
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // 跳过解析失败的行
		}

		// 解析时间戳（支持 RFC3339 和 Unix 纳秒两种格式）
		ts := parseTimestamp(event.Timestamp)

		// 更新最后一条消息的时间戳
		if !ts.IsZero() {
			lastTime = ts
		}

		// 提取会话元数据
		if event.SessionID != "" && sess.ID == "" {
			sess.ID = event.SessionID
		}
		if event.CWD != "" && sess.CWD == "" {
			sess.CWD = event.CWD
		}
		if event.GitBranch != "" && sess.GitBranch == "" {
			sess.GitBranch = event.GitBranch
		}
		if sess.StartedAt.IsZero() && !ts.IsZero() {
			sess.StartedAt = ts
		}

		// 跟踪被跳过的消息（如 attachment、queue-operation 等）
		// 这些消息的 UUID 可能被其他消息引用为 parentUuid，需要记录映射关系
		if event.UUID != "" && event.ParentUUID != "" && event.ParentUUID != "null" {
			// 判断是否为被跳过的消息类型（非 user、非 assistant）
			isSkipped := event.Type != "user" && event.Type != "assistant" &&
				!(event.Type == "message" && event.Message != nil)
			if isSkipped {
				skippedUUIDToParent[event.UUID] = event.ParentUUID
			}
		}

		// 处理 assistant 消息（包含 tool_use）
		// 支持 type == "assistant" 和 type == "message" (role == "assistant")
		isAssistant := (event.Type == "assistant") || (event.Type == "message" && event.Message != nil && event.Message.Role == "assistant")
		if isAssistant && event.Message != nil {
			// 标准化 ParentUUID（null 视为空字符串）
			if event.ParentUUID == "null" {
				event.ParentUUID = ""
			}
			// 提取模型名称
			if event.Message.Model != "" && sess.Model == "" {
				sess.Model = event.Message.Model
			}

			// 提取 token 使用（从 message.usage 中获取）
			// 使用 UUID 去重，避免重复计算同一条消息的 token
			// 对于子 Agent，使用 agentId:uuid 作为去重 key，避免与主 Agent 冲突
			dedupKey := event.UUID
			if event.AgentID != "" {
				dedupKey = event.AgentID + ":" + event.UUID
			}
			if event.Message.Usage != nil && dedupKey != "" && !seenMessageUUIDs[dedupKey] {
				seenMessageUUIDs[dedupKey] = true
				sess.TokenUsage.InputTokens += event.Message.Usage.InputTokens
				sess.TokenUsage.OutputTokens += event.Message.Usage.OutputTokens
				sess.TokenUsage.CacheReadTokens += event.Message.Usage.CacheReadTokens
				sess.TokenUsage.CacheCreationTokens += event.Message.Usage.CacheCreationTokens
				sess.TokenUsage.TotalTokens = sess.TokenUsage.InputTokens + sess.TokenUsage.OutputTokens
			}

			// 解析 content blocks
			if blocks, ok := event.Message.Content.([]any); ok {
				for _, block := range blocks {
					blockMap, ok := block.(map[string]any)
					if !ok {
						continue
					}
					r.parseContentBlock(sess, blockMap, ts)
				}
			}

			// 构建 assistant 消息
			msg := session.Message{
				ID:        event.UUID,
				Type:      session.MessageTypeAssistant,
				Model:     event.Message.Model,
				Timestamp: ts,
				Content:   r.parseMessageContent(event.Message.Content),
			}
			sess.Messages = append(sess.Messages, msg)

			// 收集 Agent 元数据（用于构建 Agent 树）
			agentInfo := session.AgentInfo{
				UUID:       event.UUID,
				AgentID:    event.AgentID,
				ParentUUID: event.ParentUUID,
				Type:       "assistant",
				Timestamp:  ts,
			}
			if event.Message.Usage != nil {
				agentInfo.InputTokens = event.Message.Usage.InputTokens
				agentInfo.OutputTokens = event.Message.Usage.OutputTokens
			}
			// 统计 tool_use 调用次数
			if blocks, ok := event.Message.Content.([]any); ok {
				for _, block := range blocks {
					if blockMap, ok := block.(map[string]any); ok {
						if blockType, _ := blockMap["type"].(string); blockType == "tool_use" {
							agentInfo.ToolCalls++
						}
					}
				}
			}
			sess.Agents = append(sess.Agents, agentInfo)
		}

		// 处理 user 消息（提取用户提示）
		// 支持 type == "user" 和 type == "message" (role == "user")
		isUser := (event.Type == "user") || (event.Type == "message" && event.Message != nil && event.Message.Role == "user")
		if isUser && event.Message != nil {
			// 标准化 ParentUUID（null 视为空字符串）
			if event.ParentUUID == "null" {
				event.ParentUUID = ""
			}
			// content 可能是 string 或 []contentBlock
			switch content := event.Message.Content.(type) {
			case string:
				if !seenPrompts[content] {
					if sess.Prompt == "" {
						sess.Prompt = content
					}
					seenPrompts[content] = true
				}
				// 构建 user 消息
				msg := session.Message{
					ID:        event.UUID,
					Type:      session.MessageTypeUser,
					Timestamp: ts,
					Content: []session.ContentBlock{
						{Type: session.ContentTypeText, Text: content},
					},
				}
				sess.Messages = append(sess.Messages, msg)

				// 收集 Agent 元数据
				sess.Agents = append(sess.Agents, session.AgentInfo{
					UUID:       event.UUID,
					AgentID:    event.AgentID,
					ParentUUID: event.ParentUUID,
					Type:       "user",
					Timestamp:  ts,
				})
			case []any:
				// 数组格式，提取第一个 text 类型的内容
				for _, block := range content {
					if blockMap, ok := block.(map[string]any); ok {
						if blockType, _ := blockMap["type"].(string); blockType == "text" {
							if text, _ := blockMap["text"].(string); text != "" && !seenPrompts[text] {
								if sess.Prompt == "" {
									sess.Prompt = text
								}
								seenPrompts[text] = true
								break
							}
						}
					}
				}
				// 构建 user 消息（包含所有 content blocks）
				msg := session.Message{
					ID:        event.UUID,
					Type:      session.MessageTypeUser,
					Timestamp: ts,
					Content:   r.parseUserContent(content),
				}
				sess.Messages = append(sess.Messages, msg)

				// 收集 Agent 元数据
				sess.Agents = append(sess.Agents, session.AgentInfo{
					UUID:       event.UUID,
					AgentID:    event.AgentID,
					ParentUUID: event.ParentUUID,
					Type:       "user",
					Timestamp:  ts,
				})
			}
		}
	}

	// 修复父节点链：将被跳过的消息（如 attachment）的 UUID 映射到其 parentUUID
	// 这样引用被跳过消息的子节点可以正确找到其实际祖先
	if len(skippedUUIDToParent) > 0 {
		for i := range sess.Agents {
			a := &sess.Agents[i]
			// 循环查找，直到找到一个未被跳过的父节点
			visited := make(map[string]bool)
			for {
				if a.ParentUUID == "" || visited[a.ParentUUID] {
					break
				}
				if parentID, ok := skippedUUIDToParent[a.ParentUUID]; ok {
					visited[a.ParentUUID] = true
					a.ParentUUID = parentID
				} else {
					break
				}
			}
		}
	}

	// 计算持续时间
	if sess.StartedAt.IsZero() {
		sess.StartedAt = time.Now()
	}

	// 使用最后一条消息的时间戳计算会话持续时间
	// 如果没有有效的时间戳，回退到当前时间
	if lastTime.IsZero() || lastTime.Before(sess.StartedAt) {
		sess.Duration = time.Since(sess.StartedAt)
	} else {
		sess.Duration = lastTime.Sub(sess.StartedAt)
	}

	return sess, scanner.Err()
}

func (r *Reader) parseContentBlock(sess *session.Session, blockMap map[string]any, ts time.Time) {
	blockType, _ := blockMap["type"].(string)

	switch blockType {
	case "tool_use":
		id, _ := blockMap["id"].(string)
		name, _ := blockMap["name"].(string)
		input, _ := blockMap["input"].(map[string]any)

		action := session.Action{
			ID:        id,
			Type:      mapToolType(name),
			FilePath:  extractFilePath(name, input),
			Input:     input,
			Timestamp: ts,
		}

		// 从 input 中提取描述
		if cmd, ok := input["command"].(string); ok {
			action.Description = cmd
		}
		if content, ok := input["content"].(string); ok {
			if len(content) > 100 {
				action.Description = content[:100] + "..."
			} else {
				action.Description = content
			}
		}
		if content, ok := input["file_path"].(string); ok {
			if action.Description == "" {
				action.Description = content
			}
		}

		sess.Actions = append(sess.Actions, action)

	case "thinking":
		// thinking block，暂时忽略
	}
}

func mapToolType(name string) session.ActionType {
	switch strings.ToLower(name) {
	case "write":
		return session.ActionWrite
	case "edit":
		return session.ActionEdit
	case "bash", "execute":
		return session.ActionBash
	case "read":
		return session.ActionRead
	case "grep", "rg":
		return session.ActionGrep
	case "glob":
		return session.ActionGlob
	default:
		return session.ActionOther
	}
}

func extractFilePath(toolName string, input map[string]any) string {
	switch strings.ToLower(toolName) {
	case "write":
		if fp, ok := input["file_path"].(string); ok {
			return fp
		}
	case "edit":
		if fp, ok := input["file_path"].(string); ok {
			return fp
		}
	case "read":
		if fp, ok := input["file_path"].(string); ok {
			return fp
		}
	}
	return ""
}

// parseMessageContent 解析 assistant 消息的 content blocks
func (r *Reader) parseMessageContent(content any) []session.ContentBlock {
	// 处理字符串类型的 content
	if text, ok := content.(string); ok {
		return []session.ContentBlock{
			{Type: session.ContentTypeText, Text: text},
		}
	}

	// 处理数组类型的 content
	blocks, ok := content.([]any)
	if !ok {
		return nil
	}

	var result []session.ContentBlock
	for _, block := range blocks {
		blockMap, ok := block.(map[string]any)
		if !ok {
			continue
		}

		blockType, _ := blockMap["type"].(string)
		switch blockType {
		case "thinking":
			thinking, _ := blockMap["thinking"].(string)
			result = append(result, session.ContentBlock{
				Type:     session.ContentTypeThinking,
				Thinking: thinking,
			})
		case "text":
			text, _ := blockMap["text"].(string)
			result = append(result, session.ContentBlock{
				Type: session.ContentTypeText,
				Text: text,
			})
		case "tool_use":
			name, _ := blockMap["name"].(string)
			id, _ := blockMap["id"].(string)
			result = append(result, session.ContentBlock{
				Type:     session.ContentTypeToolUse,
				ToolName: name,
				ToolID:   id,
				Input:    blockMap["input"],
			})
		}
	}
	return result
}

// parseUserContent 解析 user 消息的 content blocks
func (r *Reader) parseUserContent(content any) []session.ContentBlock {
	blocks, ok := content.([]any)
	if !ok {
		return nil
	}

	var result []session.ContentBlock
	for _, block := range blocks {
		blockMap, ok := block.(map[string]any)
		if !ok {
			continue
		}

		blockType, _ := blockMap["type"].(string)
		switch blockType {
		case "text":
			text, _ := blockMap["text"].(string)
			result = append(result, session.ContentBlock{
				Type: session.ContentTypeText,
				Text: text,
			})
		case "tool_result":
			toolID, _ := blockMap["tool_use_id"].(string)
			// tool_result 的 content 可以是字符串或内容块数组
			var resultContent string
			switch c := blockMap["content"].(type) {
			case string:
				resultContent = c
			case []any:
				// 从数组中提取文本
				for _, item := range c {
					if m, ok := item.(map[string]any); ok {
						if text, ok := m["text"].(string); ok {
							resultContent += text
						}
					}
				}
			}
			result = append(result, session.ContentBlock{
				Type:   session.ContentTypeToolResult,
				ToolID: toolID,
				Result: resultContent,
			})
		}
	}
	return result
}

// parseTimestamp 解析时间戳，支持 RFC3339 和 Unix 纳秒两种格式
func parseTimestamp(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	// 先尝试 RFC3339Nano
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t
	}
	// 尝试 Unix 纳秒（纯数字字符串）
	var ns int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			ns = ns*10 + int64(c-'0')
		} else {
			return time.Time{} // 包含非数字字符，不是 Unix 时间戳
		}
	}
	if ns > 0 {
		return time.Unix(0, ns)
	}
	return time.Time{}
}

// DetectFormat 检测文件是否为 Claude Code JSONL 格式
func (r *Reader) DetectFormat(path string) bool {
	// 检查文件扩展名
	ext := filepath.Ext(path)
	if ext != ".jsonl" {
		return false
	}

	// 检查父目录名是否像是 Claude 项目目录
	dir := filepath.Dir(path)
	dirName := filepath.Base(dir)
	if !strings.HasPrefix(dirName, "-") {
		return false
	}

	// 尝试读取前几行，检查是否包含 Claude Code 的特征字段
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// 使用 64KB 初始缓冲区，最大 4MB，避免 JSONL 行过长导致 token too long 错误
	const maxScanTokenSize = 4 * 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxScanTokenSize)

	linesChecked := 0
	for scanner.Scan() && linesChecked < 5 {
		line := scanner.Text()
		if line == "" {
			continue
		}
		linesChecked++

		var event map[string]any
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		// Claude Code 的 JSONL 通常有 sessionId, type, message 字段
		if _, hasSession := event["sessionId"]; hasSession {
			if _, hasType := event["type"]; hasType {
				return true
			}
		}
	}

	return false
}

// ReadLightweight 轻量级读取，只读取首行和文件尾部获取基本信息（用于列表展示）
// 比 Read 快很多，适合只需要基本信息的场景
func (r *Reader) ReadLightweight(path string) (*session.Session, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	sess := &session.Session{
		AgentType: "claude-code",
	}

	scanner := bufio.NewScanner(file)
	const maxScanTokenSize = 4 * 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxScanTokenSize)

	// 只读取第一行获取会话元数据
	if !scanner.Scan() {
		return sess, nil
	}
	firstLine := scanner.Text()
	if firstLine == "" {
		return sess, nil
	}

	var firstEvent JSONLLine
	if err := json.Unmarshal([]byte(firstLine), &firstEvent); err == nil {
		if firstEvent.SessionID != "" {
			sess.ID = firstEvent.SessionID
		}
		if firstEvent.CWD != "" {
			sess.CWD = firstEvent.CWD
		}
		if firstEvent.GitBranch != "" {
			sess.GitBranch = firstEvent.GitBranch
		}
		ts := parseTimestamp(firstEvent.Timestamp)
		if !ts.IsZero() {
			sess.StartedAt = ts
		}
	}

	// 从文件尾部读取最后 N KB 获取模型、token 使用和 prompt 信息
	const tailSize = 64 * 1024 // 读取最后 64KB
	fi, err := file.Stat()
	if err != nil {
		return sess, nil
	}

	var tailData []byte
	if fi.Size() <= tailSize {
		// 文件较小，读取整个剩余部分
		rest := make([]byte, int(fi.Size()))
		n, _ := file.ReadAt(rest, 0)
		tailData = rest[:n]
	} else {
		// 从文件末尾读取 tailSize 字节
		buf := make([]byte, tailSize)
		n, err := file.ReadAt(buf, fi.Size()-tailSize)
		if err != nil {
			return sess, scanner.Err()
		}
		tailData = buf[:n]
	}

	// 从尾部数据中提取最后几行完整的 JSON 行
	tailLines := strings.Split(string(tailData), "\n")
	// 跳过第一行（可能是不完整的部分行），从第二行开始处理
	startIdx := 0
	if fi.Size() > tailSize {
		startIdx = 1 // 跳过第一行不完整的行
	}

	var lastTime time.Time
	for i := startIdx; i < len(tailLines); i++ {
		line := tailLines[i]
		if line == "" {
			continue
		}

		var event JSONLLine
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		ts := parseTimestamp(event.Timestamp)
		if !ts.IsZero() {
			lastTime = ts
		}

		// 提取模型名称（支持 type: "assistant" 和 type: "message" 两种格式）
		isAssistant := (event.Type == "assistant") ||
			(event.Type == "message" && event.Message != nil && event.Message.Role == "assistant")
		if isAssistant && event.Message != nil {
			if event.Message.Model != "" && sess.Model == "" {
				sess.Model = event.Message.Model
			}
			// 累加 token 使用
			if event.Message.Usage != nil {
				sess.TokenUsage.InputTokens += event.Message.Usage.InputTokens
				sess.TokenUsage.OutputTokens += event.Message.Usage.OutputTokens
				sess.TokenUsage.CacheReadTokens += event.Message.Usage.CacheReadTokens
				sess.TokenUsage.CacheCreationTokens += event.Message.Usage.CacheCreationTokens
				sess.TokenUsage.TotalTokens = sess.TokenUsage.InputTokens + sess.TokenUsage.OutputTokens
			}
		}

		// 提取用户提示（支持 type: "user" 和 type: "message" 两种格式）
		isUser := (event.Type == "user") ||
			(event.Type == "message" && event.Message != nil && event.Message.Role == "user")
		if isUser && event.Message != nil && sess.Prompt == "" {
			switch content := event.Message.Content.(type) {
			case string:
				sess.Prompt = content
			case []any:
				for _, block := range content {
					if blockMap, ok := block.(map[string]any); ok {
						if blockType, _ := blockMap["type"].(string); blockType == "text" {
							if text, _ := blockMap["text"].(string); text != "" {
								sess.Prompt = text
								break
							}
						}
					}
				}
			}
		}
	}

	// 计算持续时间
	if sess.StartedAt.IsZero() {
		sess.StartedAt = time.Now()
	}
	if lastTime.IsZero() || lastTime.Before(sess.StartedAt) {
		sess.Duration = time.Since(sess.StartedAt)
	} else {
		sess.Duration = lastTime.Sub(sess.StartedAt)
	}

	return sess, scanner.Err()
}
