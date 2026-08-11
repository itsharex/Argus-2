package search

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
)

// indexEntry 倒排索引条目
type indexEntry struct {
	SessionID  string `json:"s"`  // 会话 ID
	ProjectDir string `json:"p"`  // 项目目录名
	MatchType  string `json:"t"`  // 匹配类型
	Content    string `json:"c"`  // 匹配原文片段
	Timestamp  int64  `json:"ts"` // 时间戳（Unix 毫秒）
}

// invertedIndex 倒排索引
type invertedIndex struct {
	mu          sync.RWMutex
	entries     map[string][]indexEntry // keyword → 倒排列表
	documents   map[string]int64        // jsonlPath → modTime（已索引的文档）
	totalDocs   int                     // 已索引文档数
	builtAt     time.Time               // 最后构建时间
	indexFileSize int64                 // 索引文件大小
}

// newIndex 创建空倒排索引
func newIndex() *invertedIndex {
	return &invertedIndex{
		entries:   make(map[string][]indexEntry),
		documents: make(map[string]int64),
	}
}

// indexFileContent 单个 JSONL 文件的索引内容
type indexFileContent struct {
	Path    string       `json:"path"`
	ModTime int64        `json:"modTime"`
	Entries []indexEntry `json:"entries"`
}

// indexPersistData 索引持久化数据
type indexPersistData struct {
	Version   int                    `json:"version"`
	BuiltAt   int64                  `json:"builtAt"`
	Documents map[string]int64       `json:"documents"`
	Entries   map[string][]indexEntry `json:"entries"`
}

// tokenize 将文本拆分为 token（小写、去标点、去短词）
func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' {
			current.WriteRune(r)
		} else {
			if current.Len() >= 2 {
				tokens = append(tokens, current.String())
			}
			current.Reset()
		}
	}
	if current.Len() >= 2 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

