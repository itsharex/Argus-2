package diff

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"argus-desktop/internal/session"
)

// Engine Git Diff 引擎
type Engine struct {
	WorkDir string // git 仓库根目录
}

// NewEngine 创建新的 Diff 引擎
func NewEngine(workDir string) *Engine {
	return &Engine{WorkDir: workDir}
}

// DiffResult 单个文件的 diff 结果
type DiffResult struct {
	FilePath   string // 文件路径
	ChangeType session.ChangeType
	AddedLines int
	RemovedLines int
	Patch      string // unified diff 内容
	Actions    []session.Action // 关联的 Agent 操作
}

// GetDiff 获取指定范围的 git diff
func (e *Engine) GetDiff(from, to string) ([]DiffResult, error) {
	args := []string{"diff"}
	if from != "" {
		args = append(args, from)
	}
	if to != "" {
		args = append(args, to)
	}
	args = append(args, "--numstat")

	cmd := newGitCommand(args...)
	cmd.Dir = e.WorkDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行 git diff 失败: %w", err)
	}

	return e.parseNumstat(string(output))
}

// GetDiffBetweenRefs 获取两个引用之间的 diff
func (e *Engine) GetDiffBetweenRefs(fromRef, toRef string) ([]DiffResult, error) {
	args := []string{"diff", fromRef, toRef, "--numstat"}

	cmd := newGitCommand(args...)
	cmd.Dir = e.WorkDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行 git diff 失败: %w", err)
	}

	return e.parseNumstat(string(output))
}

// GetUncommittedDiff 获取未提交的改动（包括已跟踪和未跟踪的文件）
func (e *Engine) GetUncommittedDiff() ([]DiffResult, error) {
	var results []DiffResult

	// 1. 获取已跟踪文件的改动 (git diff)
	cmd := newGitCommand("diff", "--numstat")
	cmd.Dir = e.WorkDir
	output, err := cmd.Output()
	if err == nil {
		diffs, _ := e.parseNumstat(string(output))
		results = append(results, diffs...)
	}

	// 2. 获取已暂存文件的改动 (git diff --cached)
	cmd = newGitCommand("diff", "--cached", "--numstat")
	cmd.Dir = e.WorkDir
	output, err = cmd.Output()
	if err == nil {
		diffs, _ := e.parseNumstat(string(output))
		results = append(results, diffs...)
	}

	// 3. 获取未跟踪的新文件 (git ls-files --others --exclude-standard)
	cmd = newGitCommand("ls-files", "--others", "--exclude-standard")
	cmd.Dir = e.WorkDir
	output, err = cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// 检查是否已经在结果中
			exists := false
			for _, r := range results {
				if r.FilePath == line {
					exists = true
					break
				}
			}
			if !exists {
				results = append(results, DiffResult{
					FilePath:   line,
					ChangeType: session.ChangeCreated,
					AddedLines: 0,
					RemovedLines: 0,
					Patch:      "",
				})
			}
		}
	}

	return results, nil
}

// GetStagedDiff 获取已暂存的改动
func (e *Engine) GetStagedDiff() ([]DiffResult, error) {
	cmd := newGitCommand("diff", "--cached", "--numstat")
	cmd.Dir = e.WorkDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行 git diff 失败: %w", err)
	}

	return e.parseNumstat(string(output))
}

// GetFilePatch 获取单个文件的完整 patch
func (e *Engine) GetFilePatch(filePath string) (string, error) {
	cmd := newGitCommand("diff", "--", filePath)
	cmd.Dir = e.WorkDir
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("获取文件 patch 失败: %w", err)
	}

	return string(output), nil
}

// GetDiffWithActions 获取 diff 并关联到 Agent 的 actions
func (e *Engine) GetDiffWithActions(actions []session.Action) ([]DiffResult, error) {
	// 1. 获取所有文件的 diff
	diffs, err := e.GetUncommittedDiff()
	if err != nil {
		return nil, err
	}

	// 2. 获取已暂存的 diff
	stagedDiffs, err := e.GetStagedDiff()
	if err == nil {
		diffs = append(diffs, stagedDiffs...)
	}

	// 3. 对于每个 diff，获取完整 patch
	for i := range diffs {
		patch, err := e.GetFilePatch(diffs[i].FilePath)
		if err != nil {
			continue
		}
		diffs[i].Patch = patch
	}

	// 4. 将 actions 关联到对应的文件 diff
	if len(actions) > 0 {
		// 创建文件路径到 actions 的映射
		actionsByFile := make(map[string][]session.Action)
		for _, action := range actions {
			if action.FilePath != "" {
				actionsByFile[action.FilePath] = append(actionsByFile[action.FilePath], action)
			}
		}

		// 关联 actions 到 diff 结果
		for i := range diffs {
			if fileActions, ok := actionsByFile[diffs[i].FilePath]; ok {
				diffs[i].Actions = fileActions
			}
		}
	}

	return diffs, nil
}

