// Package knowledge provides knowledge management capabilities for
// organizing and managing Claude Code's plans, memory, and other markdown documents.
package knowledge

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// DocType 文档类型
type DocType string

const (
	DocTypePlans    DocType = "plans"
	DocTypeMemory   DocType = "memory"
	DocTypeClaudeMD DocType = "claudemd"
)

// KnowledgeDoc 知识文档
type KnowledgeDoc struct {
	Path        string            `json:"path"`        // 文件路径
	Name        string            `json:"name"`        // 显示名称
	Type        DocType           `json:"type"`        // 文档类型
	Project     string            `json:"project"`     // 所属项目
	Content     string            `json:"content"`     // Markdown 内容
	Frontmatter map[string]string `json:"frontmatter"` // YAML frontmatter
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
	Size        int64             `json:"size"`
}

// SearchFilters 搜索筛选条件
type SearchFilters struct {
	Types    []DocType `json:"types,omitempty"`    // 按类型筛选
	Projects []string  `json:"projects,omitempty"` // 按项目筛选
}

// Engine 知识管理引擎
type Engine struct {
	homeDir      string
	mu           sync.RWMutex
	projectRoots []string // 缓存的项目根目录列表
}

// NewEngine 创建新的知识管理引擎
func NewEngine() (*Engine, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return &Engine{
		homeDir: homeDir,
	}, nil
}

// GetAllDocuments 获取所有文档
func (e *Engine) GetAllDocuments(docType string, project string) ([]KnowledgeDoc, error) {
	var docs []KnowledgeDoc

	// 判断是否需要扫描 plans
	scanPlans := docType == "" || docType == "all" || docType == string(DocTypePlans)
	// 判断是否需要扫描 memory
	scanMemory := docType == "" || docType == "all" || docType == string(DocTypeMemory)
	// 判断是否需要扫描 CLAUDE.md
	scanClaudeMD := docType == "" || docType == "all" || docType == string(DocTypeClaudeMD)

	// 扫描 plans/
	if scanPlans {
		plansDocs, err := e.scanPlans()
		if err == nil {
			docs = append(docs, plansDocs...)
		}
	}

	// 扫描 projects/*/memory/
	if scanMemory {
		memoryDocs, err := e.scanMemory(project)
		if err == nil {
			docs = append(docs, memoryDocs...)
		}
	}

	// 扫描 CLAUDE.md
	if scanClaudeMD {
		claudeMDDocs, err := e.scanClaudeMD()
		if err == nil {
			docs = append(docs, claudeMDDocs...)
		}
	}

	// 按修改时间倒序排列
	sort.Slice(docs, func(i, j int) bool {
		return docs[i].UpdatedAt.After(docs[j].UpdatedAt)
	})

	return docs, nil
}

// GetDocument 获取单个文档
func (e *Engine) GetDocument(path string) (*KnowledgeDoc, error) {
	return e.readDocument(path)
}

// SaveDocument 保存文档
func (e *Engine) SaveDocument(path string, content string) error {
	// 验证路径安全性
	if err := e.validatePath(path); err != nil {
		return err
	}

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 写入文件
	return os.WriteFile(path, []byte(content), 0644)
}

// DeleteDocument 删除文档
func (e *Engine) DeleteDocument(path string) error {
	// 验证路径安全性
	if err := e.validatePath(path); err != nil {
		return err
	}

	return os.Remove(path)
}

// RenameDocument 重命名文档（更新 frontmatter 中的 name 字段，并重命名磁盘上的文件）
// 返回新的文件路径
func (e *Engine) RenameDocument(path string, newName string) (string, error) {
	// 验证路径安全性
	if err := e.validatePath(path); err != nil {
		return "", err
	}

	// 读取文件内容
	content, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	contentStr := string(content)

	// 使用 ParseFrontmatter 解析 YAML frontmatter
	_, body := ParseFrontmatter(contentStr)

	// 如果文件没有 frontmatter（body == contentStr），返回错误
	if body == contentStr {
		return "", fmt.Errorf("文件没有有效的 frontmatter")
	}

	// 找到 frontmatter 结束位置（第二个 ---）
	fmEnd := strings.Index(contentStr[3:], "---")
	if fmEnd == -1 {
		return "", fmt.Errorf("文件没有有效的 frontmatter")
	}
	fmEnd += 3 // 补偿前面跳过的3个字符
	fmEnd += 3 // 包括结束的 ---

	fmSection := contentStr[0:fmEnd]
	afterFM := contentStr[fmEnd:]

	// 替换或添加 frontmatter 中的 name 字段
	lines := strings.Split(fmSection, "\n")
	var newFM []string
	nameFound := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 检查顶层 name 字段（不以空格或 tab 开头）
		if !strings.HasPrefix(line, " ") && !strings.HasPrefix(line, "\t") {
			parts := strings.SplitN(trimmed, ":", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == "name" {
				newFM = append(newFM, "name: "+newName)
				nameFound = true
				continue
			}
		}
		newFM = append(newFM, line)
	}

	// 如果没找到 name 字段，在 --- 后插入
	if !nameFound {
		// 在第一行 --- 之后插入
		rest := make([]string, len(newFM))
		copy(rest, newFM)
		newFM = []string{newFM[0], "name: " + newName}
		newFM = append(newFM, rest[1:]...)
	}

	updatedContent := strings.Join(newFM, "\n") + afterFM

	// 写入文件内容（更新 frontmatter）
	if err := os.WriteFile(path, []byte(updatedContent), 0644); err != nil {
		return "", fmt.Errorf("更新文件内容失败: %w", err)
	}

	// 生成新的文件名（基于 newName，使用 kebab-case 格式）
	newFilename := strings.ToLower(strings.ReplaceAll(newName, " ", "-")) + ".md"
	dir := filepath.Dir(path)
	newPath := filepath.Join(dir, newFilename)

	// 如果新旧路径相同，不需要重命名
	if path == newPath {
		return path, nil
	}

	// 检查目标文件是否已存在
	if _, err := os.Stat(newPath); err == nil {
		return "", fmt.Errorf("目标文件已存在: %s", newFilename)
	}

	// 重命名磁盘上的文件
	if err := os.Rename(path, newPath); err != nil {
		return "", fmt.Errorf("重命名文件失败: %w", err)
	}

	return newPath, nil
}

