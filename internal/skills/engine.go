package skills

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Engine Skills 管理引擎
type Engine struct {
	homeDir      string
	mu           sync.RWMutex
	projectRoots []string // 缓存的项目根目录列表
}

// NewEngine 创建新的 Skills 引擎
func NewEngine() (*Engine, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("获取用户主目录失败: %w", err)
	}
	return &Engine{
		homeDir: homeDir,
	}, nil
}

// ListSkills 获取 Skills 列表
func (e *Engine) ListSkills(scope string, project string) ([]SkillInfo, error) {
	var skills []SkillInfo

	// 根据 scope 筛选
	scanUser := scope == "" || scope == "all" || scope == string(ScopeUser)
	scanProject := scope == "" || scope == "all" || scope == string(ScopeProject)
	scanPlugin := scope == "" || scope == "all" || scope == string(ScopePlugin)

	// 扫描用户级 Skills: ~/.claude/skills/
	if scanUser {
		userSkills, err := e.scanUserSkills()
		if err == nil {
			skills = append(skills, userSkills...)
		}
	}

	// 扫描项目级 Skills: .claude/skills/
	if scanProject {
		projectSkills, err := e.scanProjectSkills(project)
		if err == nil {
			skills = append(skills, projectSkills...)
		}
	}

	// 扫描插件 Skills: plugins/*/skills/
	if scanPlugin {
		pluginSkills, err := e.scanPluginSkills()
		if err == nil {
			skills = append(skills, pluginSkills...)
		}
	}

	// 按修改时间倒序排列
	sort.Slice(skills, func(i, j int) bool {
		return skills[i].UpdatedAt.After(skills[j].UpdatedAt)
	})

	return skills, nil
}

// GetSkill 获取单个 Skill
func (e *Engine) GetSkill(path string) (*SkillInfo, error) {
	return e.readSkillFile(path)
}

// SaveSkill 保存 Skill
func (e *Engine) SaveSkill(scope SkillScope, name string, content string, project string) (string, error) {
	// 确定保存路径
	var dir string
	switch scope {
	case ScopeUser:
		dir = filepath.Join(e.homeDir, ".claude", "skills")
	case ScopeProject:
		if project == "" {
			return "", fmt.Errorf("project scope 需要指定项目")
		}
		// 查找项目实际根目录
		actualRoot := e.findProjectRoot(project)
		if actualRoot == "" {
			return "", fmt.Errorf("未找到项目: %s", project)
		}
		dir = filepath.Join(actualRoot, ".claude", "skills")
	case ScopePlugin:
		return "", fmt.Errorf("不支持直接保存插件 Skills")
	default:
		return "", fmt.Errorf("未知的 scope: %s", scope)
	}

	// 确保目录存在
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("创建目录失败: %w", err)
	}

	// 生成文件名（kebab-case）
	filename := strings.ToLower(strings.ReplaceAll(name, " ", "-")) + ".md"
	path := filepath.Join(dir, filename)

	// 写入文件
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", fmt.Errorf("保存 Skill 失败: %w", err)
	}

	return path, nil
}

// DeleteSkill 删除 Skill
func (e *Engine) DeleteSkill(path string) error {
	// 验证路径安全性
	if err := e.validatePath(path); err != nil {
		return err
	}

	return os.Remove(path)
}

// ValidateSkill 验证 Skill 内容
func (e *Engine) ValidateSkill(content string) []ValidationError {
	var errors []ValidationError

	// 解析 frontmatter
	frontmatter, _ := ParseSkillFrontmatter(content)

	// description 是必填字段
	if frontmatter["description"] == "" {
		errors = append(errors, ValidationError{
			Field:   "description",
			Message: "description 字段是必填的（激活触发词）",
		})
	}

	// name 字段
	if frontmatter["name"] == "" {
		errors = append(errors, ValidationError{
			Field:   "name",
			Message: "建议填写 name 字段以提高可读性",
		})
	}

	return errors
}

