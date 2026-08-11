package knowledge

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ClaudeMDLocation CLAUDE.md 文件位置
type ClaudeMDLocation struct {
	Path     string `json:"path"`     // 文件绝对路径
	Project  string `json:"project"`  // 项目名（目录编码名）
	IsGlobal bool   `json:"isGlobal"` // 是否全局配置
}

// scanClaudeMD 扫描所有 CLAUDE.md 文件
func (e *Engine) scanClaudeMD() ([]KnowledgeDoc, error) {
	var docs []KnowledgeDoc

	// 1. 扫描全局 CLAUDE.md
	globalDoc, err := e.readGlobalClaudeMD()
	if err == nil && globalDoc != nil {
		docs = append(docs, *globalDoc)
	}

	// 2. 扫描所有项目的 CLAUDE.md
	projectDocs, err := e.scanProjectClaudeMD()
	if err == nil {
		docs = append(docs, projectDocs...)
	}

	return docs, nil
}

// readGlobalClaudeMD 读取全局 CLAUDE.md (~/.claude/CLAUDE.md)
func (e *Engine) readGlobalClaudeMD() (*KnowledgeDoc, error) {
	path := filepath.Join(e.homeDir, ".claude", "CLAUDE.md")

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil // 文件不存在不是错误
	}
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	name := ExtractTitle(string(content))
	if name == "" {
		name = "Global CLAUDE.md"
	}

	return &KnowledgeDoc{
		Path:      path,
		Name:      name,
		Type:      DocTypeClaudeMD,
		Project:   "global",
		Content:   string(content),
		CreatedAt: info.ModTime(),
		UpdatedAt: info.ModTime(),
		Size:      info.Size(),
	}, nil
}

// scanProjectClaudeMD 扫描所有项目的 CLAUDE.md
func (e *Engine) scanProjectClaudeMD() ([]KnowledgeDoc, error) {
	// 刷新项目根目录缓存，确保后续保存操作可以访问这些目录
	if err := e.RefreshProjectRoots(); err != nil {
		// 刷新失败不影响扫描，继续执行
		fmt.Printf("警告：刷新项目根目录缓存失败: %v\n", err)
	}

	projectsDir := filepath.Join(e.homeDir, ".claude", "projects")

	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil, err
	}

	var docs []KnowledgeDoc

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectDir := filepath.Join(projectsDir, entry.Name())

		// 从 JSONL 会话文件中提取实际项目路径（cwd）
		actualRoot := e.findProjectRootFromSessions(projectDir)

		// 检查多个可能的 CLAUDE.md 位置
		locations := e.findClaudeMDLocations(entry.Name(), actualRoot)

		for _, loc := range locations {
			doc, err := e.readClaudeMDFile(loc.Path, loc.Project, loc.IsGlobal)
			if err != nil {
				continue
			}
			docs = append(docs, *doc)
		}
	}

	return docs, nil
}

// findProjectRootFromSessions 从会话 JSONL 文件中提取项目实际根目录
func (e *Engine) findProjectRootFromSessions(projectDir string) string {
	jsonlFiles, _ := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
	if len(jsonlFiles) == 0 {
		return ""
	}

	// 读取第一个会话文件，提取 cwd
	for _, jsonlPath := range jsonlFiles {
		cwd := e.extractCWDFromJSONL(jsonlPath)
		if cwd != "" {
			return cwd
		}
	}

	return ""
}

