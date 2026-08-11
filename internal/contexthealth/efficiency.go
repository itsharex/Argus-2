package contexthealth

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"argus-desktop/internal/session/claude"
)

// EfficiencyReport 上下文效率分析报告
type EfficiencyReport struct {
	// 缓存效率
	CacheHitRate        float64 `json:"cacheHitRate"`        // 缓存命中率 0-100
	CacheReadTokens     int     `json:"cacheReadTokens"`     // 缓存读取 token 总计
	CacheCreationTokens int     `json:"cacheCreationTokens"` // 缓存创建 token 总计
	TotalInputTokens    int     `json:"totalInputTokens"`    // 总输入 token

	// 上下文开销分解
	CLAUDEMDTokens     int      `json:"claudeMdTokens"`     // CLAUDE.md 估算 token 数
	CLAUDEMDPath       string   `json:"claudeMdPath"`       // CLAUDE.md 文件路径
	CLAUDEMDSizeBytes  int      `json:"claudeMdSizeBytes"`  // CLAUDE.md 文件大小（字节）
	SkillsCount        int      `json:"skillsCount"`        // 已启用 Skills 数量
	MCPServersCount    int      `json:"mcpServersCount"`    // 活跃 MCP Server 数
	MCPToolsTotal      int      `json:"mcpToolsTotal"`      // MCP 工具总数
	UnusedMCPTools     []string `json:"unusedMcpTools"`     // 未使用的 MCP 工具
	ConversationTokens int      `json:"conversationTokens"` // 对话内容 token 估算

	// 评分与建议
	EfficiencyScore int      `json:"efficiencyScore"` // 0-100 效率评分
	Recommendations []string `json:"recommendations"`  // 优化建议
}

// EfficiencyAnalyzer 上下文效率分析器
type EfficiencyAnalyzer struct {
	analyzer *Analyzer
}

// NewEfficiencyAnalyzer 创建效率分析器
func NewEfficiencyAnalyzer() *EfficiencyAnalyzer {
	return &EfficiencyAnalyzer{
		analyzer: NewAnalyzer(),
	}
}

// AnalyzeSessionEfficiency 分析单个会话的上下文效率
func (ea *EfficiencyAnalyzer) AnalyzeSessionEfficiency(jsonlPath string, claudeDir string) (*EfficiencyReport, error) {
	report := &EfficiencyReport{
		UnusedMCPTools:  []string{},
		Recommendations: []string{},
	}

	// 1. 从 JSONL 提取 token 数据和工具使用信息
	usedTools, err := ea.extractTokenData(jsonlPath, report)
	if err != nil {
		return nil, fmt.Errorf("提取 token 数据失败: %w", err)
	}

	// 2. 分析 CLAUDE.md 开销
	ea.analyzeCLAUDEMD(claudeDir, report)

	// 3. 分析 MCP 和 Skills 配置
	ea.analyzePluginConfig(claudeDir, report, usedTools)

	// 4. 计算效率评分
	ea.calculateEfficiencyScore(report)

	// 5. 生成优化建议
	ea.generateRecommendations(report)

	return report, nil
}

// AnalyzeGlobalEfficiency 分析全局上下文效率
func (ea *EfficiencyAnalyzer) AnalyzeGlobalEfficiency(claudeDir string) (*EfficiencyReport, error) {
	report := &EfficiencyReport{
		UnusedMCPTools:  []string{},
		Recommendations: []string{},
	}

	// 遍历所有项目目录的 JSONL 文件
	projectDirs, err := filepath.Glob(filepath.Join(claudeDir, "projects", "*"))
	if err != nil {
		return nil, fmt.Errorf("遍历项目目录失败: %w", err)
	}

	var totalCacheRead, totalCacheWrite, totalInput int

	for _, projectDir := range projectDirs {
		info, err := os.Stat(projectDir)
		if err != nil || !info.IsDir() {
			continue
		}

		jsonlFiles, err := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
		if err != nil {
			continue
		}

		for _, jsonlPath := range jsonlFiles {
			cacheRead, cacheWrite, input, err := ea.extractTokenCounts(jsonlPath)
			if err != nil {
				continue
			}
			totalCacheRead += cacheRead
			totalCacheWrite += cacheWrite
			totalInput += input
		}
	}

	report.CacheReadTokens = totalCacheRead
	report.CacheCreationTokens = totalCacheWrite
	report.TotalInputTokens = totalInput

	cacheDenom := totalCacheRead + totalCacheWrite + totalInput
	if cacheDenom > 0 {
		report.CacheHitRate = float64(totalCacheRead) / float64(cacheDenom) * 100
	}

	// 分析 CLAUDE.md 和插件配置
	ea.analyzeCLAUDEMD(claudeDir, report)
	ea.analyzePluginConfig(claudeDir, report, nil)

	ea.calculateEfficiencyScore(report)
	ea.generateRecommendations(report)

	return report, nil
}