// scanUserSkills 扫描用户级 Skills
func (e *Engine) scanUserSkills() ([]SkillInfo, error) {
	skillsDir := filepath.Join(e.homeDir, ".claude", "skills")

	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		return nil, nil
	}

	var skills []SkillInfo

	// 扫描方式1: 直接的 SKILL.md 文件
	directMd := filepath.Join(skillsDir, "SKILL.md")
	if info, err := os.Stat(directMd); err == nil && !info.IsDir() {
		skill, err := e.readSkillFile(directMd)
		if err == nil {
			skill.Scope = ScopeUser
			skills = append(skills, *skill)
		}
	}

	// 扫描方式2: 子目录中的 SKILL.md (caveman/SKILL.md)
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return skills, nil
	}

	seen := make(map[string]bool)
	seen[directMd] = true

	for _, entry := range entries {
		// 使用 os.Stat 而非 entry.IsDir() 来正确处理 Windows 上的符号链接
		entryPath := filepath.Join(skillsDir, entry.Name())
		entryInfo, err := os.Stat(entryPath)
		if err != nil {
			continue
		}

		if !entryInfo.IsDir() {
			continue
		}

		// 跳过隐藏目录
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		skillMd := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		if seen[skillMd] {
			continue
		}

		if _, err := os.Stat(skillMd); os.IsNotExist(err) {
			continue
		}

		skill, err := e.readSkillFile(skillMd)
		if err != nil {
			continue
		}
		skill.Scope = ScopeUser
		skills = append(skills, *skill)
		seen[skillMd] = true
	}

	return skills, nil
}

// scanProjectSkills 扫描项目级 Skills
func (e *Engine) scanProjectSkills(project string) ([]SkillInfo, error) {
	var skills []SkillInfo

	// 如果指定了项目，只扫描该项目
	if project != "" {
		actualRoot := e.findProjectRoot(project)
		if actualRoot == "" {
			return nil, nil
		}
		projectSkills, err := e.scanSingleProjectSkills(actualRoot, project)
		if err == nil {
			skills = append(skills, projectSkills...)
		}
		return skills, nil
	}

	// 扫描所有项目
	projectsDir := filepath.Join(e.homeDir, ".claude", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectDir := filepath.Join(projectsDir, entry.Name())
		actualRoot := e.findProjectRootFromSessions(projectDir)
		if actualRoot == "" {
			continue
		}

		projectSkills, err := e.scanSingleProjectSkills(actualRoot, entry.Name())
		if err == nil {
			skills = append(skills, projectSkills...)
		}
	}

	return skills, nil
}

// scanSingleProjectSkills 扫描单个项目的 Skills
func (e *Engine) scanSingleProjectSkills(projectRoot string, projectDirName string) ([]SkillInfo, error) {
	skillsDir := filepath.Join(projectRoot, ".claude", "skills")

	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		return nil, nil
	}

	var skills []SkillInfo

	// 扫描方式1: 直接的 SKILL.md 文件
	directMd := filepath.Join(skillsDir, "SKILL.md")
	if info, err := os.Stat(directMd); err == nil && !info.IsDir() {
		skill, err := e.readSkillFile(directMd)
		if err == nil {
			skill.Scope = ScopeProject
			skill.Project = projectDirName
			skills = append(skills, *skill)
		}
	}

	// 扫描方式2: 子目录中的 SKILL.md
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		return skills, nil
	}

	seen := make(map[string]bool)
	seen[directMd] = true

	for _, entry := range entries {
		// 使用 os.Stat 而非 entry.IsDir() 来正确处理 Windows 上的符号链接
		entryPath := filepath.Join(skillsDir, entry.Name())
		entryInfo, err := os.Stat(entryPath)
		if err != nil {
			continue
		}

		if !entryInfo.IsDir() {
			continue
		}

		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		skillMd := filepath.Join(skillsDir, entry.Name(), "SKILL.md")
		if seen[skillMd] {
			continue
		}

		if _, err := os.Stat(skillMd); os.IsNotExist(err) {
			continue
		}

		skill, err := e.readSkillFile(skillMd)
		if err != nil {
			continue
		}
		skill.Scope = ScopeProject
		skill.Project = projectDirName
		skills = append(skills, *skill)
		seen[skillMd] = true
	}

	return skills, nil
}