// extractCWDFromJSONL 从 JSONL 文件中提取 cwd 字段
func (e *Engine) extractCWDFromJSONL(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	// 使用 64KB 初始缓冲区，最大 4MB，避免 JSONL 行过长导致 token too long 错误
	const maxScanTokenSize = 4 * 1024 * 1024
	scanner.Buffer(make([]byte, 0, 64*1024), maxScanTokenSize)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
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

// findClaudeMDLocations 查找项目中所有可能的 CLAUDE.md 位置
func (e *Engine) findClaudeMDLocations(projectName string, actualRoot string) []ClaudeMDLocation {
	var locations []ClaudeMDLocation

	// 位置 1: 实际项目根目录/CLAUDE.md
	if actualRoot != "" {
		path := filepath.Join(actualRoot, "CLAUDE.md")
		if _, err := os.Stat(path); err == nil {
			locations = append(locations, ClaudeMDLocation{
				Path:     path,
				Project:  projectName,
				IsGlobal: false,
			})
		}

		// 位置 2: 实际项目根目录/.claude/CLAUDE.md
		path2 := filepath.Join(actualRoot, ".claude", "CLAUDE.md")
		if _, err := os.Stat(path2); err == nil {
			locations = append(locations, ClaudeMDLocation{
				Path:     path2,
				Project:  projectName,
				IsGlobal: false,
			})
		}
	}

	// 位置 3: ~/.claude/projects/<project>/.claude/CLAUDE.md（备选）
	path3 := filepath.Join(e.homeDir, ".claude", "projects", projectName, ".claude", "CLAUDE.md")
	if _, err := os.Stat(path3); err == nil {
		// 避免重复（如果 actualRoot 就是这个目录）
		duplicate := false
		for _, loc := range locations {
			abs1, _ := filepath.Abs(loc.Path)
			abs2, _ := filepath.Abs(path3)
			if abs1 == abs2 {
				duplicate = true
				break
			}
		}
		if !duplicate {
			locations = append(locations, ClaudeMDLocation{
				Path:     path3,
				Project:  projectName,
				IsGlobal: false,
			})
		}
	}

	return locations
}

// readClaudeMDFile 读取单个 CLAUDE.md 文件
func (e *Engine) readClaudeMDFile(path string, project string, isGlobal bool) (*KnowledgeDoc, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	name := ExtractTitle(string(content))
	if name == "" {
		name = filepath.Base(path)
		if strings.HasSuffix(name, ".md") && len(name) > 3 {
			name = name[:len(name)-3]
		}
	}

	// 构建显示项目名
	displayProject := project
	if isGlobal {
		displayProject = "global"
	}

	return &KnowledgeDoc{
		Path:      path,
		Name:      name,
		Type:      DocTypeClaudeMD,
		Project:   displayProject,
		Content:   string(content),
		CreatedAt: info.ModTime(),
		UpdatedAt: info.ModTime(),
		Size:      info.Size(),
	}, nil
}

// createClaudeMD 创建 CLAUDE.md 文件
func (e *Engine) createClaudeMD(title string, content string, project string) (string, error) {
	var path string

	if project == "" || project == "global" {
		// 全局 CLAUDE.md
		path = filepath.Join(e.homeDir, ".claude", "CLAUDE.md")
	} else {
		// 项目级 CLAUDE.md
		// 优先使用实际项目根目录
		projectDir := filepath.Join(e.homeDir, ".claude", "projects", project)
		actualRoot := e.findProjectRootFromSessions(projectDir)

		if actualRoot != "" {
			path = filepath.Join(actualRoot, "CLAUDE.md")
		} else {
			// 回退到 ~/.claude/projects/<project>/CLAUDE.md
			path = filepath.Join(projectDir, "CLAUDE.md")
		}
	}

	// 如果没有提供内容，使用模板
	if content == "" {
		content = GenerateClaudeMDTemplate(title)
	}

	// 确保目录存在
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	// 写入文件
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return "", err
	}

	return path, nil
}

// GetClaudeMDProjects 获取所有有 CLAUDE.md 的项目列表
func (e *Engine) GetClaudeMDProjects() ([]ClaudeMDProject, error) {
	var projects []ClaudeMDProject

	// 全局
	globalPath := filepath.Join(e.homeDir, ".claude", "CLAUDE.md")
	if _, err := os.Stat(globalPath); err == nil {
		projects = append(projects, ClaudeMDProject{
			Name:     "global",
			HasCLAUDE: true,
			Path:     globalPath,
		})
	}

	// 项目级
	projectsDir := filepath.Join(e.homeDir, ".claude", "projects")
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return projects, nil
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projectDir := filepath.Join(projectsDir, entry.Name())
		actualRoot := e.findProjectRootFromSessions(projectDir)

		hasCLAUDE := false
		var claudePath string

		// 检查实际项目根目录
		if actualRoot != "" {
			path := filepath.Join(actualRoot, "CLAUDE.md")
			if _, err := os.Stat(path); err == nil {
				hasCLAUDE = true
				claudePath = path
			}
		}

		// 检查 .claude/CLAUDE.md
		if !hasCLAUDE {
			path2 := filepath.Join(projectDir, ".claude", "CLAUDE.md")
			if _, err := os.Stat(path2); err == nil {
				hasCLAUDE = true
				claudePath = path2
			}
		}

		projects = append(projects, ClaudeMDProject{
			Name:      entry.Name(),
			HasCLAUDE: hasCLAUDE,
			Path:      claudePath,
			RootDir:   actualRoot,
		})
	}

	return projects, nil
}

// ClaudeMDProject CLAUDE.md 项目信息
type ClaudeMDProject struct {
	Name      string `json:"name"`
	HasCLAUDE bool   `json:"hasClaudeMD"`
	Path      string `json:"path"`      // CLAUDE.md 文件路径
	RootDir   string `json:"rootDir"`   // 项目实际根目录
}

// time.Now() 用法说明：在测试中需要 mock，这里使用文件修改时间
var _ = time.Now
