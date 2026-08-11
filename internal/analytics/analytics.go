package analytics

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"argus-desktop/internal/common"
	"argus-desktop/internal/session/claude"
)

type Engine struct {
	homeDir   string
	mu        sync.RWMutex
	cache     *TokenOverview
	refreshMu sync.Mutex
}

func NewEngine() (*Engine, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("analytics: failed to get home directory: %w", err)
	}
	return &Engine{homeDir: homeDir}, nil
}

func (e *Engine) Refresh() (*TokenOverview, error) {
	overview, err := e.scanAll()
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	e.cache = overview
	e.mu.Unlock()
	return overview, nil
}

func (e *Engine) GetOverview() (*TokenOverview, error) {
	// 快速路径：缓存命中时无需加锁
	e.mu.RLock()
	if e.cache != nil {
		result := e.cache
		e.mu.RUnlock()
		return result, nil
	}
	e.mu.RUnlock()

	// 慢路径：使用 refreshMu 防止并发刷新
	e.refreshMu.Lock()
	defer e.refreshMu.Unlock()

	// 再次检查缓存，避免重复刷新（此时已持有 refreshMu）
	e.mu.RLock()
	if e.cache != nil {
		result := e.cache
		e.mu.RUnlock()
		return result, nil
	}
	e.mu.RUnlock()

	return e.Refresh()
}

func (e *Engine) GetTrend(days int) ([]DailyUsage, error) {
	overview, err := e.GetOverview()
	if err != nil {
		return nil, err
	}
	if days <= 0 || days > len(overview.DailyTrend) {
		days = len(overview.DailyTrend)
	}
	return overview.DailyTrend[len(overview.DailyTrend)-days:], nil
}

func (e *Engine) GetProjectBreakdown() ([]ProjectStats, error) {
	overview, err := e.GetOverview()
	if err != nil {
		return nil, err
	}
	return overview.ProjectBreakdown, nil
}

func (e *Engine) GetModelBreakdown() ([]ModelStats, error) {
	overview, err := e.GetOverview()
	if err != nil {
		return nil, err
	}
	return overview.ModelBreakdown, nil
}