// extractTokenData 从 JSONL 提取 token 数据和工具使用信息
// 返回已使用的工具名集合
func (ea *EfficiencyAnalyzer) extractTokenData(jsonlPath string, report *EfficiencyReport) (map[string]bool, error) {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var totalCacheRead, totalCacheWrite, totalInput int
	usedTools := make(map[string]bool)

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		var event claude.JSONLLine
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		// 提取 token 使用数据
		if event.Type == "assistant" && event.Message != nil && event.Message.Usage != nil {
			totalCacheRead += event.Message.Usage.CacheReadTokens
			totalCacheWrite += event.Message.Usage.CacheCreationTokens
			totalInput += event.Message.Usage.InputTokens
		}

		// 提取工具使用信息
		if event.Type == "assistant" && event.Message != nil {
			if blocks, ok := event.Message.Content.([]any); ok {
				for _, block := range blocks {
					blockMap, ok := block.(map[string]any)
					if !ok {
						continue
					}
					if blockMap["type"] == "tool_use" {
						if name, ok := blockMap["name"].(string); ok {
							usedTools[name] = true
						}
					}
				}
			}
		}
	}

	report.CacheReadTokens = totalCacheRead
	report.CacheCreationTokens = totalCacheWrite
	report.TotalInputTokens = totalInput

	cacheDenom := totalCacheRead + totalCacheWrite + totalInput
	if cacheDenom > 0 {
		report.CacheHitRate = float64(totalCacheRead) / float64(cacheDenom) * 100
	}

	return usedTools, scanner.Err()
}

// extractTokenCounts 从 JSONL 提取 token 计数（轻量级，不解析 content）
func (ea *EfficiencyAnalyzer) extractTokenCounts(jsonlPath string) (cacheRead, cacheWrite, input int, err error) {
	file, err := os.Open(jsonlPath)
	if err != nil {
		return 0, 0, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" || !strings.Contains(line, `"usage"`) {
			continue
		}

		var event claude.JSONLLine
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}

		if event.Type == "assistant" && event.Message != nil && event.Message.Usage != nil {
			cacheRead += event.Message.Usage.CacheReadTokens
			cacheWrite += event.Message.Usage.CacheCreationTokens
			input += event.Message.Usage.InputTokens
		}
	}

	return cacheRead, cacheWrite, input, scanner.Err()
}

// analyzeCLAUDEMD 分析 CLAUDE.md 文件开销
func (ea *EfficiencyAnalyzer) analyzeCLAUDEMD(claudeDir string, report *EfficiencyReport) {
	homeDir := filepath.Dir(claudeDir)

	// 查找 CLAUDE.md 文件（优先项目级，其次用户级）
	paths := []string{
		filepath.Join(homeDir, "CLAUDE.md"),
		filepath.Join(claudeDir, "CLAUDE.md"),
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		report.CLAUDEMDPath = path
		report.CLAUDEMDSizeBytes = len(data)
		// 估算 token 数：按字节 / 3（混合中英文估算）
		report.CLAUDEMDTokens = len(data) / 3
		return
	}
}

