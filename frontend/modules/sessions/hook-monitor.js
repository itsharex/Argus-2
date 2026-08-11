// @ts-check
// Hook 执行日志监控 — 实时查看 Hook 执行记录与统计

// ============================================
// State
// ============================================

/** @type {Array} 最近的 Hook 执行记录 */
let hookExecutions = [];

/** @type {Object} Hook 执行统计 */
let hookStats = null;

/** @type {string} 当前过滤的 Hook 类型（空字符串表示全部） */
let hookFilterType = '';

/** @type {number} 显示条数限制 */
let hookDisplayLimit = 100;

// ============================================
// Initialization
// ============================================

/**
 * 初始化 Hook 执行日志监控
 * 在 Plugin Studio 面板激活时调用
 */
async function initHookMonitor() {
    try {
        // 监听后端告警事件
        if (window.runtime && window.runtime.EventsOn) {
            window.runtime.EventsOn('hook-alert', (alert) => {
                handleHookAlert(alert);
            });
        }

        // 加载数据
        await refreshHookLogs();
    } catch (error) {
        console.error('Failed to initialize hook monitor:', error);
    }
}

// ============================================
// Data Loading
// ============================================

/**
 * 刷新 Hook 执行日志
 */
async function refreshHookLogs() {
    try {
        // 并行加载执行记录和统计
        const [executions, stats] = await Promise.all([
            window.go.main.App.GetHookExecutions(hookDisplayLimit, hookFilterType),
            window.go.main.App.GetHookStats()
        ]);

        hookExecutions = executions || [];
        hookStats = stats;

        renderHookLogStats();
        renderHookLogTable();
    } catch (error) {
        console.error('Failed to refresh hook logs:', error);
        showToast(t('refreshFailed') || '刷新失败');
    }
}

/**
 * 清空 Hook 执行日志
 */
async function clearHookLogs() {
    try {
        await window.go.main.App.ClearHookLogs();
        hookExecutions = [];
        hookStats = null;
        renderHookLogStats();
        renderHookLogTable();
        showToast(t('hookLogsCleared') || '日志已清空');
    } catch (error) {
        console.error('Failed to clear hook logs:', error);
        showToast(t('clearFailed') || '清空失败');
    }
}

/**
 * 切换 Hook 类型过滤
 * @param {string} hookType - Hook 类型（空字符串表示全部）
 */
function filterHookLogs(hookType) {
    hookFilterType = hookType;
    refreshHookLogs();
}

// ============================================
// Rendering — 统计卡片
// ============================================

/**
 * 渲染 Hook 执行统计卡片
 */
function renderHookLogStats() {
    const container = document.getElementById('hookLogStats');
    if (!container) return;

    if (!hookStats || hookStats.totalExecutions === 0) {
        container.innerHTML = '';
        return;
    }

    const successRate = hookStats.successRate !== undefined ? hookStats.successRate.toFixed(1) : '0.0';
    const avgDuration = hookStats.avgDuration !== undefined ? formatDuration(hookStats.avgDuration) : '-';
    const failureStreak = hookStats.failureStreak || 0;

    container.innerHTML = `
        <div class="hook-log-stats-grid">
            <div class="hook-stat-card">
                <div class="hook-stat-value">${hookStats.totalExecutions}</div>
                <div class="hook-stat-label">${t('hookTotalExecutions') || '总执行次数'}</div>
            </div>
            <div class="hook-stat-card ${parseFloat(successRate) < 90 ? 'hook-stat-warning' : ''}">
                <div class="hook-stat-value">${successRate}%</div>
                <div class="hook-stat-label">${t('hookSuccessRate') || '成功率'}</div>
            </div>
            <div class="hook-stat-card">
                <div class="hook-stat-value">${avgDuration}</div>
                <div class="hook-stat-label">${t('hookAvgDuration') || '平均耗时'}</div>
            </div>
            <div class="hook-stat-card ${failureStreak > 0 ? 'hook-stat-danger' : ''}">
                <div class="hook-stat-value">${failureStreak}</div>
                <div class="hook-stat-label">${t('hookFailureStreak') || '连续失败'}</div>
            </div>
        </div>
    `;
}

// ============================================
// Rendering — 日志表格
// ============================================

/**
 * 渲染 Hook 执行日志表格
 */
