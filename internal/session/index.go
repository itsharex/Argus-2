package session

import (
	"fmt"
	"sync"
	"time"

	"argus-desktop/internal/i18n"
)

// IndexEntry 会话索引条目
type IndexEntry struct {
	SessionID   string
	JSONLPath   string
	ProjectDir  string
	ProjectPath string
	ModTime     time.Time
}

// Index 轻量级会话索引，避免每次 API 调用都遍历所有 JSONL 文件
type Index struct {
	mu      sync.RWMutex
	entries map[string]*IndexEntry // sessionID → entry
	ordered []string               // 按修改时间倒序排列的 sessionID
	builtAt time.Time
}

// NewIndex 创建空索引
func NewIndex() *Index {
	return &Index{
		entries: make(map[string]*IndexEntry),
	}
}

// Build 构建索引（遍历所有 JSONL 文件，仅读取文件名和修改时间）
func (idx *Index) Build() error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	newEntries := make(map[string]*IndexEntry)
	var newOrdered []string

	err := WalkAllJSONLFiles(func(info JSONLFileInfo) error {
		entry := &IndexEntry{
			SessionID:   info.FileName,
			JSONLPath:   info.JSONLPath,
			ProjectDir:  info.ProjectDir,
			ProjectPath: info.ProjectPath,
			ModTime:     info.ModTime,
		}
		newEntries[info.FileName] = entry
		newOrdered = append(newOrdered, info.FileName)
		return nil
	})
	if err != nil {
		return fmt.Errorf("%s: %w", i18n.ErrBuildSessionIndex(), err)
	}

	// 按修改时间倒序排列
	sortByModTime(newEntries, newOrdered)

	idx.entries = newEntries
	idx.ordered = newOrdered
	idx.builtAt = time.Now()

	return nil
}

// Get 根据 sessionID 查找索引条目（O(1)）
func (idx *Index) Get(sessionID string) (*IndexEntry, bool) {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	entry, ok := idx.entries[sessionID]
	return entry, ok
}

// GetPath 根据 sessionID 获取文件路径（O(1)）
func (idx *Index) GetPath(sessionID string) (string, bool) {
	entry, ok := idx.Get(sessionID)
	if !ok {
		return "", false
	}
	return entry.JSONLPath, true
}

// All 返回所有索引条目（按修改时间倒序）
func (idx *Index) All() []*IndexEntry {
	idx.mu.RLock()
	defer idx.mu.RUnlock()

	result := make([]*IndexEntry, 0, len(idx.ordered))
	for _, id := range idx.ordered {
		if entry, ok := idx.entries[id]; ok {
			result = append(result, entry)
		}
	}
	return result
}

// Len 返回索引中的会话数量
func (idx *Index) Len() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.entries)
}

// BuiltAt 返回索引构建时间
func (idx *Index) BuiltAt() time.Time {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return idx.builtAt
}

// sortByModTime 按修改时间倒序排列 sessionID 列表
func sortByModTime(entries map[string]*IndexEntry, ids []string) {
	for i := 0; i < len(ids)-1; i++ {
		for j := i + 1; j < len(ids); j++ {
			if entries[ids[j]].ModTime.After(entries[ids[i]].ModTime) {
				ids[i], ids[j] = ids[j], ids[i]
			}
		}
	}
}