// analyzePluginConfig 分析插件配置（MCP servers、Skills）
func (ea *EfficiencyAnalyzer) analyzePluginConfig(claudeDir string, report *EfficiencyReport, usedTools map[string]bool) {
	homeDir := filepath.Dir(claudeDir)

	// 读取 settings.json
	settingsPath := filepath.Join(claudeDir, "settings.json")
	data, err := os.ReadFile(settingsPath)
	if err != nil {
		settingsPath = filepath.Join(homeDir, ".claude", "settings.json")
		data, err = os.ReadFile(settingsPath)
		if err != nil {
			return
		}
	}

	var settings struct {
		MCPServers map[string]struct {
			Transport string `json:"transport"`
			Command   string `json:"command"`
			URL       string `json:"url"`
		} `json:"mcpServers"`
	}

	if err := json.Unmarshal(data, &settings); err != nil {
		return
	}

	report.MCPServersCount = len(settings.MCPServers)

	// 检查 Skills 目录
	skillsDir := filepath.Join(claudeDir, "skills")
	if _, err := os.Stat(skillsDir); err == nil {
		entries, err := os.ReadDir(skillsDir)
		if err == nil {
			report.SkillsCount = len(entries)
		}
	}

	// 估算 MCP 工具总数
	report.MCPToolsTotal = report.MCPServersCount * 5

	// 检测未使用的 MCP 工具（需要有 usedTools 数据时才比较）
	if usedTools != nil && report.MCPToolsTotal > 0 && len(usedTools) == 0 {
		// 如果配置了 MCP server 但没有任何工具被使用，标记所有为未使用
		// 注意：完整的工具列表需要查询 MCP server，这里只做基本检测
		report.UnusedMCPTools = []string{"（需连接 MCP Server 获取完整工具列表）"}
	}
}

// calculateEfficiencyScore 计算效率评分 (0-100)
func (ea *EfficiencyAnalyzer) calculateEfficiencyScore(report *EfficiencyReport) {
	score := 100.0

	// 缓存命中率评分（权重 40%）
	if report.TotalInputTokens > 0 {
		if report.CacheHitRate < 20 {
			score -= 40
		} else if report.CacheHitRate < 40 {
			score -= 30
		} else if report.CacheHitRate < 60 {
			score -= 20
		} else if report.CacheHitRate < 80 {
			score -= 10
		}
	}

	// CLAUDE.md 开销评分（权重 25%）
	if report.CLAUDEMDTokens > 10000 {
		score -= 25
	} else if report.CLAUDEMDTokens > 5000 {
		score -= 15
	} else if report.CLAUDEMDTokens > 2000 {
		score -= 8
	}

	// MCP 工具使用率评分（权重 20%）
	if report.MCPToolsTotal > 0 && len(report.UnusedMCPTools) > 0 {
		usageRate := 1.0 - float64(len(report.UnusedMCPTools))/float64(report.MCPToolsTotal)
		if usageRate < 0.3 {
			score -= 20
		} else if usageRate < 0.5 {
			score -= 12
		} else if usageRate < 0.7 {
			score -= 5
		}
	}

	// Skills 数量评分（权重 15%）
	if report.SkillsCount > 15 {
		score -= 15
	} else if report.SkillsCount > 10 {
		score -= 10
	} else if report.SkillsCount > 5 {
		score -= 5
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}

	report.EfficiencyScore = int(score)
}

// generateRecommendations 生成优化建议
func (ea *EfficiencyAnalyzer) generateRecommendations(report *EfficiencyReport) {
	recommendations := []string{}

	// 缓存命中率建议
	if report.TotalInputTokens > 0 {
		if report.CacheHitRate < 40 {
			recommendations = append(recommendations, "缓存命中率较低（"+fmt.Sprintf("%.1f", report.CacheHitRate)+"%），建议减少上下文变更频率，保持提示词结构稳定")
		} else if report.CacheHitRate < 70 {
			recommendations = append(recommendations, "缓存命中率中等（"+fmt.Sprintf("%.1f", report.CacheHitRate)+"%），可通过优化提示词结构进一步提升")
		}
	}

	// CLAUDE.md 开销建议
	if report.CLAUDEMDTokens > 5000 {
		recommendations = append(recommendations, fmt.Sprintf("CLAUDE.md 占用约 %d tokens（约 %.1fKB），建议精简内容或使用 #include 拆分", report.CLAUDEMDTokens, float64(report.CLAUDEMDSizeBytes)/1024))
	}

	// MCP 工具建议
	if len(report.UnusedMCPTools) > 0 && (len(report.UnusedMCPTools) != 1 || !strings.HasPrefix(report.UnusedMCPTools[0], "（")) {
		recommendations = append(recommendations, fmt.Sprintf("发现 %d 个未使用的 MCP 工具，建议禁用以减少上下文开销", len(report.UnusedMCPTools)))
	}

	// Skills 建议
	if report.SkillsCount > 10 {
		recommendations = append(recommendations, fmt.Sprintf("已启用 %d 个 Skills，建议审查必要性，禁用不常用的 Skills", report.SkillsCount))
	}

	// 无建议时的积极反馈
	if len(recommendations) == 0 {
		recommendations = append(recommendations, "当前上下文配置已较优，无需调整")
	}

	report.Recommendations = recommendations
}
