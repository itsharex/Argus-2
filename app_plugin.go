package main

import (
	"fmt"

	"argus-desktop/internal/plugin"
)

// ============================================
// Plugin Studio API
// ============================================

// PluginSettingsDTO 插件工作室配置（前端展示用）
type PluginSettingsDTO struct {
	Hooks      []HookConfigDTO      `json:"hooks"`
	MCPServers []MCPServerConfigDTO `json:"mcpServers"`
}

// HookConfigDTO Hook 配置（前端展示用）
type HookConfigDTO struct {
	Type     string   `json:"type"`
	Matcher  string   `json:"matcher"`
	Commands []string `json:"commands"`
	Enabled  bool     `json:"enabled"`
}

// MCPServerConfigDTO MCP 服务器配置（前端展示用）
type MCPServerConfigDTO struct {
	Name      string            `json:"name"`
	Transport string            `json:"transport"`
	Command   string            `json:"command"`
	URL       string            `json:"url"`
	Args      []string          `json:"args"`
	Env       map[string]string `json:"env"`
	Enabled   bool              `json:"enabled"`
}

// HookTemplateDTO Hook 模板（前端展示用）
type HookTemplateDTO struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Category    string        `json:"category"`
	Hook        HookConfigDTO `json:"hook"`
}

// ValidationErrorDTO 验证错误（前端展示用）
type ValidationErrorDTO struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// GetPluginSettings 获取插件工作室配置
func (a *App) GetPluginSettings(projectDir string) (*PluginSettingsDTO, error) {
	if a.plugin == nil {
		return &PluginSettingsDTO{
			Hooks:      []HookConfigDTO{},
			MCPServers: []MCPServerConfigDTO{},
		}, nil
	}

	settings, err := a.plugin.LoadSettings(projectDir)
	if err != nil {
		return nil, fmt.Errorf("加载插件配置失败: %w", err)
	}

	// 转换为 DTO
	hooks := make([]HookConfigDTO, len(settings.Hooks))
	for i, h := range settings.Hooks {
		hooks[i] = HookConfigDTO{
			Type:     string(h.Type),
			Matcher:  h.Matcher,
			Commands: h.Commands,
			Enabled:  h.Enabled,
		}
	}

	mcpServers := make([]MCPServerConfigDTO, len(settings.MCPServers))
	for i, s := range settings.MCPServers {
		mcpServers[i] = MCPServerConfigDTO{
			Name:      s.Name,
			Transport: string(s.Transport),
			Command:   s.Command,
			URL:       s.URL,
			Args:      s.Args,
			Env:       s.Env,
			Enabled:   s.Enabled,
		}
	}

	return &PluginSettingsDTO{
		Hooks:      hooks,
		MCPServers: mcpServers,
	}, nil
}

// SavePluginSettings 保存插件工作室配置
func (a *App) SavePluginSettings(projectDir string, settings *PluginSettingsDTO) error {
	if a.plugin == nil {
		return fmt.Errorf("插件工作室引擎未初始化")
	}

	// 转换 DTO 为内部类型
	hooks := make([]plugin.HookConfig, len(settings.Hooks))
	for i, h := range settings.Hooks {
		hooks[i] = plugin.HookConfig{
			Type:     plugin.HookType(h.Type),
			Matcher:  h.Matcher,
			Commands: h.Commands,
			Enabled:  h.Enabled,
		}
	}

	mcpServers := make([]plugin.MCPServerConfig, len(settings.MCPServers))
	for i, s := range settings.MCPServers {
		mcpServers[i] = plugin.MCPServerConfig{
			Name:      s.Name,
			Transport: plugin.TransportType(s.Transport),
			Command:   s.Command,
			URL:       s.URL,
			Args:      s.Args,
			Env:       s.Env,
			Enabled:   s.Enabled,
		}
	}

	internalSettings := &plugin.PluginSettings{
		Hooks:      hooks,
		MCPServers: mcpServers,
	}

	return a.plugin.SaveSettings(projectDir, internalSettings)
}

// AddPluginHook 添加 Hook 配置
func (a *App) AddPluginHook(projectDir string, hook HookConfigDTO) error {
	if a.plugin == nil {
		return fmt.Errorf("插件工作室引擎未初始化")
	}

	return a.plugin.AddHook(projectDir, plugin.HookConfig{
		Type:     plugin.HookType(hook.Type),
		Matcher:  hook.Matcher,
		Commands: hook.Commands,
		Enabled:  hook.Enabled,
	})
}

// UpdatePluginHook 更新 Hook 配置
func (a *App) UpdatePluginHook(projectDir string, index int, hook HookConfigDTO) error {
	if a.plugin == nil {
		return fmt.Errorf("插件工作室引擎未初始化")
	}

	return a.plugin.UpdateHook(projectDir, index, plugin.HookConfig{
		Type:     plugin.HookType(hook.Type),
		Matcher:  hook.Matcher,
		Commands: hook.Commands,
		Enabled:  hook.Enabled,
	})
}