// indexJSONLFile 索引单个 JSONL 文件
// 返回值：(新增条目数, error)
func (idx *invertedIndex) indexJSONLFile(jsonlPath, projectDir string) (int, error) {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// 检查文件是否已索引且未修改
	info, err := file.Stat()
	if err != nil {
		return 0, err
	}
	modTime := info.ModTime().UnixMilli()
	if lastMod, ok := idx.documents[jsonlPath]; ok && lastMod >= modTime {
		return 0, nil // 文件未修改，跳过
	}

	// 从文件名提取 sessionID
	sessionID := strings.TrimSuffix(filepath.Base(jsonlPath), ".jsonl")

	var entries []indexEntry
	scanner := bufio.NewScanner(file)
	const maxScanTokenSize = 4 * 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxScanTokenSize)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// 快速跳过非消息行（检查是否包含 "type" 和 "message"）
		if !strings.Contains(line, `"type"`) || !strings.Contains(line, `"message"`) {
			continue
		}

		var event struct {
			Type      string `json:"type"`
			Timestamp string `json:"timestamp"`
			Message   *struct {
				Role    string `json:"role"`
				Content any    `json:"content"`
				Model   string `json:"model"`
			} `json:"message"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		if event.Message == nil {
			continue
		}

		// 解析时间戳
		ts := parseTimestamp(event.Timestamp)

		// 提取可搜索内容
		switch event.Type {
		case "user", "message":
			if event.Message.Role == "user" || event.Type == "user" {
				texts := extractTexts(event.Message.Content)
				for _, text := range texts {
					if text == "" {
						continue
					}
					// prompt 类型（用户消息）权重更高
					entry := indexEntry{
						SessionID:  sessionID,
						ProjectDir: projectDir,
						MatchType:  "prompt",
						Content:    truncate(text, 200),
						Timestamp:  ts,
					}
					entries = append(entries, entry)

					// 同时作为 message 类型索引（用于搜索消息内容）
					msgEntry := indexEntry{
						SessionID:  sessionID,
						ProjectDir: projectDir,
						MatchType:  "message",
						Content:    truncate(text, 200),
						Timestamp:  ts,
					}
					entries = append(entries, msgEntry)
				}
			}

		case "assistant":
			if event.Message.Role == "assistant" || event.Type == "assistant" {
				texts := extractTexts(event.Message.Content)
				for _, text := range texts {
					if text == "" {
						continue
					}
					entry := indexEntry{
						SessionID:  sessionID,
						ProjectDir: projectDir,
						MatchType:  "message",
						Content:    truncate(text, 200),
						Timestamp:  ts,
					}
					entries = append(entries, entry)
				}

				// 提取 tool_use 中的 command 和 file_path
				extractToolEntries(event.Message.Content, sessionID, projectDir, ts, &entries)
			}
		}
	}

	// 将条目加入倒排索引
	count := 0
	for _, entry := range entries {
		textToIndex := ""
		switch entry.MatchType {
		case "prompt", "message":
			textToIndex = entry.Content
		case "command":
			textToIndex = entry.Content
		case "filepath":
			textToIndex = entry.Content
		}

		tokens := tokenize(textToIndex)
		for _, token := range tokens {
			idx.entries[token] = append(idx.entries[token], entry)
			count++
		}
	}

	// 更新文档记录
	idx.documents[jsonlPath] = modTime
	idx.totalDocs++

	return count, scanner.Err()
}

// extractTexts 从 message content 中提取文本
func extractTexts(content any) []string {
	if content == nil {
		return nil
	}

	switch c := content.(type) {
	case string:
		if c != "" {
			return []string{c}
		}
		return nil
	case []any:
		var texts []string
		for _, block := range c {
			blockMap, ok := block.(map[string]any)
			if !ok {
				continue
			}
			blockType, _ := blockMap["type"].(string)
			if blockType == "text" {
				if text, _ := blockMap["text"].(string); text != "" {
					texts = append(texts, text)
				}
			}
		}
		return texts
	}
	return nil
}

// extractToolEntries 从 tool_use 中提取 command 和 filepath 条目
func extractToolEntries(content any, sessionID, projectDir string, ts int64, entries *[]indexEntry) {
	if content == nil {
		return
	}

	blocks, ok := content.([]any)
	if !ok {
		return
	}

	for _, block := range blocks {
		blockMap, ok := block.(map[string]any)
		if !ok {
			continue
		}
		if blockType, _ := blockMap["type"].(string); blockType != "tool_use" {
			continue
		}

		input, ok := blockMap["input"].(map[string]any)
		if !ok {
			continue
		}

		// 提取 command
		if cmd, ok := input["command"].(string); ok && cmd != "" {
			*entries = append(*entries, indexEntry{
				SessionID:  sessionID,
				ProjectDir: projectDir,
				MatchType:  "command",
				Content:    truncate(cmd, 200),
				Timestamp:  ts,
			})
		}

		// 提取 file_path
		if fp, ok := input["file_path"].(string); ok && fp != "" {
			*entries = append(*entries, indexEntry{
				SessionID:  sessionID,
				ProjectDir: projectDir,
				MatchType:  "filepath",
				Content:    truncate(fp, 200),
				Timestamp:  ts,
			})
		}
	}
}

// truncate 截断字符串到指定长度
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// save 持久化索引到文件
func (idx *invertedIndex) save(path string) error {
	idx.mu.RLock()
	data := indexPersistData{
		Version:   1,
		BuiltAt:   idx.builtAt.UnixMilli(),
		Documents: idx.documents,
		Entries:   idx.entries,
	}
	idx.mu.RUnlock()

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 原子写入：先写临时文件，再 rename
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, jsonData, 0644); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath)
		return err
	}

	idx.mu.Lock()
	idx.indexFileSize = int64(len(jsonData))
	idx.mu.Unlock()

	return nil
}

// load 从文件加载索引
func (idx *invertedIndex) load(path string) error {
	fileData, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var data indexPersistData
	if err := json.Unmarshal(fileData, &data); err != nil {
		return err
	}

	if data.Version != 1 {
		return nil // 版本不兼容，忽略
	}

	idx.mu.Lock()
	defer idx.mu.Unlock()

	idx.entries = data.Entries
	idx.documents = data.Documents
	idx.totalDocs = len(data.Documents)
	idx.builtAt = time.UnixMilli(data.BuiltAt)
	idx.indexFileSize = int64(len(fileData))

	return nil
}

// parseTimestamp 解析时间戳，支持 RFC3339 和 Unix 纳秒
func parseTimestamp(s string) int64 {
	if s == "" {
		return 0
	}
	// 尝试 RFC3339Nano
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UnixMilli()
	}
	// 尝试 Unix 纳秒
	var ns int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			ns = ns*10 + int64(c-'0')
		} else {
			return 0
		}
	}
	if ns > 0 {
		// 判断是毫秒还是纳秒（大于 1e12 大概率是纳秒）
		if ns > 1e12 {
			return ns / 1e6
		}
		return ns
	}
	return 0
}