// GetDiffBetweenSession 获取会话前后的 diff
func (e *Engine) GetDiffBetweenSession(sess *session.Session) ([]DiffResult, error) {
	// 1. 首先获取所有 reflog，找到会话开始前的 commit
	cmd := newGitCommand("reflog", "--format=%H %ci")
	cmd.Dir = e.WorkDir
	reflogOutput, err := cmd.Output()
	if err != nil {
		// 如果无法获取 reflog，尝试获取未提交的 diff
		return e.GetUncommittedDiff()
	}

	// 解析 reflog，找到会话开始前的 commit
	refBeforeSession := findRefBeforeTime(string(reflogOutput), sess.StartedAt.Format(time.RFC3339))
	if refBeforeSession == "" {
		// 如果没有找到会话前的 ref，获取未提交的 diff
		return e.GetUncommittedDiff()
	}

	// 2. 获取当前 HEAD（在找到会话前的 ref 之后）
	cmd = newGitCommand("rev-parse", "HEAD")
	cmd.Dir = e.WorkDir
	headOutput, err := cmd.Output()
	if err != nil {
		// 如果无法获取 HEAD，获取未提交的 diff
		return e.GetUncommittedDiff()
	}
	headRef := strings.TrimSpace(string(headOutput))

	// 3. 对比两个 ref
	return e.GetDiffBetweenRefs(refBeforeSession, headRef)
}

func findRefBeforeTime(reflog string, t string) string {
	// 解析目标时间
	targetTime, err := time.Parse(time.RFC3339, t)
	if err != nil {
		// 如果时间解析失败，返回第一个 ref
		lines := strings.Split(reflog, "\n")
		if len(lines) > 0 {
			parts := strings.SplitN(lines[0], " ", 2)
			if len(parts) > 0 {
				return parts[0]
			}
		}
		return ""
	}

	lines := strings.Split(reflog, "\n")
	// reflog 是按时间倒序排列的，从最新的到最旧的
	// 我们需要找到时间在目标时间之前（更旧）的第一个 ref
	// 从前往后遍历（最新的先检查），返回最接近目标时间的 ref
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			continue
		}

		// 格式: "HASH YYYY-MM-DD HH:MM:SS +TZOFF"
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			continue
		}

		hash := parts[0]
		timeStr := parts[1]

		// 尝试解析时间
		// git reflog 时间格式: "2026-06-30 10:30:45 +0800"
		refTime, err := time.Parse("2006-01-02 15:04:05 -0700", timeStr)
		if err != nil {
			continue
		}

		// 如果这个 ref 的时间在目标时间之前（更旧），返回它
		if refTime.Before(targetTime) {
			return hash
		}
	}

	// 如果没有找到在目标时间之前的 ref，返回第一个 ref
	if len(lines) > 0 {
		parts := strings.SplitN(lines[0], " ", 2)
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return ""
}

// parseNumstat 解析 git diff --numstat 的输出
func (e *Engine) parseNumstat(output string) ([]DiffResult, error) {
	var results []DiffResult

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// numstat 格式: "added\tremoved\tfilename"
		// 删除的文件: "0\t-N\tfilename" (N 是删除的行数)
		// 新增的文件: "N\t0\tfilename" (N 是新增的行数)
		// 二进制文件: "-\t-\tfilename"
		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}

		addedStr := parts[0]
		removedStr := parts[1]
		filePath := parts[2]

		// 跳过二进制文件
		if addedStr == "-" || removedStr == "-" {
			continue
		}

		var added, removed int
		fmt.Sscanf(addedStr, "%d", &added)
		fmt.Sscanf(removedStr, "%d", &removed)

		changeType := session.ChangeModified
		if added > 0 && removed == 0 {
			changeType = session.ChangeCreated
		} else if added == 0 && removed > 0 {
			changeType = session.ChangeDeleted
		}

		results = append(results, DiffResult{
			FilePath:     filePath,
			ChangeType:   changeType,
			AddedLines:   added,
			RemovedLines: removed,
		})
	}

	return results, nil
}

// FindGitRoot 查找 git 仓库根目录
func FindGitRoot(startDir string) (string, error) {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return "", err
	}

	for {
		// 检查 .git 目录是否存在
		gitDir := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			return dir, nil
		}

		// 检查父目录
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", fmt.Errorf("未找到 git 仓库")
}

// GetStatus 获取 git 状态
func (e *Engine) GetStatus() (map[string]string, error) {
	cmd := newGitCommand("status", "--porcelain")
	cmd.Dir = e.WorkDir
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("执行 git status 失败: %w", err)
	}

	status := make(map[string]string)
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// porcelain 格式: "XY filename"
		if len(line) < 3 {
			continue
		}

		statusCode := line[:2]
		filePath := strings.TrimSpace(line[3:])

		status[filePath] = statusCode
	}

	return status, nil
}
