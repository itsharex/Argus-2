package session

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// MetaStore 会话元数据存储管理器
type MetaStore struct {
	mu       sync.RWMutex
	filePath string
	data     map[string]*SessionMeta // key: sessionID
	tags     []Tag
}

// NewMetaStore 创建新的元数据存储管理器
func NewMetaStore() (*MetaStore, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户目录失败: %w", err)
	}

	// 确保目录存在
	configDir := filepath.Join(homeDir, ".argus")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("创建配置目录失败: %w", err)
	}

	store := &MetaStore{
		filePath: filepath.Join(configDir, "session_meta.json"),
		data:     make(map[string]*SessionMeta),
		tags:     make([]Tag, 0),
	}

	// 加载现有数据
	if err := store.load(); err != nil {
		// 如果文件不存在，初始化空数据
		if !os.IsNotExist(err) {
			return nil, err
		}
	}

	return store, nil
}

// load 从文件加载数据
func (s *MetaStore) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.ReadFile(s.filePath)
	if err != nil {
		return err
	}

	var wrapper struct {
		Meta map[string]*SessionMeta `json:"meta"`
		Tags []Tag                   `json:"tags"`
	}

	if err := json.Unmarshal(file, &wrapper); err != nil {
		return err
	}

	if wrapper.Meta != nil {
		s.data = wrapper.Meta
	}
	if wrapper.Tags != nil {
		s.tags = wrapper.Tags
	}

	return nil
}

// save 保存数据到文件（原子写入：先写临时文件，再 rename）
func (s *MetaStore) save() error {
	wrapper := struct {
		Meta map[string]*SessionMeta `json:"meta"`
		Tags []Tag                   `json:"tags"`
	}{
		Meta: s.data,
		Tags: s.tags,
	}

	data, err := json.MarshalIndent(wrapper, "", "  ")
	if err != nil {
		return err
	}

	// 原子写入：先写 .tmp 文件，再 rename，避免写入过程中崩溃导致数据损坏
	tmpPath := s.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpPath, s.filePath)
}

// GetMeta 获取会话元数据
func (s *MetaStore) GetMeta(sessionID string) (*SessionMeta, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	meta, ok := s.data[sessionID]
	return meta, ok
}

// GetOrCreateMeta 获取或创建会话元数据
func (s *MetaStore) GetOrCreateMeta(sessionID string) *SessionMeta {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, ok := s.data[sessionID]
	if !ok {
		meta = &SessionMeta{
			SessionID: sessionID,
			Tags:      []string{},
			AutoTags:  []string{},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		s.data[sessionID] = meta
	}

	return meta
}

// UpdateMeta 更新会话元数据
func (s *MetaStore) UpdateMeta(meta *SessionMeta) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta.UpdatedAt = time.Now()
	s.data[meta.SessionID] = meta

	return s.save()
}

// ============================================
// 收藏功能
// ============================================

// SetFavorite 设置收藏状态
func (s *MetaStore) SetFavorite(sessionID string, favorited bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, ok := s.data[sessionID]
	if !ok {
		meta = &SessionMeta{
			SessionID: sessionID,
			Tags:      []string{},
			AutoTags:  []string{},
			CreatedAt: time.Now(),
		}
		s.data[sessionID] = meta
	}

	meta.Favorited = favorited
	meta.UpdatedAt = time.Now()

	return s.save()
}

// GetFavorites 获取所有收藏的会话 ID
func (s *MetaStore) GetFavorites() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var favorites []string
	for id, meta := range s.data {
		if meta.Favorited {
			favorites = append(favorites, id)
		}
	}

	sort.Strings(favorites)
	return favorites
}

// ============================================
// 标签功能
// ============================================

// AddTag 为会话添加标签
func (s *MetaStore) AddTag(sessionID, tag string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, ok := s.data[sessionID]
	if !ok {
		meta = &SessionMeta{
			SessionID: sessionID,
			Tags:      []string{},
			AutoTags:  []string{},
			CreatedAt: time.Now(),
		}
		s.data[sessionID] = meta
	}

	// 检查标签是否已存在
	for _, t := range meta.Tags {
		if t == tag {
			return nil // 标签已存在
		}
	}

	meta.Tags = append(meta.Tags, tag)
	sort.Strings(meta.Tags)
	meta.UpdatedAt = time.Now()

	return s.save()
}

