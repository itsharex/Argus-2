// @ts-check
// Dashboard — Token Analytics page logic

let trendChart = null;

// ---- 子窗口 Tab 切换 ----

function switchDashboardSubtab(tabId) {
    // 更新按钮状态
    document.querySelectorAll('.dashboard-subtab').forEach(btn => {
        btn.classList.toggle('active', btn.dataset.subtab === tabId);
    });
    // 更新面板显示
    document.querySelectorAll('.dashboard-subtab-panel').forEach(panel => {
        panel.classList.toggle('active', panel.id === 'panel-' + tabId);
    });
    // 按需加载子窗口数据
    if (tabId === 'context-health' && typeof loadContextHealth === 'function') {
        loadContextHealth();
    }
}

// Load and render the full dashboard
async function loadDashboard() {
    try {
        const overview = await window.go.main.App.GetTokenOverview();
        if (!overview) return;

        // Debug: log what the backend returns
        console.log('[Dashboard] overview:', JSON.stringify({
            totalTokens: overview.totalTokens,
            totalSessions: overview.totalSessions,
            todayTokens: overview.todayTokens,
            dailyTrendLen: overview.dailyTrend ? overview.dailyTrend.length : 0,
            projectBreakdownLen: overview.projectBreakdown ? overview.projectBreakdown.length : 0,
            modelBreakdownLen: overview.modelBreakdown ? overview.modelBreakdown.length : 0,
            sampleProject: overview.projectBreakdown && overview.projectBreakdown[0],
            sampleModel: overview.modelBreakdown && overview.modelBreakdown[0],
        }, null, 2));

        renderOverviewCards(overview);
        renderTrendChart(overview.dailyTrend);
        renderProjectTable(overview.projectBreakdown);
        renderModelTable(overview.modelBreakdown);
    } catch (error) {
        console.error('Failed to load dashboard:', error);
    }
}

// ---- Overview Cards ----

function renderOverviewCards(data) {
    document.getElementById('todayTokens').textContent = formatTokenCount(data.todayTokens);
    document.getElementById('thisMonthTokens').textContent = formatTokenCount(data.thisMonthTokens);
    document.getElementById('lastMonthTokens').textContent = formatTokenCount(data.lastMonthTokens);
    document.getElementById('totalSessions').textContent = formatNumber(data.totalSessions);
    document.getElementById('totalTokens').textContent = formatTokenCount(data.totalTokens);

    // Month-over-month change
    const changeEl = document.getElementById('monthChange');
    if (data.monthTokenChange !== undefined && data.monthTokenChange !== null && data.lastMonthTokens > 0) {
        const pct = data.monthTokenChange;
        const sign = pct >= 0 ? '+' : '';
        changeEl.textContent = `${sign}${pct.toFixed(1)}% ${t('vsLastMonth')}`;
        changeEl.className = 'card-change ' + (pct >= 0 ? 'up' : 'down');
    } else {
        changeEl.textContent = '';
    }
}

// ---- Trend Chart (Chart.js) - Apple/Google 风格 ----

