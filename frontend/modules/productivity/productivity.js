// @ts-check
// Productivity Dashboard — 开发者生产力仪表盘

let productivityTrendChart = null;
let productivityLoaded = false;

// ============================================
// Load & Render
// ============================================

/**
 * 加载并渲染生产力仪表盘
 */
async function loadProductivity() {
    if (productivityLoaded) return;

    const container = document.getElementById('prodContent');
    if (!container) return;

    const skeleton = document.getElementById('prodSkeleton');

    // 首次加载时显示骨架屏
    if (skeleton) {
        skeleton.classList.remove('hidden', 'fading-out');
    }

    try {
        const [report, trend] = await Promise.all([
            window.go.main.App.GetProductivityReport(30),
            window.go.main.App.GetProductivityTrend(12),
        ]);

        if (!report) {
            container.innerHTML = `
                <div class="productivity-empty">
                    <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                        <path d="M12 20V10"/>
                        <path d="M18 20V4"/>
                        <path d="M6 20v-4"/>
                    </svg>
                    <p>${t('noProductivityData') || '暂无生产力数据'}</p>
                </div>
            `;
            hideProductivitySkeleton(skeleton);
            return;
        }

        renderProductivityPanel(container, report, trend || []);
        productivityLoaded = true;
    } catch (error) {
        console.error('Failed to load productivity:', error);
        container.innerHTML = `
            <div class="productivity-empty">
                <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <circle cx="12" cy="12" r="10"/>
                    <line x1="15" y1="9" x2="9" y2="15"/>
                    <line x1="9" y1="9" x2="15" y2="15"/>
                </svg>
                <p>${t('loadFailed') || '加载失败'}: ${escapeHtml(String(error))}</p>
            </div>
        `;
    } finally {
        hideProductivitySkeleton(skeleton);
    }
}

/**
 * 隐藏生产力骨架屏（带淡出动画）
 * @param {HTMLElement|null} skeleton
 */
function hideProductivitySkeleton(skeleton) {
    if (!skeleton || skeleton.classList.contains('hidden')) return;

    skeleton.classList.add('fading-out');

    const onTransitionEnd = function() {
        skeleton.removeEventListener('transitionend', onTransitionEnd);
        skeleton.classList.add('hidden');
        skeleton.classList.remove('fading-out');
    };
    skeleton.addEventListener('transitionend', onTransitionEnd);

    // 兜底：500ms 后强制隐藏
    setTimeout(() => {
        if (!skeleton.classList.contains('hidden')) {
            skeleton.classList.add('hidden');
            skeleton.classList.remove('fading-out');
        }
    }, 500);
}

/**
 * 渲染生产力面板
 * @param {HTMLElement} container
 * @param {Object} report
 * @param {Array} trend
 */
function renderProductivityPanel(container, report, trend) {
    let html = '';

    // 1. KPI 卡片
    html += renderKPICards(report);

    // 2. 图表区域（周趋势 + 工具调用分布）
    html += '<div class="productivity-charts-row">';
    html += `<div class="productivity-chart-box">
        <div class="productivity-chart-title">${t('weeklyTrend') || '周趋势'}</div>
        <div class="productivity-chart-container"><canvas id="productivityTrendChart"></canvas></div>
    </div>`;
    html += `<div class="productivity-chart-box">
        <div class="productivity-chart-title">${t('toolCallDistribution') || '工具调用分布'}</div>
        <div class="productivity-chart-container"><canvas id="toolDistChart"></canvas></div>
    </div>`;
    html += '</div>';

    // 3. 周趋势表格
    html += renderTrendTable(trend);

    container.innerHTML = html;

    // 渲染图表（需要 DOM 已插入）
    requestAnimationFrame(() => {
        renderProductivityTrendChart(trend);
        renderToolDistChart(report);
    });
}

// ============================================
// KPI Cards
// ============================================

function renderKPICards(report) {
    const cards = [
        { label: t('prodSessionsTotal') || '总会话数', value: report.sessionsTotal, unit: '' },
        { label: t('prodSessionsPerDay') || '日均会话', value: report.sessionsPerDay.toFixed(1), unit: t('sessionsPerDay') || '次/天' },
        { label: t('prodAvgDuration') || '平均时长', value: formatDuration(report.avgSessionDurationMs), unit: '' },
        { label: t('prodAvgFiles') || '平均文件改动', value: report.avgFilesPerSession.toFixed(1), unit: t('filesPerSession') || '个/会话' },
        { label: t('prodAvgActions') || '平均操作数', value: report.avgActionsPerSession.toFixed(1), unit: t('actionsPerSession') || '次/会话' },
        { label: t('prodTotalToolCalls') || '工具调用总数', value: formatNumber(report.totalToolCalls), unit: '' },
    ];

    return '<div class="productivity-kpi-grid">' +
        cards.map(c => `
            <div class="productivity-kpi-card">
                <div class="productivity-kpi-label">${c.label}</div>
                <div class="productivity-kpi-value">${c.value}</div>
                ${c.unit ? `<div class="productivity-kpi-unit">${c.unit}</div>` : ''}
            </div>
        `).join('') +
        '</div>';
}

