// Package i18n provides internationalization support for backend strings.
package i18n

import (
	"sync"
)

// Lang 语言类型
type Lang string

const (
	LangZh Lang = "zh"
	LangEn Lang = "en"
)

var (
	currentLang = LangZh
	langMu      sync.RWMutex
)

// SetLang 设置当前语言
func SetLang(lang Lang) {
	langMu.Lock()
	defer langMu.Unlock()
	currentLang = lang
}

// GetLang 获取当前语言
func GetLang() Lang {
	langMu.RLock()
	defer langMu.RUnlock()
	return currentLang
}

// Msg 返回当前语言的消息
func Msg(zh, en string) string {
	langMu.RLock()
	defer langMu.RUnlock()
	if currentLang == LangEn {
		return en
	}
	return zh
}

// --- 会话发现 ---

func ErrGetHomeDir() string       { return Msg("获取主目录失败", "Failed to get home directory") }
func ErrGetWorkDir() string       { return Msg("获取工作目录失败", "Failed to get working directory") }
func ErrReadClaudeProjects() string { return Msg("读取 Claude 项目目录失败", "Failed to read Claude projects directory") }
func ErrSessionNotFound(id string) string { return Msg("未找到会话: "+id, "Session not found: "+id) }
func ErrProjectDirNotExist(dir string) string { return Msg("项目目录不存在: "+dir, "Project directory does not exist: "+dir) }
func ErrNoSessionsInDir(dir string) string { return Msg("目录中没有会话: "+dir, "No sessions in directory: "+dir) }
func ErrNoSessions() string       { return Msg("未找到任何会话", "No sessions found") }

// --- 会话分析 ---

func ErrOpenJSONLFailed() string  { return Msg("打开 JSONL 文件失败", "Failed to open JSONL file") }
func ErrReadSessionFailed() string { return Msg("读取会话失败", "Failed to read session") }
func ErrTraverseJSONLFailed() string { return Msg("遍历会话文件失败", "Failed to traverse session files") }
func ErrSessionIDEmpty() string   { return Msg("会话 ID 不能为空", "Session ID cannot be empty") }
func ErrBuildSessionIndex() string { return Msg("构建会话索引失败", "Failed to build session index") }

// --- 知识库 ---

func ErrReadKnowledgeDir() string { return Msg("读取知识库目录失败", "Failed to read knowledge directory") }
func ErrDocumentNotFound(name string) string { return Msg("文档不存在: "+name, "Document not found: "+name) }
func ErrInvalidDocumentType() string { return Msg("无效的文档类型", "Invalid document type") }

// --- 导出 ---

func ErrExportFailed() string     { return Msg("导出失败", "Export failed") }
func ErrInvalidFormat() string    { return Msg("无效的导出格式", "Invalid export format") }

// --- 合规审计 ---

func ErrNoClaudeMD() string       { return Msg("未找到 CLAUDE.md 文件", "CLAUDE.md file not found") }
func ErrAuditFailed() string      { return Msg("审计失败", "Audit failed") }

// --- LLM ---

func ErrLLMNotConfigured() string { return Msg("LLM 未配置", "LLM not configured") }
func ErrLLMRequestFailed() string { return Msg("LLM 请求失败", "LLM request failed") }

// --- Diff ---

func ErrDiffFailed() string       { return Msg("获取 diff 失败", "Failed to get diff") }
func ErrGitNotAvailable() string  { return Msg("Git 不可用", "Git not available") }

// --- 监控 ---

func ErrMonitorStartFailed() string { return Msg("监控启动失败", "Failed to start monitoring") }

// --- 设置 ---

func ErrSettingsLoadFailed() string { return Msg("加载设置失败", "Failed to load settings") }
func ErrSettingsSaveFailed() string { return Msg("保存设置失败", "Failed to save settings") }
