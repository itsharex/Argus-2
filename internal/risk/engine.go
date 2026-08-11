package risk

import (
	"regexp"
	"sort"
	"strings"

	"argus-desktop/internal/session"
)

// Engine 风险规则引擎
type Engine struct {
	rules []Rule
}

// Rule 风险规则
type Rule struct {
	Name        string
	Description string
	Level       session.RiskLevel
	Priority    int // 优先级，数字越小优先级越高
	Check       func(fc session.FileChange) bool
}

// NewEngine 创建新的风险引擎
func NewEngine() *Engine {
	e := &Engine{}
	e.loadDefaultRules()
	return e
}

func (e *Engine) loadDefaultRules() {
	e.rules = []Rule{
		// 🔴 Danger 规则 (优先级 10-19)
		{
			Name:        "delete_file",
			Description: "删除文件（需人工审查）",
			Level:       session.RiskReview,
			Priority:    10,
			Check: func(fc session.FileChange) bool {
				return fc.ChangeType == session.ChangeDeleted
			},
		},
		{
			Name:        "secret_file",
			Description: "修改了包含敏感信息的文件",
			Level:       session.RiskDanger,
			Priority:    11,
			Check: func(fc session.FileChange) bool {
				patterns := []string{"secret", "password", "api_key", "apikey", "token", "credential", ".env", "key"}
				lower := strings.ToLower(fc.Path)
				for _, p := range patterns {
					if strings.Contains(lower, p) {
						return true
					}
				}
				return false
			},
		},
		{
			Name:        "dangerous_command",
			Description: "执行了危险的 shell 命令",
			Level:       session.RiskDanger,
			Priority:    12,
			Check: func(fc session.FileChange) bool {
				for _, action := range fc.Actions {
					if action.Type != session.ActionBash {
						continue
					}
					cmd := strings.ToLower(action.Description)
					dangerousPatterns := []string{"rm -rf", "rm -r /", "chmod 777", "curl | bash", "wget | bash", "> /dev/", "mkfs", "dd if="}
					for _, p := range dangerousPatterns {
						if strings.Contains(cmd, p) {
							return true
						}
					}
				}
				return false
			},
		},
		{
			Name:        "large_change",
			Description: "改动超过 500 行",
			Level:       session.RiskDanger,
			Priority:    13,
			Check: func(fc session.FileChange) bool {
				// 计算实际的代码变更行数（以+或-开头，排除+++和---）
				lines := strings.Split(fc.Diff, "\n")
				changeCount := 0
				for _, line := range lines {
					if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
						changeCount++
					} else if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
						changeCount++
					}
				}
				return changeCount > 500
			},
		},
		{
			Name:        "config_file",
			Description: "修改了配置文件",
			Level:       session.RiskDanger,
			Priority:    14,
			Check: func(fc session.FileChange) bool {
				configPatterns := []string{".git/config", "CI", ".github/", ".gitlab-ci", "Jenkinsfile", ".circleci", "docker-compose", "Dockerfile"}
				for _, p := range configPatterns {
					if strings.Contains(fc.Path, p) {
						return true
					}
				}
				return false
			},
		},

		// 🔴 Danger 规则 — 大规模删除（优先级 9，在 delete_file 之前评估）
		{
			Name:        "large_deletion",
			Description: "删除超过 50 行代码",
			Level:       session.RiskDanger,
			Priority:    9,
			Check: func(fc session.FileChange) bool {
				if fc.ChangeType != session.ChangeDeleted {
					return false
				}
				lines := strings.Split(fc.Diff, "\n")
				removed := 0
				for _, line := range lines {
					if strings.HasPrefix(line, "-") && !strings.HasPrefix(line, "---") {
						removed++
					}
				}
				return removed > 50
			},
		},
		{
			Name:        "multiple_edits",
			Description: "同一个文件被多次编辑",
			Level:       session.RiskReview,
			Priority:    21,
			Check: func(fc session.FileChange) bool {
				editCount := 0
				for _, action := range fc.Actions {
					if action.Type == session.ActionEdit {
						editCount++
					}
				}
				return editCount > 2
			},
		},
		{
			Name:        "dependency_file",
			Description: "修改了依赖文件",
			Level:       session.RiskReview,
			Priority:    22,
			Check: func(fc session.FileChange) bool {
				depPatterns := []string{"go.mod", "go.sum", "package.json", "package-lock.json", "yarn.lock", "requirements.txt", "Cargo.toml", "Cargo.lock"}
				for _, p := range depPatterns {
					if strings.HasSuffix(fc.Path, p) || strings.Contains(fc.Path, "/"+p) {
						return true
					}
				}
				return false
			},
		},
		{
			Name:        "test_file",
			Description: "修改了测试文件",
			Level:       session.RiskReview,
			Priority:    23,
			Check: func(fc session.FileChange) bool {
				testPatterns := []string{"_test.go", ".test.", ".spec.", "test/", "tests/"}
				for _, p := range testPatterns {
					if strings.Contains(fc.Path, p) {
						return true
					}
				}
				return false
			},
		},

		// 🟢 Safe 规则 (优先级 30-39)
		{
			Name:        "new_file",
			Description: "新增文件（纯添加）",
			Level:       session.RiskSafe,
			Priority:    30,
			Check: func(fc session.FileChange) bool {
				return fc.ChangeType == session.ChangeCreated
			},
		},
		{
			Name:        "small_change",
			Description: "小改动（< 20 行）",
			Level:       session.RiskSafe,
			Priority:    31,
			Check: func(fc session.FileChange) bool {
				lines := strings.Split(fc.Diff, "\n")
				changes := 0
				for _, line := range lines {
					if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
						if !strings.HasPrefix(line, "+++") && !strings.HasPrefix(line, "---") {
							changes++
						}
					}
				}
				return changes < 20
			},
		},
		{
			Name:        "doc_change",
			Description: "只修改了文档/注释",
			Level:       session.RiskSafe,
			Priority:    32,
			Check: func(fc session.FileChange) bool {
				docPatterns := []string{".md", ".txt", ".rst", "README", "CHANGELOG", "LICENSE"}
				for _, p := range docPatterns {
					if strings.Contains(fc.Path, p) {
						return true
					}
				}
				return false
			},
		},
	}

	// 按优先级排序规则
	sort.Slice(e.rules, func(i, j int) bool {
		return e.rules[i].Priority < e.rules[j].Priority
	})
}

