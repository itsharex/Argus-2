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

// jsonlLine 表示 JSONL 文件中的一行
type jsonlLine struct {
	Type       string         `json:"type"`
	UUID       string         `json:"uuid"`
	ParentUUID string         `json:"parentUuid"`
	SessionID  string         `json:"sessionId"`
	Timestamp  string         `json:"timestamp"`
	CWD        string         `json:"cwd"`
	GitBranch  string         `json:"gitBranch"`
	Message    *message       `json:"message"`
}

type message struct {
	Role    string    `json:"role"`
	Content any       `json:"content"` // string 或 []contentBlock
	Model   string    `json:"model"`
	Usage   *usage    `json:"usage"` // token 使用情况
}

type usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
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

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event jsonlLine
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue // 跳过解析失败的行
		}

		// 解析时间戳
		ts, _ := time.Parse(time.RFC3339Nano, event.Timestamp)

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

		// 处理 assistant 消息（包含 tool_use）
		if event.Type == "assistant" && event.Message != nil {
			// 提取模型名称
			if event.Message.Model != "" && sess.Model == "" {
				sess.Model = event.Message.Model
			}

			// 提取 token 使用（从 message.usage 中获取）
			// 使用 UUID 去重，避免重复计算同一条消息的 token
			if event.Message.Usage != nil && event.UUID != "" && !seenMessageUUIDs[event.UUID] {
				seenMessageUUIDs[event.UUID] = true
				sess.TokenUsage.InputTokens += event.Message.Usage.InputTokens
				sess.TokenUsage.OutputTokens += event.Message.Usage.OutputTokens
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
		}

		// 处理 user 消息（提取用户提示）
		if event.Type == "user" && event.Message != nil {
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

// ReadLightweight 轻量级读取，只读取首尾行获取基本信息（用于列表展示）
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

	// 收集所有行用于读取最后几行
	var lines []string
	scanner := bufio.NewScanner(file)
	const maxScanTokenSize = 4 * 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxScanTokenSize)

	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			lines = append(lines, line)
		}
	}

	if len(lines) == 0 {
		return sess, nil
	}

	// 解析第一行获取基本信息
	var firstEvent jsonlLine
	if err := json.Unmarshal([]byte(lines[0]), &firstEvent); err == nil {
		if firstEvent.SessionID != "" {
			sess.ID = firstEvent.SessionID
		}
		if firstEvent.CWD != "" {
			sess.CWD = firstEvent.CWD
		}
		if firstEvent.GitBranch != "" {
			sess.GitBranch = firstEvent.GitBranch
		}
		ts, _ := time.Parse(time.RFC3339Nano, firstEvent.Timestamp)
		if !ts.IsZero() {
			sess.StartedAt = ts
		}
	}

	// 解析最后几行获取模型、token 使用、prompt 和结束时间
	var lastTime time.Time
	startIdx := 0
	if len(lines) > 10 {
		startIdx = len(lines) - 10 // 只读取最后 10 行
	}

	for i := startIdx; i < len(lines); i++ {
		var event jsonlLine
		if err := json.Unmarshal([]byte(lines[i]), &event); err != nil {
			continue
		}

		ts, _ := time.Parse(time.RFC3339Nano, event.Timestamp)
		if !ts.IsZero() {
			lastTime = ts
		}

		// 提取模型名称
		if event.Type == "assistant" && event.Message != nil {
			if event.Message.Model != "" && sess.Model == "" {
				sess.Model = event.Message.Model
			}
			// 累加 token 使用
			if event.Message.Usage != nil {
				sess.TokenUsage.InputTokens += event.Message.Usage.InputTokens
				sess.TokenUsage.OutputTokens += event.Message.Usage.OutputTokens
				sess.TokenUsage.TotalTokens = sess.TokenUsage.InputTokens + sess.TokenUsage.OutputTokens
			}
		}

		// 提取用户提示
		if event.Type == "user" && event.Message != nil && sess.Prompt == "" {
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
