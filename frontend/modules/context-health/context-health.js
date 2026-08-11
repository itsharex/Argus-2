// @ts-check
// Context Health Dashboard — 上下文健康仪表盘

let ctxGrowthChart = null;
let ctxScoreChart = null;

// ---- 加载上下文健康数据 ----

let ctxHealthLoading = false;
let ctxHealthLoaded = false;

async function loadContextHealth() {
    if (ctxHealthLoading) return;
    ctxHealthLoading = true;

    const skeleton = document.getElementById('ctxSkeleton');

    // 首次加载时显示骨架屏
    if (!ctxHealthLoaded && skeleton) {
        skeleton.classList.remove('hidden', 'fading-out');
    }

    try {
        const overview = await window.go.main.App.GetContextHealthOverview();
        if (!overview) return;

        console.log('[ContextHealth] overview:', JSON.stringify(overview, null, 2));

        renderContextOverviewCards(overview);
        renderContextGrowthChart(overview.topSessions);
        renderHealthScoreChart(overview);
        renderHealthSessionTable(overview.topSessions);

        // 同步等待效率分析加载完成，避免骨架屏消失后出现二次加载闪烁
        await loadEfficiencyAnalysis();

        ctxHealthLoaded = true;
    } catch (error) {
        console.error('Failed to load context health:', error);
    } finally {
        ctxHealthLoading = false;
        hideContextSkeleton(skeleton);
    }
}

/**
 * 隐藏骨架屏（带淡出动画）
 * @param {HTMLElement|null} skeleton
 */
