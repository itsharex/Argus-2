package compliance

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Cache 管理审计结果缓存。
type Cache struct {
	mu      sync.RWMutex
	dir     string
	entries map[string]*CacheEntry
}

// CacheEntry 单条缓存记录。
type CacheEntry struct {
	RulesHash string           `json:"rulesHash"` // 规则列表的哈希
	Score     *ComplianceScore `json:"score"`      // 缓存的审计结果
}

// cacheData 整个缓存文件的数据结构。
type cacheData struct {
	Entries map[string]*CacheEntry `json:"entries"`
}

// NewCache 创建新的缓存管理器。
func NewCache() (*Cache, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户目录失败: %w", err)
	}

	dir := filepath.Join(homeDir, ".argus")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建缓存目录失败: %w", err)
	}

	cache := &Cache{
		dir:     dir,
		entries: make(map[string]*CacheEntry),
	}
	cache.load()

	return cache, nil
}

// Get 获取缓存的审计结果。
// rulesHash 用于检测规则是否变化，如果规则变了缓存失效。
func (c *Cache) Get(sessionID string, rulesHash string) (*ComplianceScore, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[sessionID]
	if !exists || entry.RulesHash != rulesHash {
		return nil, false
	}
	return entry.Score, true
}

// Set 缓存审计结果。
func (c *Cache) Set(sessionID string, rulesHash string, score *ComplianceScore) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[sessionID] = &CacheEntry{
		RulesHash: rulesHash,
		Score:     score,
	}
	c.save()
}

// Clear 清空缓存。
func (c *Cache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries = make(map[string]*CacheEntry)
	c.save()
}

// RulesHash 计算规则列表的哈希值。
func RulesHash(rules []ComplianceRule) string {
	data, _ := json.Marshal(rules)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:8])
}

// cacheFilePath 缓存文件路径。
func (c *Cache) cacheFilePath() string {
	return filepath.Join(c.dir, "compliance-cache.json")
}

// load 从磁盘加载缓存。
func (c *Cache) load() {
	data, err := os.ReadFile(c.cacheFilePath())
	if err != nil {
		return // 文件不存在或读取失败，使用空缓存
	}

	var cacheData cacheData
	if err := json.Unmarshal(data, &cacheData); err != nil {
		return // 解析失败，使用空缓存
	}

	if cacheData.Entries != nil {
		c.entries = cacheData.Entries
	}
}

// save 将缓存写入磁盘。
func (c *Cache) save() {
	// 如果 dir 为空（降级缓存），跳过持久化
	if c.dir == "" {
		return
	}
	data := cacheData{Entries: c.entries}
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return
	}
	os.WriteFile(c.cacheFilePath(), jsonData, 0644)
}
