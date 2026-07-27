// Package skills provides unified management capabilities for Agent Skills,
// supporting both user-level and project-level skill definitions.
package skills

import (
	"time"
)

// SkillScope Skill 作用域
type SkillScope string

const (
	ScopeUser    SkillScope = "user"    // 用户级：~/.claude/skills/
	ScopeProject SkillScope = "project" // 项目级：.claude/skills/
	ScopePlugin  SkillScope = "plugin"  // 插件内嵌：plugins/*/skills/
)

// SkillMeta Skill 的 frontmatter 元数据
type SkillMeta struct {
	Name                   string   `yaml:"name" json:"name"`
	Description            string   `yaml:"description" json:"description"`                       // 激活触发词（关键字段）
	DisableModelInvocation bool     `yaml:"disable-model-invocation,omitempty" json:"disableModelInvocation"`
	UserInvocable          bool     `yaml:"user-invocable,omitempty" json:"userInvocable"`
	Paths                  []string `yaml:"paths,omitempty" json:"paths"`                          // 文件 glob 自动激活
	AllowedTools           []string `yaml:"allowed-tools,omitempty" json:"allowedTools"`
	DisallowedTools        []string `yaml:"disallowed-tools,omitempty" json:"disallowedTools"`
}

// SkillInfo Skill 完整信息
type SkillInfo struct {
	Path      string    `json:"path"`      // 文件路径
	Name      string    `json:"name"`      // 显示名称
	Scope     SkillScope `json:"scope"`     // user, project, plugin
	Project   string    `json:"project"`   // 所属项目（project scope 时有值）
	Meta      SkillMeta `json:"meta"`      // frontmatter 元数据
	Content   string    `json:"content"`   // 完整文件内容（含 frontmatter）
	Body      string    `json:"body"`      // Markdown 内容（不含 frontmatter）
	UpdatedAt time.Time `json:"updatedAt"` // 最后更新时间
	CreatedAt time.Time `json:"createdAt"` // 创建时间
	Size      int64     `json:"size"`      // 文件大小
}

// SkillUsageStat Skill 使用统计
type SkillUsageStat struct {
	Name       string `json:"name"`       // Skill 名称
	InvokeCount int   `json:"invokeCount"` // 调用次数
	LastUsed   string `json:"lastUsed"`   // 最后使用时间
}

// ValidationError Skill 校验错误
type ValidationError struct {
	Field   string `json:"field"`   // 出错字段
	Message string `json:"message"` // 错误描述
}
