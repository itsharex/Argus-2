// @ts-check
// Compliance Audit — 合规审计页面逻辑（LLM 驱动，嵌入知识库子视图）

// 合规审计状态
let currentComplianceOverview = null;
let currentRules = [];
let isAuditing = false;
let auditTimer = null;

// ============================================
// Initialization
// ============================================

async function initCompliance() {
    updateAuditButton(false);
}

// ============================================
// Audit Actions
// ============================================

async function startComplianceAudit() {
    // 清除可能残留的旧 timer（防止双击或异常退出后泄漏）
    if (auditTimer) { clearInterval(auditTimer); auditTimer = null; }
    if (isAuditing) return;

    // 优先使用知识库子视图传入的路径和项目名
    let claudeMDPath = null;
    let projectName = '';
    if (typeof currentClaudeMDPathForAudit !== 'undefined' && currentClaudeMDPathForAudit) {
        claudeMDPath = currentClaudeMDPathForAudit;
    }
    if (typeof currentProjectForAudit !== 'undefined' && currentProjectForAudit) {
        projectName = currentProjectForAudit;
    }
    if (!claudeMDPath) {
        claudeMDPath = await getCurrentClaudeMDPath();
    }
    if (!claudeMDPath) {
        showToast(t('noClaudeMD') || '未找到 CLAUDE.md 文件，请先在知识库中创建');
        return;
    }

    isAuditing = true;
    updateAuditButton(true);
    showAuditProgress();
    currentComplianceOverview = null;

    try {
        const overview = await window.go.main.App.GetComplianceOverview(claudeMDPath, projectName);
        currentComplianceOverview = overview;
        renderAuditResult(overview);
    } catch (error) {
        console.error('Audit failed:', error);
        showToast((t('auditFailed') || '审计失败') + ': ' + (error.message || error));
        showAuditError(error.message || String(error));
    } finally {
        isAuditing = false;
        updateAuditButton(false);
        if (auditTimer) { clearInterval(auditTimer); auditTimer = null; }
    }
}

async function getCurrentClaudeMDPath() {
    try {
        const docs = await window.go.main.App.GetKnowledgeDocuments('claudemd', '');
        if (docs && docs.length > 0) {
            return docs[0].path;
        }
    } catch (error) {
        console.error('Failed to get CLAUDE.md path:', error);
    }
    return null;
}

// ============================================
// UI State
// ============================================

function updateAuditButton(auditing) {
    // 知识库子视图中的审计按钮
    const btn = document.getElementById('knowledgeAuditStartBtn');
    if (!btn) return;

    if (auditing) {
        btn.disabled = true;
        btn.innerHTML = `
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="spin">
                <circle cx="12" cy="12" r="10"/>
                <path d="M12 6v6l4 2"/>
            </svg>
            <span data-i18n="auditing">${t('auditing') || '审计中...'}</span>
        `;
    } else {
        btn.disabled = false;
        btn.innerHTML = `
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/>
                <polyline points="22 4 12 14.01 9 11.01"/>
            </svg>
            <span data-i18n="startAudit">${t('startAudit') || '开始审计'}</span>
        `;
    }
}

function showAuditProgress() {
    const container = document.getElementById('knowledgeAuditResult');
    if (!container) return;

    const startTime = Date.now();

    container.innerHTML = `
        <div class="audit-loading">
            <div class="audit-spinner"></div>
            <p>${t('auditInProgress') || '正在使用 LLM 分析 CLAUDE.md 规则并审计会话...'}</p>
            <p class="audit-loading-hint" id="auditElapsed"></p>
        </div>
    `;

    // 更新已用时间
    if (auditTimer) clearInterval(auditTimer);
    auditTimer = setInterval(() => {
        const el = document.getElementById('auditElapsed');
        if (!el) { clearInterval(auditTimer); auditTimer = null; return; }
        const secs = Math.floor((Date.now() - startTime) / 1000);
        el.textContent = (t('auditElapsed') || '已用时') + ' ' + secs + 's';
    }, 1000);
}

function showAuditError(message) {
    const container = document.getElementById('knowledgeAuditResult');
    if (!container) return;

    container.innerHTML = `
        <div class="audit-error">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <circle cx="12" cy="12" r="10"/>
                <line x1="15" y1="9" x2="9" y2="15"/>
                <line x1="9" y1="9" x2="15" y2="15"/>
            </svg>
            <p>${escapeHtml(message)}</p>
            <p class="audit-error-hint">${t('auditErrorHint') || '请确认已配置 LLM 并重试'}</p>
        </div>
    `;
}