// Evaluate 评估文件改动的风险等级
func (e *Engine) Evaluate(fc session.FileChange) session.FileChange {
	// 按优先级检查规则：Danger > Review > Safe
	for _, rule := range e.rules {
		if rule.Check(fc) {
			fc.Risk = rule.Level
			fc.RiskReason = rule.Description
			return fc
		}
	}

	// 默认标记为 Review
	fc.Risk = session.RiskReview
	fc.RiskReason = "未匹配任何规则，建议人工审查"
	return fc
}

// EvaluateAll 批量评估文件改动的风险等级
func (e *Engine) EvaluateAll(fcs []session.FileChange) []session.FileChange {
	results := make([]session.FileChange, len(fcs))
	for i, fc := range fcs {
		results[i] = e.Evaluate(fc)
	}
	return results
}

// AddRule 添加自定义规则
func (e *Engine) AddRule(rule Rule) {
	e.rules = append(e.rules, rule)
}

// GetRules 获取所有规则
func (e *Engine) GetRules() []Rule {
	return e.rules
}

// containsSensitive 检查字符串是否包含敏感信息（辅助函数）
func containsSensitive(s string) bool {
	patterns := []string{
		"password", "secret", "api_key", "apikey", "token",
		"credential", "private_key", "access_key",
	}
	lower := strings.ToLower(s)
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// matchesRegex 检查字符串是否匹配正则表达式（辅助函数）
func matchesRegex(s string, pattern string) bool {
	matched, err := regexp.MatchString(pattern, s)
	if err != nil {
		return false
	}
	return matched
}