// ============================================
// Trend Chart (Chart.js)
// ============================================

function renderProductivityTrendChart(trend) {
    const canvas = document.getElementById('productivityTrendChart');
    if (!canvas || !trend || trend.length === 0 || typeof Chart === 'undefined') return;

    if (productivityTrendChart) {
        productivityTrendChart.destroy();
        productivityTrendChart = null;
    }

    const labels = trend.map(w => w.week);
    const sessions = trend.map(w => w.sessions || 0);
    const filesChanged = trend.map(w => w.filesChanged || 0);

    const isDark = document.documentElement.getAttribute('data-theme') === 'dark';
    const colors = isDark ? {
        text: '#8e8e93',
        grid: 'rgba(255, 255, 255, 0.06)',
        sessions: 'rgba(99, 179, 237, 0.85)',
        sessionsBorder: 'rgba(99, 179, 237, 1)',
        files: 'rgba(72, 187, 120, 0.85)',
        filesBorder: 'rgba(72, 187, 120, 1)',
        tooltipBg: 'rgba(30, 30, 30, 0.95)',
        tooltipText: '#ffffff',
    } : {
        text: '#86868b',
        grid: 'rgba(0, 0, 0, 0.04)',
        sessions: 'rgba(0, 122, 255, 0.75)',
        sessionsBorder: 'rgba(0, 122, 255, 1)',
        files: 'rgba(52, 199, 89, 0.75)',
        filesBorder: 'rgba(52, 199, 89, 1)',
        tooltipBg: 'rgba(255, 255, 255, 0.95)',
        tooltipText: '#1d1d1f',
    };

    const fontFamily = '-apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Roboto, sans-serif';

    productivityTrendChart = new Chart(canvas, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [{
                label: t('prodSessionsTotal') || '会话数',
                data: sessions,
                borderColor: colors.sessionsBorder,
                backgroundColor: colors.sessions,
                fill: false,
                tension: 0.4,
                pointRadius: 4,
                pointHoverRadius: 6,
                yAxisID: 'y',
            }, {
                label: t('prodTotalFiles') || '文件改动',
                data: filesChanged,
                borderColor: colors.filesBorder,
                backgroundColor: colors.files,
                fill: false,
                tension: 0.4,
                pointRadius: 4,
                pointHoverRadius: 6,
                yAxisID: 'y1',
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            interaction: {
                mode: 'index',
                intersect: false,
            },
            animation: {
                duration: 800,
                easing: 'easeOutQuart',
            },
            plugins: {
                legend: {
                    display: true,
                    position: 'top',
                    align: 'end',
                    labels: {
                        color: colors.text,
                        font: { size: 11, weight: '500', family: fontFamily },
                        usePointStyle: true,
                        pointStyleWidth: 8,
                        boxHeight: 6,
                        padding: 16,
                    }
                },
                tooltip: {
                    backgroundColor: colors.tooltipBg,
                    titleColor: colors.tooltipText,
                    bodyColor: colors.tooltipText,
                    titleFont: { size: 12, weight: '600', family: fontFamily },
                    bodyFont: { size: 12, family: fontFamily },
                    padding: { top: 10, bottom: 10, left: 14, right: 14 },
                    cornerRadius: 10,
                    borderColor: isDark ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.08)',
                    borderWidth: 1,
                }
            },
            scales: {
                x: {
                    border: { display: false },
                    ticks: {
                        color: colors.text,
                        font: { size: 10, family: fontFamily },
                        maxRotation: 0,
                        autoSkip: true,
                        maxTicksLimit: 8,
                        padding: 8,
                    },
                    grid: { display: false },
                },
                y: {
                    type: 'linear',
                    display: true,
                    position: 'left',
                    border: { display: false },
                    ticks: {
                        color: colors.text,
                        font: { size: 10, family: fontFamily },
                        padding: 8,
                        maxTicksLimit: 5,
                    },
                    grid: { color: colors.grid, drawTicks: false },
                    title: {
                        display: true,
                        text: t('prodSessionsTotal') || '会话数',
                        color: colors.text,
                        font: { size: 10, family: fontFamily },
                    },
                },
                y1: {
                    type: 'linear',
                    display: true,
                    position: 'right',
                    border: { display: false },
                    ticks: {
                        color: colors.text,
                        font: { size: 10, family: fontFamily },
                        padding: 8,
                        maxTicksLimit: 5,
                    },
                    grid: { drawOnChartArea: false },
                    title: {
                        display: true,
                        text: t('prodTotalFiles') || '文件改动',
                        color: colors.text,
                        font: { size: 10, family: fontFamily },
                    },
                },
            }
        }
    });
}

// ============================================
// Tool Distribution Chart (Doughnut)
// ============================================