// RemoveTag 移除会话标签
func (s *MetaStore) RemoveTag(sessionID, tag string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, ok := s.data[sessionID]
	if !ok {
		return nil
	}

	// 查找并移除标签
	for i, t := range meta.Tags {
		if t == tag {
			meta.Tags = append(meta.Tags[:i], meta.Tags[i+1:]...)
			meta.UpdatedAt = time.Now()
			return s.save()
		}
	}

	return nil
}

// GetTags 获取会话的所有标签（手动 + 自动）
func (s *MetaStore) GetTags(sessionID string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	meta, ok := s.data[sessionID]
	if !ok {
		return []string{}
	}

	// 合并手动标签和自动标签
	allTags := make([]string, 0, len(meta.Tags)+len(meta.AutoTags))
	allTags = append(allTags, meta.Tags...)
	allTags = append(allTags, meta.AutoTags...)

	return allTags
}

// GetAllTags 获取所有已使用的标签
func (s *MetaStore) GetAllTags() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tagSet := make(map[string]bool)
	for _, meta := range s.data {
		for _, tag := range meta.Tags {
			tagSet[tag] = true
		}
		for _, tag := range meta.AutoTags {
			tagSet[tag] = true
		}
	}

	allTags := make([]string, 0, len(tagSet))
	for tag := range tagSet {
		allTags = append(allTags, tag)
	}

	sort.Strings(allTags)
	return allTags
}

// GetCustomTags 获取用户自定义标签列表
func (s *MetaStore) GetCustomTags() []Tag {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.tags
}

// AddCustomTag 添加自定义标签
func (s *MetaStore) AddCustomTag(tag Tag) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已存在
	for _, t := range s.tags {
		if t.Name == tag.Name {
			return fmt.Errorf("标签 '%s' 已存在", tag.Name)
		}
	}

	s.tags = append(s.tags, tag)
	return s.save()
}

// RemoveCustomTag 删除自定义标签
func (s *MetaStore) RemoveCustomTag(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, t := range s.tags {
		if t.Name == name {
			s.tags = append(s.tags[:i], s.tags[i+1:]...)
			return s.save()
		}
	}

	return nil
}

// ============================================
// 自动标签
// ============================================

// ApplyAutoTags 应用自动标签规则
func (s *MetaStore) ApplyAutoTags(sessionID, prompt string, filePaths []string, commands []string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, ok := s.data[sessionID]
	if !ok {
		meta = &SessionMeta{
			SessionID: sessionID,
			Tags:      []string{},
			AutoTags:  []string{},
			CreatedAt: time.Now(),
		}
		s.data[sessionID] = meta
	}

	newTags := make([]string, 0)
	tagSet := make(map[string]bool)
	for _, t := range meta.AutoTags {
		tagSet[t] = true
	}

	// 构建匹配文本
	var texts []string
	for _, field := range []string{"prompt", "filePath", "command"} {
		switch field {
		case "prompt":
			texts = append(texts, prompt)
		case "filePath":
			texts = append(texts, strings.Join(filePaths, " "))
		case "command":
			texts = append(texts, strings.Join(commands, " "))
		}
	}

	// 应用规则
	for _, rule := range DefaultAutoTags {
		// 检查是否匹配指定字段
		var matchText string
		for _, field := range rule.Fields {
			switch field {
			case "prompt":
				matchText += " " + prompt
			case "filePath":
				matchText += " " + strings.Join(filePaths, " ")
			case "command":
				matchText += " " + strings.Join(commands, " ")
			}
		}

		// 使用预编译的正则表达式
		if rule.CompiledRe != nil && rule.CompiledRe.MatchString(matchText) {
			if !tagSet[rule.Tag] {
				newTags = append(newTags, rule.Tag)
				tagSet[rule.Tag] = true
			}
		}
	}

	if len(newTags) > 0 {
		meta.AutoTags = append(meta.AutoTags, newTags...)
		sort.Strings(meta.AutoTags)
		meta.UpdatedAt = time.Now()
		if err := s.save(); err != nil {
			log.Printf("Failed to save auto tags for session %s: %v", sessionID, err)
		}
	}

	return newTags
}