// validatePath 验证文件路径是否在允许的目录内
func (e *Engine) validatePath(path string) error {
	// 解析路径为绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}

	// 检查路径是否在允许的目录内
	allowedDirs := []string{
		filepath.Join(e.homeDir, ".claude"),
	}

	// 添加缓存的项目根目录
	e.mu.RLock()
	allowedDirs = append(allowedDirs, e.projectRoots...)
	e.mu.RUnlock()

	for _, dir := range allowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		// 使用 filepath.Clean 标准化路径，确保分隔符一致
		// 追加分隔符避免前缀匹配绕过（如 /home/user/.claude-evil 不能匹配 /home/user/.claude）
		absDir = filepath.Clean(absDir) + string(filepath.Separator)
		cleanPath := filepath.Clean(absPath) + string(filepath.Separator)
		if strings.HasPrefix(cleanPath, absDir) {
			return nil
		}
	}

	return fmt.Errorf("access denied: path outside allowed directory")
}

// RefreshProjectRoots 刷新项目根目录缓存
// 从会话文件中提取所有项目根目录，用于验证路径访问权限
func (e *Engine) RefreshProjectRoots() error {
	projectsDir := filepath.Join(e.homeDir, ".claude", "projects")

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		// 如果目录不存在，清空缓存
		e.mu.Lock()
		e.projectRoots = nil
		e.mu.Unlock()
		return nil
	}

	var roots []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectDir := filepath.Join(projectsDir, entry.Name())
		actualRoot := e.findProjectRootFromSessions(projectDir)
		if actualRoot != "" {
			roots = append(roots, actualRoot)
		}
	}

	e.mu.Lock()
	e.projectRoots = roots
	e.mu.Unlock()

	return nil
}

// CreateDocument 创建新文档
func (e *Engine) CreateDocument(docType DocType, title string, content string, project string, sessionId string) (string, error) {
	var path string

	// 如果 title 为空，生成默认标题
	if title == "" {
		title = "new-" + strings.TrimSuffix(GenerateRandomName(), ".md")
	}

	switch docType {
	case DocTypePlans:
		// plans/ 目录下使用随机文件名
		filename := GenerateRandomName() + ".md"
		path = filepath.Join(e.homeDir, ".claude", "plans", filename)
	case DocTypeMemory:
		// memory/ 需要指定项目
		if project == "" {
			// 尝试获取第一个可用的项目
			projectsDir := filepath.Join(e.homeDir, ".claude", "projects")
			entries, err := os.ReadDir(projectsDir)
			if err != nil || len(entries) == 0 {
				return "", fmt.Errorf("no projects found, please specify a project")
			}
			// 使用第一个项目
			for _, entry := range entries {
				if entry.IsDir() {
					project = entry.Name()
					break
				}
			}
		}
		// 确保 memory 目录存在
		memoryDir := filepath.Join(e.homeDir, ".claude", "projects", project, "memory")
		if err := os.MkdirAll(memoryDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create memory directory: %w", err)
		}
		// 使用标题生成文件名（kebab-case）
		filename := strings.ToLower(strings.ReplaceAll(title, " ", "-")) + ".md"
		path = filepath.Join(memoryDir, filename)
	case DocTypeClaudeMD:
		// CLAUDE.md 创建
		var err error
		path, err = e.createClaudeMD(title, content, project)
		if err != nil {
			return "", err
		}
		// 跳过下面的 SaveDocument，因为 createClaudeMD 已经写入了文件
		return path, nil
	default:
		path = filepath.Join(e.homeDir, ".claude", "plans", title+".md")
	}

	// 如果没有提供内容，使用模板
	if content == "" {
		content = GenerateTemplate(docType, title, sessionId)
	}

	// 保存文件
	if err := e.SaveDocument(path, content); err != nil {
		return "", err
	}

	return path, nil
}