function renderTrendChart(dailyTrend) {
    if (!dailyTrend || dailyTrend.length === 0) {
        console.warn('[Dashboard] dailyTrend is empty or null');
        return;
    }

    const canvas = document.getElementById('trendChart');
    if (!canvas) {
        console.warn('[Dashboard] trendChart canvas not found');
        return;
    }

    if (typeof Chart === 'undefined') {
        console.error('[Dashboard] Chart.js is NOT loaded!');
        return;
    }

    // Destroy previous chart instance
    if (trendChart) {
        trendChart.destroy();
        trendChart = null;
    }

    const labels = dailyTrend.map(d => {
        const parts = d.date.split('-');
        return parts[1] + '/' + parts[2]; // "07/01"
    });
    const inputTokens = dailyTrend.map(d => d.inputTokens || 0);
    const outputTokens = dailyTrend.map(d => d.outputTokens || 0);

    const isDark = document.documentElement.getAttribute('data-theme') === 'dark';

    // Apple/Google 风格配色
    const colors = isDark ? {
        text: '#8e8e93',
        textLight: '#636366',
        grid: 'rgba(255, 255, 255, 0.06)',
        input: 'rgba(99, 179, 237, 0.85)',
        inputBorder: 'rgba(99, 179, 237, 1)',
        output: 'rgba(72, 187, 120, 0.85)',
        outputBorder: 'rgba(72, 187, 120, 1)',
        tooltipBg: 'rgba(30, 30, 30, 0.95)',
        tooltipText: '#ffffff',
    } : {
        text: '#86868b',
        textLight: '#aeaeb2',
        grid: 'rgba(0, 0, 0, 0.04)',
        input: 'rgba(0, 122, 255, 0.75)',
        inputBorder: 'rgba(0, 122, 255, 1)',
        output: 'rgba(52, 199, 89, 0.75)',
        outputBorder: 'rgba(52, 199, 89, 1)',
        tooltipBg: 'rgba(255, 255, 255, 0.95)',
        tooltipText: '#1d1d1f',
    };

    trendChart = new Chart(canvas, {
        type: 'bar',
        data: {
            labels: labels,
            datasets: [{
                label: t('tokenIn'),
                data: inputTokens,
                backgroundColor: colors.input,
                borderColor: colors.inputBorder,
                borderWidth: 0,
                yAxisID: 'y',
            }, {
                label: t('tokenOut'),
                data: outputTokens,
                backgroundColor: colors.output,
                borderColor: colors.outputBorder,
                borderWidth: 0,
                yAxisID: 'y',
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
                        font: {
                            size: 11,
                            weight: '500',
                            family: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Roboto, sans-serif'
                        },
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
                    titleFont: {
                        size: 12,
                        weight: '600',
                        family: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Roboto, sans-serif'
                    },
                    bodyFont: {
                        size: 12,
                        family: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Roboto, sans-serif'
                    },
                    padding: { top: 10, bottom: 10, left: 14, right: 14 },
                    cornerRadius: 10,
                    displayColors: true,
                    boxWidth: 8,
                    boxHeight: 8,
                    boxPadding: 4,
                    usePointStyle: false,
                    borderColor: isDark ? 'rgba(255,255,255,0.1)' : 'rgba(0,0,0,0.08)',
                    borderWidth: 1,
                    callbacks: {
                        label: function(ctx) {
                            return `  ${ctx.dataset.label}: ${formatTokenCount(ctx.parsed.y)}`;
                        }
                    }
                }
            },
            scales: {
                x: {
                    stacked: true,
                    border: {
                        display: false,
                    },
                    ticks: {
                        color: colors.text,
                        font: {
                            size: 10,
                            family: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Roboto, sans-serif'
                        },
                        maxRotation: 0,
                        autoSkip: true,
                        maxTicksLimit: 10,
                        padding: 8,
                    },
                    grid: {
                        display: false,
                    }
                },
                y: {
                    stacked: true,
                    border: {
                        display: false,
                    },
                    ticks: {
                        color: colors.text,
                        font: {
                            size: 10,
                            family: '-apple-system, BlinkMacSystemFont, "SF Pro Text", "Segoe UI", Roboto, sans-serif'
                        },
                        callback: v => formatTokenCount(v),
                        padding: 8,
                        maxTicksLimit: 6,
                    },
                    grid: {
                        color: colors.grid,
                        drawTicks: false,
                    },
                    title: {
                        display: false,
                    }
                }
            }
        }
    });
}

// ---- Project Breakdown Table ----

function renderProjectTable(projects) {
    const tbody = document.getElementById('projectTableBody');
    console.log('[Dashboard] projectBreakdown:', projects);
    if (!projects || projects.length === 0) {
        tbody.innerHTML = `<tr><td colspan="5" class="empty-state-cell">${t('noData')}</td></tr>`;
        return;
    }

    tbody.innerHTML = projects.map(p => `
        <tr>
            <td class="project-name-cell" title="${escapeHtmlAttr(p.projectDir)}">${escapeHtml(p.projectName)}</td>
            <td>${p.sessionCount}</td>
            <td>${formatTokenCount(p.inputTokens)}</td>
            <td>${formatTokenCount(p.outputTokens)}</td>
            <td class="token-cell">${formatTokenCount(p.totalTokens)}</td>
        </tr>
    `).join('');
}

// ---- Model Breakdown Table ----

function renderModelTable(models) {
    const tbody = document.getElementById('modelTableBody');
    console.log('[Dashboard] modelBreakdown:', models);
    if (!models || models.length === 0) {
        tbody.innerHTML = `<tr><td colspan="5" class="empty-state-cell">${t('noData')}</td></tr>`;
        return;
    }

    tbody.innerHTML = models.map(m => `
        <tr>
            <td class="model-name-cell">${escapeHtml(m.model)}</td>
            <td>${m.sessionCount}</td>
            <td>${formatTokenCount(m.inputTokens)}</td>
            <td>${formatTokenCount(m.outputTokens)}</td>
            <td class="token-cell">${formatTokenCount(m.totalTokens)}</td>
        </tr>
    `).join('');
}

// ---- Formatting Helpers ----

function formatTokenCount(tokens) {
    if (tokens === undefined || tokens === null) return '0';
    if (tokens >= 1_000_000) {
        return (tokens / 1_000_000).toFixed(1) + 'M';
    }
    if (tokens >= 1_000) {
        return (tokens / 1_000).toFixed(1) + 'K';
    }
    return tokens.toString();
}
