package search

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"argus-desktop/internal/session"
)

// Engine 全文搜索引擎
type Engine struct {
	mu      sync.RWMutex
	index   *invertedIndex
	dataDir string // ~/.argus/
}

// NewEngine 创建搜索引擎，加载已有索引
func NewEngine() (*Engine, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户目录失败: %w", err)
	}

	dataDir := filepath.Join(homeDir, ".argus")
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	e := &Engine{
		index:   newIndex(),
		dataDir: dataDir,
	}

	// 尝试加载已有索引
	indexPath := filepath.Join(dataDir, "search_index.json")
	if err := e.index.load(indexPath); err == nil {
		// 加载成功，检查是否需要重建
		if e.index.totalDocs > 0 {
			return e, nil
		}
	}

	// 无索引或加载失败，执行首次构建
	if err := e.RebuildIndex(); err != nil {
		// 构建失败不影响使用，只是搜索结果为空
		fmt.Printf("WARN: 搜索索引构建失败: %v\n", err)
	}

	return e, nil
}

// RebuildIndex 重建搜索索引
func (e *Engine) RebuildIndex() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	newIndex := newIndex()

	// 遍历所有 JSONL 文件
	err := session.WalkAllJSONLFiles(func(info session.JSONLFileInfo) error {
		_, err := newIndex.indexJSONLFile(info.JSONLPath, info.ProjectDir)
		if err != nil {
			// 单个文件失败不影响整体索引
			fmt.Printf("WARN: 索引文件失败 %s: %v\n", info.JSONLPath, err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("遍历会话文件失败: %w", err)
	}

	newIndex.builtAt = time.Now()
	e.index = newIndex

	// 持久化索引
	indexPath := filepath.Join(e.dataDir, "search_index.json")
	if err := e.index.save(indexPath); err != nil {
		return fmt.Errorf("保存索引失败: %w", err)
	}

	return nil
}

// Search 执行全文搜索
func (e *Engine) Search(req SearchRequest) ([]FullTextResult, error) {
	if req.Keyword == "" {
		return nil, fmt.Errorf("搜索关键词不能为空")
	}

	limit := req.Limit
	if limit <= 0 {
		limit = defaultLimit
	}

	// 构建过滤器
	filter := SearchFilter{
		matchTypes: make(map[string]bool),
		projects:   make(map[string]bool),
		limit:      limit,
	}
	for _, mt := range req.MatchTypes {
		filter.matchTypes[mt] = true
	}
	for _, p := range req.Projects {
		filter.projects[p] = true
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	// 分词搜索
	keyword := strings.ToLower(req.Keyword)
	tokens := tokenize(keyword)

	if len(tokens) == 0 {
		return nil, fmt.Errorf("搜索关键词无效")
	}

	// 收集所有匹配的条目（去重）
	entryScores := make(map[string]*FullTextResult) // key: sessionID+matchType+content

	for _, token := range tokens {
		entries, ok := e.index.entries[token]
		if !ok {
			continue
		}

		for _, entry := range entries {
			// 应用过滤器
			if len(filter.matchTypes) > 0 && !filter.matchTypes[entry.MatchType] {
				continue
			}
			if len(filter.projects) > 0 && !filter.projects[entry.ProjectDir] {
				continue
			}

			// 生成去重 key
			dedupKey := entry.SessionID + "|" + entry.MatchType + "|" + entry.Content

			if existing, ok := entryScores[dedupKey]; ok {
				// 已存在，累加分数
				existing.Score += matchTypeWeights[entry.MatchType]
			} else {
				entryScores[dedupKey] = &FullTextResult{
					SessionID:   entry.SessionID,
					ProjectDir:  entry.ProjectDir,
					MatchType:   entry.MatchType,
					MatchContent: entry.Content,
					Score:       matchTypeWeights[entry.MatchType],
					Timestamp:   entry.Timestamp,
				}
			}
		}
	}

	// 转换为切片并排序
	results := make([]FullTextResult, 0, len(entryScores))
	for _, r := range entryScores {
		results = append(results, *r)
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Score != results[j].Score {
			return results[i].Score > results[j].Score
		}
		// 分数相同按时间倒序
		return results[i].Timestamp > results[j].Timestamp
	})

	// 截断到 limit
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// GetStatus 获取索引状态
func (e *Engine) GetStatus() (*IndexStatus, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	indexPath := filepath.Join(e.dataDir, "search_index.json")
	var fileSize int64
	if info, err := os.Stat(indexPath); err == nil {
		fileSize = info.Size()
	}

	return &IndexStatus{
		TotalDocuments: e.index.totalDocs,
		TotalEntries:   len(e.index.entries),
		LastBuiltAt:    e.index.builtAt.UnixMilli(),
		IndexSizeBytes: fileSize,
	}, nil
}