function renderToolDistChart(report) {
    const canvas = document.getElementById('toolDistChart');
    if (!canvas || typeof Chart === 'undefined') return;

    const data = [
        { label: t('toolCalls') || '工具调用', value: report.totalToolCalls || 0 },
        { label: t('prodTotalFiles') || '文件改动', value: report.totalFilesChanged || 0 },
        { label: t('prodTotalActions') || '操作总数', value: report.totalActions || 0 },
    ];

    const isDark = document.documentElement.getAttribute('data-theme') === 'dark';
    const bgColors = [
        'rgba(99, 179, 237, 0.75)',
        'rgba(72, 187, 120, 0.75)',
        'rgba(245, 158, 11, 0.75)',
    ];

    new Chart(canvas, {
        type: 'doughnut',
        data: {
            labels: data.map(d => d.label),
            datasets: [{
                data: data.map(d => d.value),
                backgroundColor: bgColors,
                borderColor: isDark ? 'rgba(30,30,30,0.8)' : 'rgba(255,255,255,0.8)',
                borderWidth: 2,
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            animation: {
                duration: 800,
                easing: 'easeOutQuart',
            },
            plugins: {
                legend: {
                    display: true,
                    position: 'bottom',
                    labels: {
                        color: isDark ? '#8e8e93' : '#86868b',
                        font: { size: 11, family: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Roboto, sans-serif' },
                        usePointStyle: true,
                        pointStyleWidth: 8,
                        boxHeight: 6,
                        padding: 16,
                    }
                },
                tooltip: {
                    backgroundColor: isDark ? 'rgba(30, 30, 30, 0.95)' : 'rgba(255, 255, 255, 0.95)',
                    titleColor: isDark ? '#ffffff' : '#1d1d1f',
                    bodyColor: isDark ? '#ffffff' : '#1d1d1f',
                    padding: { top: 10, bottom: 10, left: 14, right: 14 },
                    cornerRadius: 10,
                    borderColor: isDark ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.08)',
                    borderWidth: 1,
                }
            },
            cutout: '60%',
        }
    });
}

// ============================================
// Trend Table
// ============================================

function renderTrendTable(trend) {
    if (!trend || trend.length === 0) {
        return `<div class="productivity-trend-table-wrapper">
            <div class="productivity-trend-table-title">${t('weeklyTrend') || '周趋势'}</div>
            <div style="padding: 20px; text-align: center; color: var(--text-secondary); font-size: 13px;">${t('noData') || '暂无数据'}</div>
        </div>`;
    }

    let html = `<div class="productivity-trend-table-wrapper">
        <div class="productivity-trend-table-title">${t('weeklyTrend') || '周趋势'}</div>
        <table class="productivity-trend-table">
            <thead>
                <tr>
                    <th>${t('week') || '周'}</th>
                    <th>${t('prodSessionsTotal') || '会话数'}</th>
                    <th>${t('prodTotalFiles') || '文件改动'}</th>
                    <th>${t('tokenUsage') || 'Token 使用'}</th>
                    <th>${t('prodAvgDuration') || '平均时长'}</th>
                </tr>
            </thead>
            <tbody>`;

    // 倒序显示（最近的周在前）
    for (let i = trend.length - 1; i >= 0; i--) {
        const w = trend[i];
        html += `<tr>
            <td>${escapeHtml(w.week)}</td>
            <td>${w.sessions || 0}</td>
            <td>${w.filesChanged || 0}</td>
            <td>${formatTokenCount(w.tokensUsed || 0)}</td>
            <td>${formatDuration(w.avgDurationMs || 0)}</td>
        </tr>`;
    }

    html += '</tbody></table></div>';
    return html;
}

// ============================================
// Helpers
// ============================================

/**
 * 格式化时长（毫秒 → 人类可读）
 * @param {number} ms - 毫秒
 * @returns {string}
 */
function formatDuration(ms) {
    if (!ms || ms <= 0) return '0s';
    if (ms < 1000) return ms + 'ms';
    if (ms < 60000) return (ms / 1000).toFixed(1) + 's';
    const minutes = Math.floor(ms / 60000);
    const seconds = Math.floor((ms % 60000) / 1000);
    if (minutes < 60) return `${minutes}m ${seconds}s`;
    const hours = Math.floor(minutes / 60);
    const remainMinutes = minutes % 60;
    return `${hours}h ${remainMinutes}m`;
}

/**
 * 格式化数字
 * @param {number} n
 * @returns {string}
 */
function formatNumber(n) {
    if (n === undefined || n === null) return '0';
    if (n >= 1000000) return (n / 1000000).toFixed(1) + 'M';
    if (n >= 1000) return (n / 1000).toFixed(1) + 'K';
    return String(n);
}

/**
 * 格式化 token 数量
 * @param {number} tokens
 * @returns {string}
 */
function formatTokenCount(tokens) {
    if (!tokens) return '0';
    if (tokens >= 1000000) return (tokens / 1000000).toFixed(1) + 'M';
    if (tokens >= 1000) return (tokens / 1000).toFixed(1) + 'K';
    return String(tokens);
}

/**
 * 转义 HTML
 * @param {string} str
 * @returns {string}
 */
function escapeHtml(str) {
    if (!str) return '';
    return str
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}