// scanPluginSkills 扫描插件 Skills
func (e *Engine) scanPluginSkills() ([]SkillInfo, error) {
	pluginsDir := filepath.Join(e.homeDir, ".claude", "plugins")

	if _, err := os.Stat(pluginsDir); os.IsNotExist(err) {
		return nil, nil
	}

	var skills []SkillInfo

	// 遍历插件目录
	entries, err := os.ReadDir(pluginsDir)
	if err != nil {
		return nil, nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillsDir := filepath.Join(pluginsDir, entry.Name(), "skills")
		if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
			continue
		}

		// 扫描插件内的 skills - 子目录结构
		skillEntries, err := os.ReadDir(skillsDir)
		if err != nil {
			continue
		}

		seen := make(map[string]bool)

		for _, skillEntry := range skillEntries {
			// 使用 os.Stat 而非 entry.IsDir() 来正确处理 Windows 上的符号链接
			entryPath := filepath.Join(skillsDir, skillEntry.Name())
			entryInfo, err := os.Stat(entryPath)
			if err != nil {
				continue
			}

			if !entryInfo.IsDir() {
				continue
			}

			if strings.HasPrefix(skillEntry.Name(), ".") {
				continue
			}

			skillMd := filepath.Join(skillsDir, skillEntry.Name(), "SKILL.md")
			if seen[skillMd] {
				continue
			}

			if _, err := os.Stat(skillMd); os.IsNotExist(err) {
				continue
			}

			skill, err := e.readSkillFile(skillMd)
			if err != nil {
				continue
			}
			skill.Scope = ScopePlugin
			skills = append(skills, *skill)
			seen[skillMd] = true
		}
	}

	return skills, nil
}

// readSkillFile 读取单个 Skill 文件
func (e *Engine) readSkillFile(path string) (*SkillInfo, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// 解析 frontmatter
	frontmatter, body := ParseSkillFrontmatter(string(content))

	// 构建 meta
	meta := SkillMeta{
		Name:        frontmatter["name"],
		Description: frontmatter["description"],
	}

	// 解析布尔字段
	if val, ok := frontmatter["disable-model-invocation"]; ok {
		meta.DisableModelInvocation = strings.ToLower(val) == "true"
	}
	if val, ok := frontmatter["user-invocable"]; ok {
		meta.UserInvocable = strings.ToLower(val) == "true"
	}

	// 解析数组字段（简单逗号分隔）
	if val, ok := frontmatter["paths"]; ok && val != "" {
		meta.Paths = strings.Split(val, ",")
		for i := range meta.Paths {
			meta.Paths[i] = strings.TrimSpace(meta.Paths[i])
		}
	}
	if val, ok := frontmatter["allowed-tools"]; ok && val != "" {
		meta.AllowedTools = strings.Split(val, ",")
		for i := range meta.AllowedTools {
			meta.AllowedTools[i] = strings.TrimSpace(meta.AllowedTools[i])
		}
	}
	if val, ok := frontmatter["disallowed-tools"]; ok && val != "" {
		meta.DisallowedTools = strings.Split(val, ",")
		for i := range meta.DisallowedTools {
			meta.DisallowedTools[i] = strings.TrimSpace(meta.DisallowedTools[i])
		}
	}

	// 如果 name 为空，从文件名提取
	if meta.Name == "" {
		baseName := filepath.Base(path)
		meta.Name = strings.TrimSuffix(baseName, ".md")
	}

	return &SkillInfo{
		Path:      path,
		Meta:      meta,
		Content:   string(content),
		Body:      body,
		Size:      info.Size(),
		CreatedAt: info.ModTime(),
		UpdatedAt: info.ModTime(),
	}, nil
}