// ============================================
// 搜索功能
// ============================================

// SearchableSession 可搜索的会话信息
type SearchableSession struct {
	ID          string
	Prompt      string
	Model       string
	Branch      string
	ProjectDir  string
}

// Search 搜索会话
func (s *MetaStore) Search(sessions []SearchableSession, query SearchQuery) []SearchResult {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make([]SearchResult, 0)

	for _, sess := range sessions {
		// 检查收藏过滤
		if query.Favorited != nil {
			meta, ok := s.data[sess.ID]
			if !ok || meta.Favorited != *query.Favorited {
				continue
			}
		}

		// 检查标签过滤
		if len(query.Tags) > 0 {
			meta, ok := s.data[sess.ID]
			if !ok {
				continue
			}
			if !hasAnyTag(meta, query.Tags) {
				continue
			}
		}

		// 检查项目过滤
		if len(query.Projects) > 0 {
			if !containsString(query.Projects, sess.ProjectDir) {
				continue
			}
		}

		// 关键词搜索
		if query.Keyword != "" {
			keyword := strings.ToLower(query.Keyword)
			var matches []string
			score := 0.0

			// 搜索 Prompt
			if len(query.Fields) == 0 || containsString(query.Fields, "prompt") {
				if strings.Contains(strings.ToLower(sess.Prompt), keyword) {
					matches = append(matches, "prompt:"+sess.Prompt)
					score += 10.0
				}
			}

			// 搜索文件路径（从 actions 中提取）
			if len(query.Fields) == 0 || containsString(query.Fields, "filePath") {
				// 文件路径信息在 SearchableSession 中没有，需要从 meta 或其他地方获取
				// 这里简化处理，只搜索已有的字段
			}

			// 搜索模型
			if strings.Contains(strings.ToLower(sess.Model), keyword) {
				matches = append(matches, "model:"+sess.Model)
				score += 5.0
			}

			// 搜索分支
			if strings.Contains(strings.ToLower(sess.Branch), keyword) {
				matches = append(matches, "branch:"+sess.Branch)
				score += 3.0
			}

			// 搜索标签
			if meta, ok := s.data[sess.ID]; ok {
				for _, tag := range meta.Tags {
					if strings.Contains(strings.ToLower(tag), keyword) {
						matches = append(matches, "tag:"+tag)
						score += 7.0
					}
				}
				for _, tag := range meta.AutoTags {
					if strings.Contains(strings.ToLower(tag), keyword) {
						matches = append(matches, "autoTag:"+tag)
						score += 4.0
					}
				}
			}

			if len(matches) == 0 {
				continue
			}

			results = append(results, SearchResult{
				SessionID: sess.ID,
				Matches:   matches,
				Score:     score,
			})
		} else {
			// 无关键词，返回所有（用于标签/收藏过滤）
			results = append(results, SearchResult{
				SessionID: sess.ID,
				Matches:   []string{},
				Score:     0,
			})
		}
	}

	// 按分数排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// hasAnyTag 检查是否有任意一个标签
func hasAnyTag(meta *SessionMeta, tags []string) bool {
	tagSet := make(map[string]bool)
	for _, t := range meta.Tags {
		tagSet[t] = true
	}
	for _, t := range meta.AutoTags {
		tagSet[t] = true
	}

	for _, t := range tags {
		if tagSet[t] {
			return true
		}
	}

	return false
}

// containsString 检查字符串切片是否包含指定字符串
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

// ============================================
// 备注功能
// ============================================

// SetNote 设置会话备注
func (s *MetaStore) SetNote(sessionID, note string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	meta, ok := s.data[sessionID]
	if !ok {
		meta = &SessionMeta{
			SessionID: sessionID,
			Tags:      []string{},
			AutoTags:  []string{},
			CreatedAt: time.Now(),
		}
		s.data[sessionID] = meta
	}

	meta.Note = note
	meta.UpdatedAt = time.Now()

	return s.save()
}

// GetNote 获取会话备注
func (s *MetaStore) GetNote(sessionID string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if meta, ok := s.data[sessionID]; ok {
		return meta.Note
	}
	return ""
}