// RemovePluginHook 删除 Hook 配置
func (a *App) RemovePluginHook(projectDir string, index int) error {
	if a.plugin == nil {
		return fmt.Errorf("插件工作室引擎未初始化")
	}

	return a.plugin.RemoveHook(projectDir, index)
}

// AddMCPServer 添加 MCP 服务器配置
func (a *App) AddMCPServer(projectDir string, server MCPServerConfigDTO) error {
	if a.plugin == nil {
		return fmt.Errorf("插件工作室引擎未初始化")
	}

	return a.plugin.AddMCPServer(projectDir, plugin.MCPServerConfig{
		Name:      server.Name,
		Transport: plugin.TransportType(server.Transport),
		Command:   server.Command,
		URL:       server.URL,
		Args:      server.Args,
		Env:       server.Env,
		Enabled:   server.Enabled,
	})
}

// UpdateMCPServer 更新 MCP 服务器配置
func (a *App) UpdateMCPServer(projectDir string, index int, server MCPServerConfigDTO) error {
	if a.plugin == nil {
		return fmt.Errorf("插件工作室引擎未初始化")
	}

	return a.plugin.UpdateMCPServer(projectDir, index, plugin.MCPServerConfig{
		Name:      server.Name,
		Transport: plugin.TransportType(server.Transport),
		Command:   server.Command,
		URL:       server.URL,
		Args:      server.Args,
		Env:       server.Env,
		Enabled:   server.Enabled,
	})
}

// RemoveMCPServer 删除 MCP 服务器配置
func (a *App) RemoveMCPServer(projectDir string, index int) error {
	if a.plugin == nil {
		return fmt.Errorf("插件工作室引擎未初始化")
	}

	return a.plugin.RemoveMCPServer(projectDir, index)
}

// GetHookTemplates 获取 Hook 模板列表
func (a *App) GetHookTemplates() ([]HookTemplateDTO, error) {
	if a.plugin == nil {
		return nil, fmt.Errorf("插件工作室引擎未初始化")
	}

	templates := a.plugin.GetHookTemplates()
	result := make([]HookTemplateDTO, len(templates))
	for i, t := range templates {
		result[i] = HookTemplateDTO{
			Name:        t.Name,
			Description: t.Description,
			Category:    t.Category,
			Hook: HookConfigDTO{
				Type:     string(t.Hook.Type),
				Matcher:  t.Hook.Matcher,
				Commands: t.Hook.Commands,
				Enabled:  t.Hook.Enabled,
			},
		}
	}

	return result, nil
}

// ApplyHookTemplate 应用 Hook 模板
func (a *App) ApplyHookTemplate(projectDir string, template HookTemplateDTO) error {
	if a.plugin == nil {
		return fmt.Errorf("插件工作室引擎未初始化")
	}

	return a.plugin.ApplyHookTemplate(projectDir, plugin.HookTemplate{
		Name:        template.Name,
		Description: template.Description,
		Category:    template.Category,
		Hook: plugin.HookConfig{
			Type:     plugin.HookType(template.Hook.Type),
			Matcher:  template.Hook.Matcher,
			Commands: template.Hook.Commands,
			Enabled:  template.Hook.Enabled,
		},
	})
}

// ValidatePluginSettings 验证插件配置
func (a *App) ValidatePluginSettings(settings *PluginSettingsDTO) ([]ValidationErrorDTO, error) {
	if a.plugin == nil {
		return nil, fmt.Errorf("插件工作室引擎未初始化")
	}

	// 转换 DTO 为内部类型
	hooks := make([]plugin.HookConfig, len(settings.Hooks))
	for i, h := range settings.Hooks {
		hooks[i] = plugin.HookConfig{
			Type:     plugin.HookType(h.Type),
			Matcher:  h.Matcher,
			Commands: h.Commands,
			Enabled:  h.Enabled,
		}
	}

	mcpServers := make([]plugin.MCPServerConfig, len(settings.MCPServers))
	for i, s := range settings.MCPServers {
		mcpServers[i] = plugin.MCPServerConfig{
			Name:      s.Name,
			Transport: plugin.TransportType(s.Transport),
			Command:   s.Command,
			URL:       s.URL,
			Args:      s.Args,
			Env:       s.Env,
			Enabled:   s.Enabled,
		}
	}

	internalSettings := &plugin.PluginSettings{
		Hooks:      hooks,
		MCPServers: mcpServers,
	}

	errors := a.plugin.ValidateSettings(internalSettings)
	result := make([]ValidationErrorDTO, len(errors))
	for i, e := range errors {
		result[i] = ValidationErrorDTO{
			Field:   e.Field,
			Message: e.Message,
		}
	}

	return result, nil
}
