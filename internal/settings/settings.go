// Package settings provides application settings management.
package settings

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"sync"

	"argus-desktop/internal/session"
)

// Theme 主题类型
type Theme string

const (
	ThemeLight Theme = "light"
	ThemeDark  Theme = "dark"
	ThemeAuto  Theme = "auto"
)

// LLMConfig 大模型配置
type LLMConfig struct {
	Provider string `json:"provider"` // 用户自定义提供商名称
	APIKey   string `json:"apiKey"`   // 用户 API Key
	BaseURL  string `json:"baseUrl"`  // 请求地址
	Model    string `json:"model"`    // 模型名
	Enabled  bool   `json:"enabled"`  // 是否启用
}

// Settings 应用设置
type Settings struct {
	mu sync.RWMutex `json:"-"`

	// 主题设置
	Theme Theme `json:"theme"`

	// 风险规则
	CustomRules []CustomRule `json:"customRules"`

	// LLM 配置
	LLM LLMConfig `json:"llm"`
}

// CustomRule 自定义风险规则
type CustomRule struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Level       session.RiskLevel `json:"level"`
	Pattern     string           `json:"pattern"` // 文件路径匹配模式
	Enabled     bool             `json:"enabled"`
}

// Manager 设置管理器
type Manager struct {
	settings *Settings
	filePath string
}

// NewManager 创建设置管理器
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configDir := filepath.Join(homeDir, ".argus")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, err
	}

	m := &Manager{
		settings: &Settings{
			Theme:       ThemeAuto,
			CustomRules: []CustomRule{},
		},
		filePath: filepath.Join(configDir, "settings.json"),
	}

	// 加载已有的设置
	if err := m.load(); err != nil {
		// 如果文件不存在，使用默认设置
		// 对于其他错误（如文件损坏），记录日志并使用默认设置
		if !os.IsNotExist(err) {
			log.Printf("WARN: 加载设置失败: %v，使用默认设置", err)
		}
	}

	return m, nil
}

// Get 获取设置
func (m *Manager) Get() *Settings {
	m.settings.mu.RLock()
	defer m.settings.mu.RUnlock()

	// 返回副本（不复制锁）
	result := &Settings{
		Theme:       m.settings.Theme,
		CustomRules: make([]CustomRule, len(m.settings.CustomRules)),
		LLM:         m.settings.LLM,
	}
	copy(result.CustomRules, m.settings.CustomRules)
	return result
}

// UpdateTheme 更新主题设置
func (m *Manager) UpdateTheme(theme Theme) error {
	m.settings.mu.Lock()
	m.settings.Theme = theme
	data := m.copySettingsJSON()
	m.settings.mu.Unlock()

	return m.writeSettings(data)
}

// AddCustomRule 添加自定义规则
func (m *Manager) AddCustomRule(rule CustomRule) error {
	m.settings.mu.Lock()
	m.settings.CustomRules = append(m.settings.CustomRules, rule)
	data := m.copySettingsJSON()
	m.settings.mu.Unlock()

	return m.writeSettings(data)
}

// RemoveCustomRule 删除自定义规则
func (m *Manager) RemoveCustomRule(name string) error {
	m.settings.mu.Lock()
	found := false
	for i, rule := range m.settings.CustomRules {
		if rule.Name == name {
			m.settings.CustomRules = append(
				m.settings.CustomRules[:i],
				m.settings.CustomRules[i+1:]...,
			)
			found = true
			break
		}
	}
	var data []byte
	if found {
		data = m.copySettingsJSON()
	}
	m.settings.mu.Unlock()

	if !found {
		return nil
	}
	return m.writeSettings(data)
}

// UpdateCustomRule 更新自定义规则
func (m *Manager) UpdateCustomRule(name string, rule CustomRule) error {
	m.settings.mu.Lock()
	found := false
	for i, r := range m.settings.CustomRules {
		if r.Name == name {
			m.settings.CustomRules[i] = rule
			found = true
			break
		}
	}
	var data []byte
	if found {
		data = m.copySettingsJSON()
	}
	m.settings.mu.Unlock()

	if !found {
		return nil
	}
	return m.writeSettings(data)
}

// UpdateLLMConfig 更新 LLM 配置
func (m *Manager) UpdateLLMConfig(config LLMConfig) error {
	m.settings.mu.Lock()
	m.settings.LLM = config
	data := m.copySettingsJSON()
	m.settings.mu.Unlock()

	return m.writeSettings(data)
}

// GetLLMConfig 获取当前 LLM 配置
func (m *Manager) GetLLMConfig() LLMConfig {
	m.settings.mu.RLock()
	defer m.settings.mu.RUnlock()

	return m.settings.LLM
}

// load 从文件加载设置
func (m *Manager) load() error {
	data, err := os.ReadFile(m.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在，使用默认设置
		}
		return err
	}

	m.settings.mu.Lock()
	defer m.settings.mu.Unlock()

	return json.Unmarshal(data, m.settings)
}

// copySettingsJSON 在锁内复制设置并序列化为 JSON（不执行 I/O）
func (m *Manager) copySettingsJSON() []byte {
	data, _ := json.MarshalIndent(m.settings, "", "  ")
	return data
}

// writeSettings 将序列化后的 JSON 写入磁盘（原子写入，在锁外执行）
func (m *Manager) writeSettings(data []byte) error {
	if data == nil {
		return nil
	}
	tmpPath := m.filePath + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return err
	}
	return os.Rename(tmpPath, m.filePath)
}
