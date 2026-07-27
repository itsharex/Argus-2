package compliance

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"

	"argus-desktop/internal/llm"
	"argus-desktop/internal/session"
)

// Engine 合规审计引擎
type Engine struct {
	auditor *LLMAuditor
	cache   *Cache
}

// NewEngine 创建新的合规审计引擎。
func NewEngine(cfg llm.ProviderConfig) *Engine {
	cache, err := NewCache()
	if err != nil {
		log.Printf("合规审计缓存初始化失败: %v", err)
		// 使用空缓存继续
		cache = &Cache{entries: make(map[string]*CacheEntry)}
	}

	return &Engine{
		auditor: NewLLMAuditor(cfg),
		cache:   cache,
	}
}

// ExtractRules 使用 LLM 从 CLAUDE.md 提取规则。
func (e *Engine) ExtractRules(ctx context.Context, claudeMDContent string) ([]ComplianceRule, error) {
	return e.auditor.ExtractRules(ctx, claudeMDContent)
}

// AuditSession 审计单个会话（使用缓存）。
func (e *Engine) AuditSession(ctx context.Context, rules []ComplianceRule, sess *session.Session) (*ComplianceScore, error) {
	rulesHash := RulesHash(rules)

	// 检查缓存
	if cached, ok := e.cache.Get(sess.ID, rulesHash); ok {
		log.Printf("会话 %s 使用缓存结果", sess.ID)
		return cached, nil
	}

	// 调用 LLM 审计
	score, err := e.auditor.AuditSession(ctx, rules, sess)
	if err != nil {
		return nil, err
	}

	// 缓存结果
	e.cache.Set(sess.ID, rulesHash, score)

	return score, nil
}

// GetComplianceOverview 获取所有会话的合规概览。
func (e *Engine) GetComplianceOverview(ctx context.Context, sessions []*session.Session, claudeMDContent string) (*ComplianceOverview, error) {
	if len(sessions) == 0 {
		return &ComplianceOverview{
			AverageScore:    0,
			TotalSessions:   0,
			AuditedSessions: 0,
			Violations:      []ViolationStat{},
			Sessions:        []SessionAuditResult{},
		}, nil
	}

	// Step 1: 提取规则
	rules, err := e.ExtractRules(ctx, claudeMDContent)
	if err != nil {
		return nil, fmt.Errorf("规则提取失败: %w", err)
	}
	log.Printf("从 CLAUDE.md 提取了 %d 条规则", len(rules))

	// Step 2: 逐个审计会话（并发，限制并发数）
	type result struct {
		index int
		score *ComplianceScore
		err   error
	}

	results := make([]result, len(sessions))
	sem := make(chan struct{}, 5) // 最多 5 个并发 LLM 调用
	var wg sync.WaitGroup

	for i, sess := range sessions {
		wg.Add(1)
		go func(idx int, s *session.Session) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			score, err := e.AuditSession(ctx, rules, s)
			idPrefix := s.ID
			if len(s.ID) > 8 {
				idPrefix = s.ID[:8]
			}
			log.Printf("会话审计进度: %d/%d (会话 %s)", idx+1, len(sessions), idPrefix)
			results[idx] = result{index: idx, score: score, err: err}
		}(i, sess)
	}
	wg.Wait()

	// Step 3: 聚合结果
	totalScore := 0.0
	auditedCount := 0
	var sessionResults []SessionAuditResult
	violationCounts := make(map[string]*ViolationStat)

	for _, r := range results {
		if r.err != nil {
			log.Printf("会话 %s 审计失败: %v", sessions[r.index].ID, r.err)
			continue
		}
		if r.score == nil {
			continue
		}

		totalScore += r.score.Overall
		auditedCount++
		sessionResults = append(sessionResults, SessionAuditResult{
			SessionID: sessions[r.index].ID,
			Score:     r.score,
		})

		// 统计违规
		for _, v := range r.score.Violations {
			if v.Compliant {
				continue
			}
			key := v.Rule
			if existing, ok := violationCounts[key]; ok {
				existing.Count++
			} else {
				violationCounts[key] = &ViolationStat{
					Rule:     v.Rule,
					Count:    1,
					Severity: v.Severity,
				}
			}
		}
	}

	// 按违规次数排序
	var violations []ViolationStat
	for _, v := range violationCounts {
		violations = append(violations, *v)
	}
	sort.Slice(violations, func(i, j int) bool {
		return violations[i].Count > violations[j].Count
	})

	averageScore := 0.0
	if auditedCount > 0 {
		averageScore = totalScore / float64(auditedCount)
	}

	return &ComplianceOverview{
		AverageScore:    averageScore,
		TotalSessions:   len(sessions),
		AuditedSessions: auditedCount,
		Violations:      violations,
		Sessions:        sessionResults,
	}, nil
}