function renderHookLogTable() {
    const container = document.getElementById('hookLogTable');
    if (!container) return;

    // 渲染过滤器
    renderHookLogFilter();

    if (!hookExecutions || hookExecutions.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <div class="empty-icon">📋</div>
                <div class="empty-text">${t('noHookLogs') || '暂无执行日志'}</div>
                <div class="empty-hint">${t('noHookLogsHint') || 'Hook 执行后会自动记录日志'}</div>
            </div>
        `;
        return;
    }

    let html = `
        <div class="hook-log-table-wrapper">
            <table class="hook-log-table">
                <thead>
                    <tr>
                        <th>${t('hookLogTime') || '时间'}</th>
                        <th>${t('hookLogType') || '类型'}</th>
                        <th>${t('hookLogMatcher') || '匹配器'}</th>
                        <th>${t('hookLogCommand') || '命令'}</th>
                        <th>${t('hookLogStatus') || '状态'}</th>
                        <th>${t('hookLogDuration') || '耗时'}</th>
                        <th>${t('hookLogSource') || '来源'}</th>
                    </tr>
                </thead>
                <tbody>
    `;

    for (const ex of hookExecutions) {
        const isFailure = ex.status === 'failure' || ex.status === 'timeout';
        const statusClass = isFailure ? 'hook-status-failure' : 'hook-status-success';
        const rowClass = isFailure ? 'hook-log-row-failure' : '';
        const statusText = getStatusText(ex.status);
        const typeBadgeClass = getTypeBadgeClass(ex.hookType);
        const duration = formatDuration(ex.duration);
        const commandTruncated = ex.command.length > 50 ? ex.command.substring(0, 50) + '...' : ex.command;
        const sourceLabel = ex.source === 'jsonl' ? 'JSONL' : 'Log';

        html += `
            <tr class="${rowClass}" title="${escapeHtmlAttr(ex.stderr || '')}">
                <td class="hook-log-time">${escapeHtml(ex.startTime)}</td>
                <td><span class="hook-type-badge ${typeBadgeClass}">${escapeHtml(ex.hookType)}</span></td>
                <td class="hook-log-matcher"><code>${escapeHtml(ex.matcher)}</code></td>
                <td class="hook-log-command" title="${escapeHtmlAttr(ex.command)}"><code>${escapeHtml(commandTruncated)}</code></td>
                <td><span class="hook-status-badge ${statusClass}">${statusText}</span></td>
                <td class="hook-log-duration">${duration}</td>
                <td class="hook-log-source"><span class="hook-source-tag">${sourceLabel}</span></td>
            </tr>
        `;
    }

    html += `
                </tbody>
            </table>
        </div>
    `;

    container.innerHTML = html;
}

/**
 * 渲染 Hook 类型过滤器
 */
function renderHookLogFilter() {
    const container = document.getElementById('hookLogFilter');
    if (!container) return;

    const hookTypes = ['', 'PreToolUse', 'PostToolUse', 'Stop', 'Notification'];
    const typeLabels = {
        '': t('hookLogAll') || '全部',
        'PreToolUse': 'PreToolUse',
        'PostToolUse': 'PostToolUse',
        'Stop': 'Stop',
        'Notification': 'Notification'
    };

    let html = '<div class="hook-log-filter-bar">';
    html += `<span class="hook-filter-label">${t('filter') || '过滤'}:</span>`;
    html += '<div class="hook-filter-options">';

    for (const ht of hookTypes) {
        const activeClass = hookFilterType === ht ? 'hook-filter-active' : '';
        html += `<button class="hook-filter-btn ${activeClass}" onclick="filterHookLogs('${ht}')">${typeLabels[ht]}</button>`;
    }

    html += '</div></div>';
    container.innerHTML = html;
}

// ============================================
// Alert Handling
// ============================================

/**
 * 处理 Hook 执行告警
 * @param {Object} alert - 告警事件数据
 */
function handleHookAlert(alert) {
    if (!alert) return;

    const message = (t('hookAlertMessage') || 'Hook 连续失败 %d 次')
        .replace('%d', alert.count);

    // 显示桌面通知
    showToast(`⚠️ ${t('hookAlertTitle') || 'Hook 执行告警'}: ${message}`, 'warning');

    // 刷新日志
    refreshHookLogs();
}

// ============================================
// Utility Functions
// ============================================

/**
 * 格式化耗时显示
 * @param {number} ms - 毫秒数
 * @returns {string}
 */
function formatDuration(ms) {
    if (ms === undefined || ms === null) return '-';
    if (ms < 1) return '<1ms';
    if (ms < 1000) return ms + 'ms';
    if (ms < 60000) return (ms / 1000).toFixed(1) + 's';
    return (ms / 60000).toFixed(1) + 'min';
}

/**
 * 获取状态显示文本
 * @param {string} status - 状态值
 * @returns {string}
 */
function getStatusText(status) {
    const map = {
        'start': t('hookLogStarted') || '执行中',
        'success': t('hookLogSuccess') || '成功',
        'failure': t('hookLogFailure') || '失败',
        'timeout': t('hookLogTimeout') || '超时'
    };
    return map[status] || status;
}

/**
 * 获取 Hook 类型的 CSS 类名
 * @param {string} hookType - Hook 类型
 * @returns {string}
 */
function getTypeBadgeClass(hookType) {
    const map = {
        'PreToolUse': 'hook-type-pre',
        'PostToolUse': 'hook-type-post',
        'Stop': 'hook-type-stop',
        'Notification': 'hook-type-notification'
    };
    return map[hookType] || '';
}