// GetClaudeMDProjects 获取所有有 CLAUDE.md 的项目列表（复用 knowledge 引擎的逻辑）
func (e *Engine) GetClaudeMDProjects() ([]string, error) {
	projectsDir := filepath.Join(e.homeDir, ".claude", "projects")

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, err
	}

	var projects []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// 检查项目是否有 .claude/skills/ 目录
		projectDir := filepath.Join(projectsDir, entry.Name())
		actualRoot := e.findProjectRootFromSessions(projectDir)
		if actualRoot == "" {
			continue
		}

		skillsDir := filepath.Join(actualRoot, ".claude", "skills")
		if _, err := os.Stat(skillsDir); err == nil {
			projects = append(projects, entry.Name())
		}
	}

	return projects, nil
}

// ParseSkillFrontmatter 解析 Skill 的 YAML frontmatter
func ParseSkillFrontmatter(content string) (map[string]string, string) {
	frontmatter := make(map[string]string)
	body := content

	// 检查是否以 --- 开头
	if !strings.HasPrefix(content, "---") {
		return frontmatter, body
	}

	// 查找结束标记
	endIndex := strings.Index(content[3:], "---")
	if endIndex == -1 {
		return frontmatter, body
	}

	// 提取 frontmatter 部分
	fmContent := content[3 : endIndex+3]
	body = content[endIndex+6:]

	// 简单解析 YAML（支持 key: value 格式）
	lines := strings.Split(fmContent, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			value := strings.TrimSpace(parts[1])
			frontmatter[key] = value
		}
	}

	return frontmatter, strings.TrimSpace(body)
}

// findProjectRoot 从项目目录名查找实际项目根目录
func (e *Engine) findProjectRoot(projectDirName string) string {
	projectsDir := filepath.Join(e.homeDir, ".claude", "projects")
	projectDir := filepath.Join(projectsDir, projectDirName)

	return e.findProjectRootFromSessions(projectDir)
}

// findProjectRootFromSessions 从会话文件中提取项目根目录
func (e *Engine) findProjectRootFromSessions(projectDir string) string {
	// 查找 JSONL 文件
	jsonlFiles, _ := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
	if len(jsonlFiles) == 0 {
		return ""
	}

	// 读取第一个 JSONL 文件，使用 JSON 解析提取 cwd 字段
	file, err := os.Open(jsonlFiles[0])
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || !strings.Contains(line, `"cwd"`) {
			continue
		}

		var event struct {
			CWD string `json:"cwd"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.CWD != "" {
			return event.CWD
		}
	}

	return ""
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
		// 使用 filepath.Clean 标准化路径，确保分隔符一致并追加分隔符避免前缀匹配绕过
		absDir = filepath.Clean(absDir) + string(filepath.Separator)
		cleanPath := filepath.Clean(absPath) + string(filepath.Separator)
		if strings.HasPrefix(cleanPath, absDir) {
			return nil
		}
	}

	return fmt.Errorf("access denied: path outside allowed directory")
}

// RefreshProjectRoots 刷新项目根目录缓存
func (e *Engine) RefreshProjectRoots() error {
	projectsDir := filepath.Join(e.homeDir, ".claude", "projects")

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
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

// GetSkillUsageStats 获取 Skill 使用统计
// 从 JSONL 中解析 type: "skill_start" 事件
func (e *Engine) GetSkillUsageStats(projectDir string) ([]SkillUsageStat, error) {
	// TODO: 实现从 JSONL 中提取 Skill 使用统计
	// 目前返回空列表，后续可以从会话数据中提取
	return []SkillUsageStat{}, nil
}

// GenerateSkillTemplate 生成 Skill 模板
func GenerateSkillTemplate(name string, description string) string {
	if name == "" {
		name = "my-skill"
	}
	if description == "" {
		description = "Description of what this skill does"
	}

	return `---
name: ` + name + `
description: ` + description + `
user-invocable: true
---

# ` + name + `

## Instructions

[在此编写 Skill 的指令内容]

## Examples

[在此提供使用示例]
`
}