// ============================================
// Rendering
// ============================================

function renderAuditResult(overview) {
    const container = document.getElementById('knowledgeAuditResult');
    if (!container) return;

    if (!overview) {
        container.innerHTML = `<div class="audit-empty">${t('noData') || '暂无数据'}</div>`;
        return;
    }

    const o = normalizeKeys(overview);
    const score = o.averageScore || 0;
    const totalSessions = o.totalSessions || 0;
    const auditedSessions = o.auditedSessions || 0;
    const violations = o.violations || [];

    // 分数颜色
    let scoreClass = 'score-good';
    if (score < 60) scoreClass = 'score-bad';
    else if (score < 80) scoreClass = 'score-warn';

    container.innerHTML = `
        <!-- 概览卡片 -->
        <div class="audit-summary">
            <div class="audit-score-card ${scoreClass}">
                <div class="score-number">${score.toFixed(1)}%</div>
                <div class="score-desc">${t('averageScore') || '平均合规分数'}</div>
            </div>
            <div class="audit-stats-row">
                <div class="audit-stat">
                    <span class="stat-num">${auditedSessions}</span>
                    <span class="stat-desc">${t('auditedSessions') || '已审计'}</span>
                </div>
                <div class="audit-stat">
                    <span class="stat-num">${totalSessions}</span>
                    <span class="stat-desc">${t('totalSessions') || '总会话'}</span>
                </div>
                <div class="audit-stat">
                    <span class="stat-num">${violations.length}</span>
                    <span class="stat-desc">${t('violationTypes') || '违规类型'}</span>
                </div>
            </div>
        </div>

        <!-- 违规列表 -->
        <div class="audit-violations-section">
            <h3>${t('violationDetails') || '违规详情'}</h3>
            ${violations.length === 0
                ? `<div class="audit-no-violations">${t('noViolations') || '无违规记录，所有会话均符合 CLAUDE.md 规则'}</div>`
                : `<div class="audit-violations-list">
                    ${violations.map(v => {
                        const n = normalizeKeys(v);
                        return `
                        <div class="audit-violation-item severity-${n.severity || 'low'}">
                            <div class="violation-left">
                                <span class="violation-severity-badge severity-${n.severity || 'low'}">${getSeverityLabel(n.severity)}</span>
                                <span class="violation-rule">${escapeHtml(n.rule || '')}</span>
                            </div>
                            <span class="violation-count">${n.count || 0} ${t('times') || '次'}</span>
                        </div>
                    `}).join('')}
                </div>`
            }
        </div>
    `;
}

// ============================================
// Helper Functions
// ============================================

function normalizeKeys(obj) {
    if (Array.isArray(obj)) {
        return obj.map(item => normalizeKeys(item));
    }
    if (obj !== null && typeof obj === 'object' && !(obj instanceof Date)) {
        const result = {};
        for (const [key, value] of Object.entries(obj)) {
            const camelKey = key.charAt(0).toLowerCase() + key.slice(1);
            result[camelKey] = normalizeKeys(value);
        }
        return result;
    }
    return obj;
}

function getSeverityLabel(severity) {
    const labels = {
        'high': t('high') || '高',
        'medium': t('medium') || '中',
        'low': t('low') || '低'
    };
    return labels[severity] || severity;
}

// ============================================
// Export
// ============================================

function exportComplianceReport() {
    if (!currentComplianceOverview) {
        showToast(t('noData') || '暂无数据');
        return;
    }

    const o = normalizeKeys(currentComplianceOverview);
    let report = `# ${t('complianceReport') || '合规审计报告'}\n\n`;
    report += `**${t('generatedAt') || '生成时间'}**: ${new Date().toLocaleString()}\n\n`;
    report += `## ${t('overview') || '概览'}\n\n`;
    report += `- **${t('averageScore') || '平均分数'}**: ${(o.averageScore || 0).toFixed(1)}%\n`;
    report += `- **${t('auditedSessions') || '已审计会话'}**: ${o.auditedSessions || 0}\n`;
    report += `- **${t('totalSessions') || '总会话数'}**: ${o.totalSessions || 0}\n\n`;

    const violations = o.violations || [];
    if (violations.length > 0) {
        report += `## ${t('violationDetails') || '违规详情'}\n\n`;
        violations.forEach(v => {
            const n = normalizeKeys(v);
            report += `- **${n.rule || ''}**: ${n.count || 0} ${t('times') || '次'} (${getSeverityLabel(n.severity)})\n`;
        });
    }

    const blob = new Blob([report], { type: 'text/markdown' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'compliance-report.md';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
}