func (e *Engine) scanAll() (*TokenOverview, error) {
	claudeDir := filepath.Join(e.homeDir, ".claude", "projects")
	if _, err := os.Stat(claudeDir); os.IsNotExist(err) {
		return &TokenOverview{}, nil
	}
	entries, err := os.ReadDir(claudeDir)
	if err != nil {
		return nil, fmt.Errorf("analytics: failed to read Claude projects dir: %w", err)
	}
	projectMap := make(map[string]*ProjectStats)
	modelMap := make(map[string]*ModelStats)
	dailyMap := make(map[string]*DailyUsage)
	var totalInput, totalOutput, totalSessions int
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		projectDir := filepath.Join(claudeDir, entry.Name())
		jsonlFiles, _ := filepath.Glob(filepath.Join(projectDir, "*.jsonl"))
		for _, jsonlPath := range jsonlFiles {
			rec, err := e.parseJSONL(jsonlPath, entry.Name())
			if err != nil || rec == nil {
				continue
			}
			totalInput += rec.InputTokens
			totalOutput += rec.OutputTokens
			totalSessions++
			tokens := rec.InputTokens + rec.OutputTokens
			pStats, ok := projectMap[entry.Name()]
			if !ok {
				pStats = &ProjectStats{ProjectDir: entry.Name(), ProjectName: common.FormatProjectName(entry.Name())}
				projectMap[entry.Name()] = pStats
			}
			pStats.SessionCount++
			pStats.TotalTokens += tokens
			pStats.InputTokens += rec.InputTokens
			pStats.OutputTokens += rec.OutputTokens
			model := rec.Model
			if model == "" {
				model = "unknown"
			}
			mStats, ok := modelMap[model]
			if !ok {
				mStats = &ModelStats{Model: model}
				modelMap[model] = mStats
			}
			mStats.SessionCount++
			mStats.InputTokens += rec.InputTokens
			mStats.OutputTokens += rec.OutputTokens
			mStats.TotalTokens += tokens
			if rec.ModDay != "" {
				du, ok := dailyMap[rec.ModDay]
				if !ok {
					du = &DailyUsage{Date: rec.ModDay}
					dailyMap[rec.ModDay] = du
				}
				du.Tokens += tokens
				du.InputTokens += rec.InputTokens
				du.OutputTokens += rec.OutputTokens
				du.SessionCount++
			}
		}
	}
	now := time.Now()
	today := now.Format("2006-01-02")
	thisMonth := now.Format("2006-01")
	lastMonth := now.AddDate(0, -1, 0).Format("2006-01")
	var todayTokens, thisMonthTokens, lastMonthTokens int
	for _, du := range dailyMap {
		if du.Date == today {
			todayTokens += du.Tokens
		}
		dMonth := du.Date[:7]
		if dMonth == thisMonth {
			thisMonthTokens += du.Tokens
		} else if dMonth == lastMonth {
			lastMonthTokens += du.Tokens
		}
	}
	dailyTrend := make([]DailyUsage, 0, 30)
	for i := 29; i >= 0; i-- {
		d := now.AddDate(0, 0, -i).Format("2006-01-02")
		if du, ok := dailyMap[d]; ok {
			dailyTrend = append(dailyTrend, *du)
		} else {
			dailyTrend = append(dailyTrend, DailyUsage{Date: d})
		}
	}
	projectList := make([]ProjectStats, 0, len(projectMap))
	for _, p := range projectMap {
		projectList = append(projectList, *p)
	}
	sort.Slice(projectList, func(i, j int) bool { return projectList[i].TotalTokens > projectList[j].TotalTokens })
	modelList := make([]ModelStats, 0, len(modelMap))
	for _, m := range modelMap {
		modelList = append(modelList, *m)
	}
	sort.Slice(modelList, func(i, j int) bool { return modelList[i].TotalTokens > modelList[j].TotalTokens })
	monthTokenChange := 0.0
	if lastMonthTokens > 0 {
		monthTokenChange = (float64(thisMonthTokens-lastMonthTokens) / float64(lastMonthTokens)) * 100
	}
	return &TokenOverview{
		TotalInputTokens:  totalInput,
		TotalOutputTokens: totalOutput,
		TotalTokens:       totalInput + totalOutput,
		TotalSessions:     totalSessions,
		TodayTokens:       todayTokens,
		ThisMonthTokens:   thisMonthTokens,
		LastMonthTokens:   lastMonthTokens,
		MonthTokenChange:  monthTokenChange,
		ProjectBreakdown:  projectList,
		ModelBreakdown:    modelList,
		DailyTrend:        dailyTrend,
	}, nil
}

type sessionRecord struct {
	SessionID, Model, ProjectDir, ModDay string
	InputTokens, OutputTokens           int
}

func (e *Engine) parseJSONL(path, projectDirName string) (*sessionRecord, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	modDay := fi.ModTime().Format("2006-01-02")
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	rec := &sessionRecord{ProjectDir: projectDirName, ModDay: modDay}
	seenUUIDs := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	// 使用 1MB 初始缓冲区，最大 4MB，避免每次调用分配 50MB
	scanner.Buffer(make([]byte, 0, 1024*1024), 4*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		var event claude.JSONLLine
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue
		}
		if event.SessionID != "" && rec.SessionID == "" {
			rec.SessionID = event.SessionID
		}
		if event.Type == "assistant" && event.Message != nil {
			if event.Message.Model != "" && rec.Model == "" {
				rec.Model = event.Message.Model
			}
			if event.Message.Usage != nil {
				key := event.Timestamp
				if key == "" {
					key = fmt.Sprintf("%s:%d", rec.SessionID, len(seenUUIDs))
				}
				if !seenUUIDs[key] {
					seenUUIDs[key] = true
					rec.InputTokens += event.Message.Usage.InputTokens
					rec.OutputTokens += event.Message.Usage.OutputTokens
				}
			}
		}
	}
	if rec.SessionID == "" || (rec.InputTokens == 0 && rec.OutputTokens == 0) {
		return nil, nil
	}
	return rec, nil
}