function hideContextSkeleton(skeleton) {
    if (!skeleton || skeleton.classList.contains('hidden')) return;

    skeleton.classList.add('fading-out');

    // 监听 transition 结束
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

// ---- 概览卡片 ----

function renderContextOverviewCards(data) {
    document.getElementById('ctxAvgUsage').textContent = data.avgContextUsage.toFixed(1) + '%';
    document.getElementById('ctxMaxUsage').textContent = data.maxContextUsage.toFixed(1) + '%';
    document.getElementById('ctxAvgScore').textContent = Math.round(data.avgHealthScore) + '分';

    const alertCount = data.warningCount + data.criticalCount;
    const alertEl = document.getElementById('ctxAlertCount');
    alertEl.textContent = alertCount + '个';
    if (data.criticalCount > 0) {
        alertEl.style.color = 'var(--red)';
    } else if (data.warningCount > 0) {
        alertEl.style.color = 'var(--orange)';
    } else {
        alertEl.style.color = '';
    }
}

// ---- 采样数据点（避免图表过密）----

function sampleTurns(turns, maxPoints) {
    if (!turns || turns.length <= maxPoints) return turns;
    const step = Math.ceil(turns.length / maxPoints);
    const sampled = [];
    for (let i = 0; i < turns.length; i += step) {
        sampled.push(turns[i]);
    }
    // 确保最后一个点被包含
    const last = turns[turns.length - 1];
    if (sampled[sampled.length - 1] !== last) {
        sampled.push(last);
    }
    return sampled;
}

// ---- 上下文增长趋势图 (Chart.js) ----

function renderContextGrowthChart(sessions) {
    if (!sessions || sessions.length === 0) {
        console.warn('[ContextHealth] no sessions for growth chart');
        return;
    }

    const canvas = document.getElementById('ctxGrowthChart');
    if (!canvas) return;
    if (typeof Chart === 'undefined') {
        console.error('[ContextHealth] Chart.js is NOT loaded!');
        return;
    }

    if (ctxGrowthChart) {
        ctxGrowthChart.destroy();
        ctxGrowthChart = null;
    }

    const isDark = document.documentElement.getAttribute('data-theme') === 'dark';

    const colors = isDark ? {
        text: '#8e8e93',
        grid: 'rgba(255, 255, 255, 0.06)',
        line1: 'rgba(99, 179, 237, 1)',
        fill1: 'rgba(99, 179, 237, 0.12)',
        line2: 'rgba(72, 187, 120, 1)',
        fill2: 'rgba(72, 187, 120, 0.12)',
        line3: 'rgba(255, 149, 0, 1)',
        fill3: 'rgba(255, 149, 0, 0.12)',
        line4: 'rgba(175, 82, 222, 1)',
        fill4: 'rgba(175, 82, 222, 0.12)',
        line5: 'rgba(255, 59, 48, 1)',
        fill5: 'rgba(255, 59, 48, 0.12)',
    } : {
        text: '#86868b',
        grid: 'rgba(0, 0, 0, 0.04)',
        line1: 'rgba(0, 122, 255, 1)',
        fill1: 'rgba(0, 122, 255, 0.08)',
        line2: 'rgba(52, 199, 89, 1)',
        fill2: 'rgba(52, 199, 89, 0.08)',
        line3: 'rgba(255, 149, 0, 1)',
        fill3: 'rgba(255, 149, 0, 0.08)',
        line4: 'rgba(175, 82, 222, 1)',
        fill4: 'rgba(175, 82, 222, 0.08)',
        line5: 'rgba(255, 59, 48, 1)',
        fill5: 'rgba(255, 59, 48, 0.08)',
    };

    const lineColors = [colors.line1, colors.line2, colors.line3, colors.line4, colors.line5];
    const fillColors = [colors.fill1, colors.fill2, colors.fill3, colors.fill4, colors.fill5];

    // 取前 5 个会话，每个最多 80 个数据点
    const topSessions = sessions.slice(0, 5);
    const MAX_POINTS = 80;

    let maxTurns = 0;
    const datasets = topSessions.map((s, i) => {
        const sampled = sampleTurns(s.turns, MAX_POINTS);
        const dataPoints = sampled.map(t => t.inputTokens);
        if (sampled.length > maxTurns) maxTurns = sampled.length;
        return {
            label: s.sessionId.substring(0, 8) + ' (' + s.model + ')',
            data: dataPoints,
            borderColor: lineColors[i % lineColors.length],
            backgroundColor: fillColors[i % fillColors.length],
            fill: false,
            tension: 0.2,
            pointRadius: 2,
            pointHoverRadius: 5,
            borderWidth: 2,
        };
    });

    const labels = Array.from({ length: maxTurns }, (_, i) => {
        if (maxTurns <= 20) return 'Turn ' + (i + 1);
        // 稀疏标签
        if (i % Math.ceil(maxTurns / 15) === 0 || i === maxTurns - 1) return 'T' + (i + 1);
        return '';
    });

    // 200K 上下文限制线
    const limitLine = {
        label: '200K 上下文限制',
        data: Array(maxTurns).fill(200000),
        borderColor: 'rgba(255, 59, 48, 0.4)',
        borderDash: [6, 4],
        borderWidth: 1.5,
        pointRadius: 0,
        fill: false,
    };

    ctxGrowthChart = new Chart(canvas, {
        type: 'line',
        data: {
            labels: labels,
            datasets: [...datasets, limitLine],
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            interaction: {
                mode: 'index',
                intersect: false,
            },
            animation: {
                duration: 600,
                easing: 'easeOutQuart',
            },
            plugins: {
                legend: {
                    display: true,
                    position: 'top',
                    align: 'end',
                    labels: {
                        color: colors.text,
                        font: { size: 10, family: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Roboto, sans-serif' },
                        usePointStyle: true,
                        pointStyleWidth: 8,
                        boxHeight: 6,
                        padding: 12,
                    }
                },
                tooltip: {
                    backgroundColor: isDark ? 'rgba(30,30,30,0.95)' : 'rgba(255,255,255,0.95)',
                    titleColor: isDark ? '#fff' : '#1d1d1f',
                    bodyColor: isDark ? '#fff' : '#1d1d1f',
                    padding: { top: 10, bottom: 10, left: 14, right: 14 },
                    cornerRadius: 10,
                    borderColor: isDark ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.08)',
                    borderWidth: 1,
                    callbacks: {
                        label: function(ctx) {
                            return '  ' + ctx.dataset.label + ': ' + formatTokenCount(ctx.parsed.y);
                        }
                    }
                }
            },
            scales: {
                x: {
                    border: { display: false },
                    ticks: {
                        color: colors.text,
                        font: { size: 10 },
                        maxRotation: 0,
                        autoSkip: true,
                        maxTicksLimit: 15,
                        padding: 8,
                    },
                    grid: { display: false },
                },
                y: {
                    border: { display: false },
                    ticks: {
                        color: colors.text,
                        font: { size: 10 },
                        callback: v => formatTokenCount(v),
                        padding: 8,
                        maxTicksLimit: 6,
                    },
                    grid: { color: colors.grid, drawTicks: false },
                }
            }
        }
    });
}

// ---- 健康评分分布 (Chart.js 环形图) ----

function renderHealthScoreChart(data) {
    const canvas = document.getElementById('ctxScoreChart');
    if (!canvas) return;
    if (typeof Chart === 'undefined') return;

    if (ctxScoreChart) {
        ctxScoreChart.destroy();
        ctxScoreChart = null;
    }

    const isDark = document.documentElement.getAttribute('data-theme') === 'dark';

    // 统计各等级数量
    const counts = { excellent: 0, good: 0, warning: 0, critical: 0 };
    if (data.topSessions) {
        data.topSessions.forEach(s => {
            counts[s.healthLevel] = (counts[s.healthLevel] || 0) + 1;
        });
    }

    const total = counts.excellent + counts.good + counts.warning + counts.critical;
    if (total === 0) {
        const ctx = canvas.getContext('2d');
        ctx.clearRect(0, 0, canvas.width, canvas.height);
        ctx.fillStyle = isDark ? '#8e8e93' : '#86868b';
        ctx.font = '13px -apple-system, sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText(t('noData'), canvas.width / 2, canvas.height / 2);
        return;
    }

    ctxScoreChart = new Chart(canvas, {
        type: 'doughnut',
        data: {
            labels: [t('excellent'), t('good'), t('warning'), t('critical')],
            datasets: [{
                data: [counts.excellent, counts.good, counts.warning, counts.critical],
                backgroundColor: [
                    'rgba(52, 199, 89, 0.8)',   // green
                    'rgba(0, 122, 255, 0.8)',    // blue
                    'rgba(255, 149, 0, 0.8)',    // orange
                    'rgba(255, 59, 48, 0.8)',    // red
                ],
                borderColor: isDark ? '#1c1c1e' : '#ffffff',
                borderWidth: 2,
            }]
        },
        options: {
            responsive: true,
            maintainAspectRatio: false,
            cutout: '60%',
            plugins: {
                legend: {
                    position: 'bottom',
                    labels: {
                        color: isDark ? '#8e8e93' : '#86868b',
                        font: { size: 11, family: '-apple-system, sans-serif' },
                        padding: 12,
                        usePointStyle: true,
                        pointStyleWidth: 8,
                    }
                },
                tooltip: {
                    backgroundColor: isDark ? 'rgba(30,30,30,0.95)' : 'rgba(255,255,255,0.95)',
                    titleColor: isDark ? '#fff' : '#1d1d1f',
                    bodyColor: isDark ? '#fff' : '#1d1d1f',
                    padding: 10,
                    cornerRadius: 8,
                    callbacks: {
                        label: function(ctx) {
                            const pct = total > 0 ? (ctx.raw / total * 100).toFixed(1) : 0;
                            return ' ' + ctx.label + ': ' + ctx.raw + ' (' + pct + '%)';
                        }
                    }
                }
            }
        }
    });
}

// ---- 会话健康列表 ----

function renderHealthSessionTable(sessions) {
    const tbody = document.getElementById('ctxHealthTableBody');
    if (!sessions || sessions.length === 0) {
        tbody.innerHTML = '<tr><td colspan="6" class="empty-state-cell">' + t('noData') + '</td></tr>';
        return;
    }

    tbody.innerHTML = sessions.map(s => {
        const levelClass = 'ctx-level-' + s.healthLevel;
        const levelText = t(s.healthLevel);
        const usageBar = `<div class="ctx-usage-bar"><div class="ctx-usage-fill ${levelClass}" style="width:${Math.min(s.contextUsagePct, 100)}%"></div></div>`;

        // 压缩事件标记
        let compressionTag = '';
        if (s.compressionEvents > 5) {
            compressionTag = '<span class="ctx-compress-tag high">' + s.compressionEvents + '</span>';
        } else if (s.compressionEvents > 0) {
            compressionTag = '<span class="ctx-compress-tag">' + s.compressionEvents + '</span>';
        }

        return `<tr>
            <td class="session-id-cell" title="${escapeHtmlAttr(s.sessionId)}">${escapeHtml(s.sessionId.substring(0, 12))}…</td>
            <td>${escapeHtml(s.model)}</td>
            <td>${usageBar}<span class="ctx-usage-text">${s.contextUsagePct.toFixed(1)}%</span></td>
            <td><span class="ctx-score-badge ${levelClass}">${s.healthScore}</span></td>
            <td><span class="ctx-level-badge ${levelClass}">${levelText}</span></td>
            <td>${compressionTag}</td>
        </tr>`;
    }).join('');
}

// ============================================
// 效率分析
// ============================================

let efficiencyData = null;

/**
 * 加载并渲染效率分析数据
 */
async function loadEfficiencyAnalysis() {
    const container = document.getElementById('efficiencyContent');
    if (!container) return;

    try {
        efficiencyData = await window.go.main.App.GetGlobalEfficiencyReport();
        if (!efficiencyData) {
            container.innerHTML = `<div class="empty-state"><p>${t('noData') || '暂无数据'}</p></div>`;
            return;
        }
        renderEfficiencySection(container, efficiencyData);
    } catch (error) {
        console.error('Failed to load efficiency data:', error);
        container.innerHTML = `<div class="empty-state"><p>${t('loadEfficiencyFailed') || '加载效率数据失败'}</p></div>`;
    }
}

/**
 * 渲染效率分析完整区块
 * @param {HTMLElement} container
 * @param {Object} report
 */
function renderEfficiencySection(container, report) {
    let html = '';

    // 1. 效率评分卡片 + 缓存命中率环形图 + 开销分解
    html += '<div class="efficiency-grid">';

    // 效率评分
    html += `<div class="efficiency-score-card">
        <div class="efficiency-score-value">${report.efficiencyScore}</div>
        <div class="efficiency-score-label">${t('efficiencyScore') || '效率评分'}</div>
        <div class="efficiency-score-bar">
            <div class="efficiency-score-fill" style="width:${report.efficiencyScore}%;background:${getScoreColor(report.efficiencyScore)}"></div>
        </div>
    </div>`;

    // 缓存命中率环形图
    html += `<div class="efficiency-chart-box">
        <div class="efficiency-chart-title">${t('cacheHitRate') || '缓存命中率'}</div>
        <canvas id="cacheHitRateChart" width="180" height="180"></canvas>
    </div>`;

    // 上下文开销分解
    html += `<div class="efficiency-chart-box">
        <div class="efficiency-chart-title">${t('contextOverhead') || '上下文开销'}</div>
        <canvas id="contextOverheadChart" width="180" height="180"></canvas>
    </div>`;

    html += '</div>';

    // 2. Token 统计详情
    html += '<div class="efficiency-stats-row">';
    html += renderStatItem(t('cacheReadTokens') || '缓存读取', formatTokenCount(report.cacheReadTokens), 'var(--green)');
    html += renderStatItem(t('cacheWriteTokens') || '缓存写入', formatTokenCount(report.cacheCreationTokens), 'var(--orange)');
    html += renderStatItem(t('totalInputTokens') || '总输入', formatTokenCount(report.totalInputTokens), 'var(--blue)');
    html += renderStatItem(t('claudeMdTokens') || 'CLAUDE.md', formatTokenCount(report.claudeMdTokens), 'var(--purple)');
    html += renderStatItem(t('skillsCount') || 'Skills', String(report.skillsCount), 'var(--teal)');
    html += renderStatItem(t('mcpServersCount') || 'MCP 服务器', String(report.mcpServersCount), 'var(--pink)');
    html += '</div>';

    // 3. 未使用 MCP 工具
    if (report.unusedMcpTools && report.unusedMcpTools.length > 0) {
        html += '<div class="efficiency-section">';
        html += `<div class="efficiency-section-title">${t('unusedMcpTools') || '未使用的 MCP 工具'}</div>`;
        html += '<div class="unused-tools-list">';
        for (const tool of report.unusedMcpTools) {
            html += `<span class="unused-tool-tag">${escapeHtml(tool)}</span>`;
        }
        html += '</div></div>';
    }

    // 4. 优化建议
    if (report.recommendations && report.recommendations.length > 0) {
        html += '<div class="efficiency-section">';
        html += `<div class="efficiency-section-title">${t('optimizationTips') || '优化建议'}</div>`;
        html += '<div class="recommendations-list">';
        for (const rec of report.recommendations) {
            html += `<div class="recommendation-item">
                <span class="recommendation-icon">💡</span>
                <span class="recommendation-text">${escapeHtml(rec)}</span>
            </div>`;
        }
        html += '</div></div>';
    }

    container.innerHTML = html;

    // 渲染图表（DOM 就绪后）
    setTimeout(() => {
        renderCacheHitRateChart(report);
        renderContextOverheadChart(report);
    }, 50);
}

/**
 * 渲染缓存命中率环形图
 */
function renderCacheHitRateChart(report) {
    const canvas = document.getElementById('cacheHitRateChart');
    if (!canvas || typeof Chart === 'undefined') return;

    const rate = report.cacheHitRate || 0;
    const color = rate >= 70 ? 'rgba(52, 199, 89, 0.8)' :
                  rate >= 40 ? 'rgba(255, 149, 0, 0.8)' :
                  'rgba(255, 59, 48, 0.8)';
    const bgColor = rate >= 70 ? 'rgba(52, 199, 89, 0.15)' :
                     rate >= 40 ? 'rgba(255, 149, 0, 0.15)' :
                     'rgba(255, 59, 48, 0.15)';

    const isDark = document.documentElement.getAttribute('data-theme') === 'dark';

    new Chart(canvas, {
        type: 'doughnut',
        data: {
            datasets: [{
                data: [rate, 100 - rate],
                backgroundColor: [color, bgColor],
                borderColor: isDark ? '#1c1c1e' : '#ffffff',
                borderWidth: 2,
            }]
        },
        options: {
            responsive: false,
            cutout: '70%',
            plugins: {
                legend: { display: false },
                tooltip: { enabled: false },
            }
        },
        plugins: [{
            id: 'centerText',
            afterDraw: function(chart) {
                const ctx = chart.ctx;
                const centerX = chart.chartArea.left + (chart.chartArea.right - chart.chartArea.left) / 2;
                const centerY = chart.chartArea.top + (chart.chartArea.bottom - chart.chartArea.top) / 2;

                ctx.save();
                ctx.textAlign = 'center';
                ctx.textBaseline = 'middle';

                // 大数字
                ctx.font = 'bold 24px -apple-system, sans-serif';
                ctx.fillStyle = isDark ? '#ffffff' : '#1d1d1f';
                ctx.fillText(rate.toFixed(1) + '%', centerX, centerY - 6);

                // 小标签
                ctx.font = '10px -apple-system, sans-serif';
                ctx.fillStyle = isDark ? '#8e8e93' : '#86868b';
                ctx.fillText(t('cacheHitRate') || '缓存命中率', centerX, centerY + 14);

                ctx.restore();
            }
        }]
    });
}

/**
 * 渲染上下文开销分解环形图
 */
function renderContextOverheadChart(report) {
    const canvas = document.getElementById('contextOverheadChart');
    if (!canvas || typeof Chart === 'undefined') return;

    const data = [
        report.claudeMdTokens || 0,
        (report.skillsCount || 0) * 500, // 估算
        (report.mcpServersCount || 0) * 1000, // 估算
        Math.max(0, (report.totalInputTokens || 0) - (report.claudeMdTokens || 0) - (report.skillsCount || 0) * 500 - (report.mcpServersCount || 0) * 1000),
    ];

    const labels = [
        'CLAUDE.md',
        'Skills',
        'MCP',
        t('conversation') || '对话',
    ];

    const colors = [
        'rgba(175, 82, 222, 0.8)',  // purple
        'rgba(0, 199, 190, 0.8)',   // teal
        'rgba(255, 149, 0, 0.8)',   // orange
        'rgba(0, 122, 255, 0.8)',   // blue
    ];

    const isDark = document.documentElement.getAttribute('data-theme') === 'dark';

    // 过滤掉零值
    const filteredData = [];
    const filteredLabels = [];
    const filteredColors = [];
    for (let i = 0; i < data.length; i++) {
        if (data[i] > 0) {
            filteredData.push(data[i]);
            filteredLabels.push(labels[i]);
            filteredColors.push(colors[i]);
        }
    }

    if (filteredData.length === 0) {
        const ctx = canvas.getContext('2d');
        ctx.clearRect(0, 0, canvas.width, canvas.height);
        ctx.fillStyle = isDark ? '#8e8e93' : '#86868b';
        ctx.font = '12px -apple-system, sans-serif';
        ctx.textAlign = 'center';
        ctx.fillText(t('noData') || '暂无数据', canvas.width / 2, canvas.height / 2);
        return;
    }

    new Chart(canvas, {
        type: 'doughnut',
        data: {
            labels: filteredLabels,
            datasets: [{
                data: filteredData,
                backgroundColor: filteredColors,
                borderColor: isDark ? '#1c1c1e' : '#ffffff',
                borderWidth: 2,
            }]
        },
        options: {
            responsive: false,
            cutout: '55%',
            plugins: {
                legend: {
                    position: 'bottom',
                    labels: {
                        color: isDark ? '#8e8e93' : '#86868b',
                        font: { size: 10, family: '-apple-system, sans-serif' },
                        padding: 8,
                        usePointStyle: true,
                        pointStyleWidth: 8,
                        boxHeight: 6,
                    }
                },
                tooltip: {
                    callbacks: {
                        label: function(ctx) {
                            const total = ctx.dataset.data.reduce((a, b) => a + b, 0);
                            const pct = total > 0 ? (ctx.raw / total * 100).toFixed(1) : 0;
                            return ' ' + ctx.label + ': ' + formatTokenCount(ctx.raw) + ' (' + pct + '%)';
                        }
                    }
                }
            }
        }
    });
}

/**
 * 渲染统计项
 */
function renderStatItem(label, value, color) {
    return `<div class="efficiency-stat-item">
        <div class="efficiency-stat-dot" style="background:${color}"></div>
        <div class="efficiency-stat-info">
            <div class="efficiency-stat-value">${value}</div>
            <div class="efficiency-stat-label">${label}</div>
        </div>
    </div>`;
}

/**
 * 获取评分颜色
 */
function getScoreColor(score) {
    if (score >= 80) return 'var(--green)';
    if (score >= 60) return 'var(--blue)';
    if (score >= 40) return 'var(--orange)';
    return 'var(--red)';
}