// SearchDocuments 搜索文档
func (e *Engine) SearchDocuments(query string, filters SearchFilters) ([]KnowledgeDoc, error) {
	docs, err := e.GetAllDocuments("", "")
	if err != nil {
		return nil, err
	}

	var results []KnowledgeDoc
	query = strings.ToLower(query)

	for _, doc := range docs {
		// 应用类型筛选
		if len(filters.Types) > 0 {
			found := false
			for _, t := range filters.Types {
				if doc.Type == t {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 应用项目筛选
		if len(filters.Projects) > 0 {
			found := false
			for _, p := range filters.Projects {
				if doc.Project == p {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}

		// 关键词搜索
		if query != "" {
			if strings.Contains(strings.ToLower(doc.Name), query) ||
				strings.Contains(strings.ToLower(doc.Content), query) {
				results = append(results, doc)
			}
		} else if len(filters.Types) > 0 || len(filters.Projects) > 0 {
			// 没有查询但有筛选条件，返回筛选后的结果
			results = append(results, doc)
		}
		// 如果没有查询也没有筛选条件，不返回任何结果（避免返回所有文档）
	}

	return results, nil
}

// readDocument 读取单个文档
func (e *Engine) readDocument(path string) (*KnowledgeDoc, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// 判断文档类型
	docType := DocTypePlans
	project := ""

	// 检查路径结构判断类型（更精确的判断）
	// memory 文件路径格式: ~/.claude/projects/<project>/memory/*.md
	// plans 文件路径格式: ~/.claude/plans/*.md
	parts := strings.Split(path, string(os.PathSeparator))

	// 检查是否在 projects 目录下且包含 memory 目录
	inProjectsDir := false
	inMemoryDir := false
	for i, part := range parts {
		if part == "projects" {
			inProjectsDir = true
			// 下一个目录是项目名
			if i+1 < len(parts) {
				project = parts[i+1]
			}
		}
		if inProjectsDir && part == "memory" {
			inMemoryDir = true
		}
	}

	if inProjectsDir && inMemoryDir {
		docType = DocTypeMemory
	}

	// 检测 CLAUDE.md 文件（通过文件名匹配）
	fileName := filepath.Base(path)
	if strings.EqualFold(fileName, "CLAUDE.md") {
		docType = DocTypeClaudeMD
		// 如果项目名尚未确定，从 ~/.claude/projects/ 中查找匹配的项目
		if project == "" {
			project = e.findProjectForClaudeMD(path)
		}
	}

	// 解析 YAML frontmatter
	frontmatter, body := ParseFrontmatter(string(content))

	// 提取名称
	name := frontmatter["name"]
	if name == "" {
		// 从内容提取标题
		name = ExtractTitle(body)
	}
	if name == "" {
		// 使用文件名
		name = filepath.Base(path)
		name = strings.TrimSuffix(name, ".md")
	}

	return &KnowledgeDoc{
		Path:        path,
		Name:        name,
		Type:        docType,
		Project:     project,
		Content:     string(content),
		Frontmatter: frontmatter,
		CreatedAt:   info.ModTime(),
		UpdatedAt:   info.ModTime(),
		Size:        info.Size(),
	}, nil
}

// findProjectForClaudeMD 从 ~/.claude/projects/ 目录中查找与 CLAUDE.md 路径匹配的项目
// 通过读取会话文件中的 cwd 字段，找到实际项目根目录包含 CLAUDE.md 所在目录的项目
func (e *Engine) findProjectForClaudeMD(claudeMDPath string) string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	projectsDir := filepath.Join(homeDir, ".claude", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return ""
	}

	// CLAUDE.md 所在的目录
	claudeMDDir := filepath.Dir(claudeMDPath)
	claudeMDDir = strings.ToLower(filepath.Clean(claudeMDDir))

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectDir := filepath.Join(projectsDir, entry.Name())

		// 从 JSONL 会话文件中提取 cwd
		jsonlFiles, _ := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
		for _, jsonlPath := range jsonlFiles {
			cwd := e.extractCWDFromJSONL(jsonlPath)
			if cwd == "" {
				continue
			}

			// 检查 cwd 是否匹配 CLAUDE.md 所在目录
			cwdLower := strings.ToLower(filepath.Clean(cwd))
			if cwdLower == claudeMDDir || strings.HasPrefix(claudeMDDir, cwdLower) || strings.HasPrefix(cwdLower, claudeMDDir) {
				return entry.Name()
			}
		}
	}

	return ""
}
