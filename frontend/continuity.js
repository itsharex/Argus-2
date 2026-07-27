// @ts-check

// ============================================
// 会话连续性面板
// ============================================

let continuityProjects = [];
let currentContinuityProject = '';
let continuitySummary = null;

// 初始化连续性面板
async function initContinuity() {
    await loadContinuityProjects();
}

// 加载项目列表
async function loadContinuityProjects() {
    try {
        continuityProjects = await window.go.main.App.GetContinuityProjects();
        renderProjectSelect();
    } catch (error) {
        console.error('加载项目列表失败:', error);
    }
}

// 渲染项目选择下拉框
function renderProjectSelect() {
    const select = document.getElementById('continuityProjectSelect');
    if (!select) return;

    select.innerHTML = '';

    if (continuityProjects.length === 0) {
        select.innerHTML = '<option value="">' + t('noSessions') + '</option>';
        return;
    }

    continuityProjects.forEach(project => {
        const option = document.createElement('option');
        option.value = project.dirName;
        option.textContent = project.name + ' (' + project.sessionCount + ' sessions)';
        select.appendChild(option);
    });

    // 默认选中第一个
    if (continuityProjects.length > 0) {
        currentContinuityProject = continuityProjects[0].dirName;
        select.value = currentContinuityProject;

        // 设置分析会话数上限
        const sessionCountInput = document.getElementById('continuitySessionCount');
        if (sessionCountInput) {
            sessionCountInput.max = continuityProjects[0].sessionCount;
        }
    }
}

// 项目选择变化
function onContinuityProjectChange(value) {
    currentContinuityProject = value;

    // 更新分析会话数上限
    const project = continuityProjects.find(p => p.dirName === value);
    if (project) {
        const sessionCountInput = document.getElementById('continuitySessionCount');
        if (sessionCountInput) {
            sessionCountInput.max = project.sessionCount;
            // 如果当前值超过上限，自动调整
            if (parseInt(sessionCountInput.value) > project.sessionCount) {
                sessionCountInput.value = project.sessionCount;
            }
        }
    }
}

// 生成交接摘要
async function generateContinuityHandoff() {
    if (!currentContinuityProject) {
        showContinuityToast(t('selectProjectFirst') || '请先选择项目', 'error');
        return;
    }

    // 检查 LLM 是否已启用
    try {
        const llmEnabled = await window.go.main.App.IsContinuityLLMEnabled();
        if (!llmEnabled) {
            showContinuityToast(t('llmRequired') || '此功能需要接入 LLM，请在设置中配置', 'error');
            return;
        }
    } catch (err) {
        console.error('检查 LLM 状态失败:', err);
    }

    const sessionCountInput = document.getElementById('continuitySessionCount');
    const sessionCount = sessionCountInput ? parseInt(sessionCountInput.value) || 10 : 10;

    // 显示加载状态
    showContinuityLoading(true);

    try {
        continuitySummary = await window.go.main.App.GenerateContinuityHandoff(currentContinuityProject, sessionCount);
        stopProgressAnimation();
        renderContinuitySummary();
    } catch (error) {
        console.error('生成交接摘要失败:', error);
        stopProgressAnimation();
        showContinuityToast(t('generateFailed') || '生成失败: ' + error, 'error');
    } finally {
        showContinuityLoading(false);
    }
}

// 渲染交接摘要
function renderContinuitySummary() {
    const content = document.getElementById('continuityContent');
    if (!content || !continuitySummary) return;

    const s = continuitySummary;

    let html = '';

    // 头部信息
    html += '<div class="handoff-header">';
    html += '<h2>' + t('handoffTitle') + ' - ' + s.project + '</h2>';
    html += '<div class="handoff-meta">';
    html += '<span class="meta-item"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/><circle cx="9" cy="7" r="4"/></svg> ' + s.sessionsUsed + ' / ' + s.sessionsTotal + ' sessions</span>';
    html += '<span class="meta-item"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg> ' + formatTime(s.generatedAt) + '</span>';
    if (s.llmEnhanced) {
        html += '<span class="meta-item llm-badge" style="color: var(--green, #22c55e); font-weight: 600;">LLM Enhanced</span>';
    }
    html += '</div>';
    html += '</div>';

    // 统计卡片
    html += '<div class="handoff-stats">';
    html += '<div class="stat-card"><div class="stat-value">' + s.completedTasks.length + '</div><div class="stat-label">' + (t('completed') || '已完成') + '</div></div>';
    html += '<div class="stat-card"><div class="stat-value">' + s.pendingTasks.length + '</div><div class="stat-label">' + (t('pending') || '待办') + '</div></div>';
    html += '<div class="stat-card"><div class="stat-value">' + s.keyDecisions.length + '</div><div class="stat-label">' + (t('decisions') || '决策') + '</div></div>';
    html += '<div class="stat-card"><div class="stat-value">' + s.modifiedFiles.length + '</div><div class="stat-label">' + (t('files') || '文件') + '</div></div>';
    html += '</div>';

    // 质量评分
    if (s.quality) {
        html += '<div class="handoff-quality">';
        html += '<h3><span class="section-icon info"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg></span> ' + (t('qualityScore') || '质量评分') + '</h3>';
        html += '<div class="quality-grid">';
        html += '<div class="quality-item"><span class="quality-label">' + (t('completeness') || '完整性') + '</span><span class="quality-value">' + Math.round(s.quality.completeness * 100) + '%</span></div>';
        html += '<div class="quality-item"><span class="quality-label">' + (t('accuracy') || '准确性') + '</span><span class="quality-value">' + Math.round(s.quality.accuracy * 100) + '%</span></div>';
        html += '<div class="quality-item"><span class="quality-label">' + (t('freshness') || '时效性') + '</span><span class="quality-value">' + Math.round(s.quality.freshness * 100) + '%</span></div>';
        html += '<div class="quality-item"><span class="quality-label">' + (t('overallScore') || '综合评分') + '</span><span class="quality-value overall">' + Math.round(s.quality.overallScore * 100) + '%</span></div>';
        html += '</div>';
        html += '</div>';
    }

    // 叙事摘要（LLM 生成或关键词提取的核心内容）
    if (s.summary) {
        html += '<div class="handoff-section">';
        html += '<h3><span class="section-icon info"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><path d="M12 16v-4"/><path d="M12 8h.01"/></svg></span> ' + (t('coreSummary') || '核心摘要') + '</h3>';
        html += '<div class="summary-narrative">' + escapeHtml(s.summary).replace(/\n/g, '<br>') + '</div>';
        html += '</div>';
    }

    // 已完成任务
    if (s.completedTasks.length > 0) {
        html += '<div class="handoff-section">';
        html += '<h3><span class="section-icon success"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M22 11.08V12a10 10 0 1 1-5.93-9.14"/><polyline points="22 4 12 14.01 9 11.01"/></svg></span> ' + (t('completedTasks') || '已完成任务') + ' <span class="badge">' + s.completedTasks.length + '</span></h3>';
        html += '<div class="task-list">';
        s.completedTasks.forEach(task => {
            const verifiedClass = task.verifiedByGit ? ' verified' : '';
            html += '<div class="task-item' + verifiedClass + '">';
            html += '<div class="task-desc">' + escapeHtml(task.description) + '</div>';
            html += '<div class="task-meta">';
            html += '<span class="meta-tag">' + task.sessionId.substring(0, 8) + '</span>';
            html += '<span class="meta-tag ' + (task.verifiedByGit ? 'tag-success' : 'tag-warning') + '">' + (task.verifiedByGit ? 'Git verified' : 'Unverified') + '</span>';
            if (task.filesChanged && task.filesChanged.length > 0) {
                html += '<span class="meta-tag">' + task.filesChanged.length + ' files</span>';
            }
            html += '</div>';
            if (task.filesChanged && task.filesChanged.length > 0) {
                html += '<div class="task-files">';
                task.filesChanged.slice(0, 5).forEach(file => {
                    html += '<span class="file-tag">' + escapeHtml(truncatePath(file)) + '</span>';
                });
                if (task.filesChanged.length > 5) {
                    html += '<span class="file-tag">+' + (task.filesChanged.length - 5) + ' more</span>';
                }
                html += '</div>';
            }
            html += '</div>';
        });
        html += '</div>';
        html += '</div>';
    }

    // 待办事项
    if (s.pendingTasks.length > 0) {
        html += '<div class="handoff-section">';
        html += '<h3><span class="section-icon warning"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="8" x2="12" y2="12"/><line x1="12" y1="16" x2="12.01" y2="16"/></svg></span> ' + (t('pendingTasks') || '待办事项') + ' <span class="badge">' + s.pendingTasks.length + '</span></h3>';
        html += '<div class="task-list">';
        s.pendingTasks.forEach(task => {
            html += '<div class="task-item pending">';
            html += '<div class="task-desc">' + escapeHtml(task.description) + '</div>';
            html += '<div class="task-meta">';
            html += '<span class="meta-tag">' + task.sessionId.substring(0, 8) + '</span>';
            html += '<span class="meta-tag">' + task.source + '</span>';
            html += '</div>';
            if (task.filesHint && task.filesHint.length > 0) {
                html += '<div class="task-files">';
                task.filesHint.forEach(file => {
                    html += '<span class="file-tag">' + escapeHtml(truncatePath(file)) + '</span>';
                });
                html += '</div>';
            }
            html += '</div>';
        });
        html += '</div>';
        html += '</div>';
    }

    // 关键决策
    if (s.keyDecisions.length > 0) {
        html += '<div class="handoff-section">';
        html += '<h3><span class="section-icon info"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><line x1="12" y1="16" x2="12" y2="12"/><line x1="12" y1="8" x2="12.01" y2="8"/></svg></span> ' + (t('keyDecisions') || '关键决策') + ' <span class="badge">' + s.keyDecisions.length + '</span></h3>';
        html += '<div class="decision-list">';
        s.keyDecisions.forEach(decision => {
            html += '<div class="decision-item">';
            html += '<div class="decision-desc">' + escapeHtml(decision.description) + '</div>';
            if (decision.context) {
                html += '<div class="decision-context">' + escapeHtml(truncate(decision.context, 200)) + '</div>';
            }
            html += '</div>';
        });
        html += '</div>';
        html += '</div>';
    }

    // 修改的文件
    if (s.modifiedFiles.length > 0) {
        html += '<div class="handoff-section">';
        html += '<h3><span class="section-icon"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg></span> ' + (t('modifiedFiles') || '修改的文件') + ' <span class="badge">' + s.modifiedFiles.length + '</span></h3>';
        html += '<table class="file-table">';
        html += '<thead><tr>';
        html += '<th>' + (t('file') || '文件') + '</th>';
        html += '<th>' + (t('changeCount') || '操作次数') + '</th>';
        html += '<th>' + (t('lastAction') || '最后操作') + '</th>';
        html += '<th>' + (t('fileType') || '类型') + '</th>';
        html += '</tr></thead>';
        html += '<tbody>';
        s.modifiedFiles.forEach(file => {
            let typeBadge = '<span class="file-type-badge code">' + (t('code') || '代码') + '</span>';
            if (file.isTestFile) {
                typeBadge = '<span class="file-type-badge test">' + (t('test') || '测试') + '</span>';
            } else if (file.isConfigFile) {
                typeBadge = '<span class="file-type-badge config">' + (t('config') || '配置') + '</span>';
            }
            html += '<tr>';
            html += '<td class="file-path">' + escapeHtml(file.path) + '</td>';
            html += '<td>' + file.actionCount + '</td>';
            html += '<td>' + file.lastAction + '</td>';
            html += '<td>' + typeBadge + '</td>';
            html += '</tr>';
        });
        html += '</tbody></table>';
        html += '</div>';
    }

    // 已知问题
    if (s.knownIssues && s.knownIssues.length > 0) {
        html += '<div class="handoff-section">';
        html += '<h3><span class="section-icon danger"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/><line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/></svg></span> ' + (t('knownIssues') || '已知问题') + ' <span class="badge">' + s.knownIssues.length + '</span></h3>';
        html += '<div class="issue-list">';
        s.knownIssues.forEach(issue => {
            html += '<div class="issue-item">' + escapeHtml(issue) + '</div>';
        });
        html += '</div>';
        html += '</div>';
    }

    // 如果没有内容
    if (s.completedTasks.length === 0 && s.pendingTasks.length === 0 && s.keyDecisions.length === 0) {
        html += '<div class="continuity-empty">';
        html += '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/></svg>';
        html += '<h3>' + (t('noDataFound') || '未找到有效数据') + '</h3>';
        html += '<p>' + (t('tryMoreSessions') || '尝试增加分析的会话数量') + '</p>';
        html += '</div>';
    }

    content.innerHTML = html;
}

// 导出到 memory
async function exportContinuityToMemory() {
    console.log('[Continuity] exportContinuityToMemory called, project:', currentContinuityProject);
    if (!currentContinuityProject) {
        showContinuityToast(t('selectProjectFirst') || '请先选择项目', 'error');
        return;
    }

    // 如果没有已生成的摘要，先生成一个
    if (!continuitySummary) {
        showContinuityToast(t('generateSummaryFirst') || '请先生成交接摘要', 'error');
        return;
    }

    try {
        // 传递已生成的摘要数据，避免重新调用 LLM
        const path = await window.go.main.App.ExportContinuityToMemoryWithData(currentContinuityProject, continuitySummary);
        showContinuityToast((t('exportSuccess') || '导出成功') + ': ' + path, 'success');
    } catch (error) {
        console.error('导出到 memory 失败:', error);
        showContinuityToast((t('exportFailed') || '导出失败') + ': ' + error, 'error');
    }
}

// 生成 Markdown
async function generateContinuityMarkdown() {
    console.log('[Continuity] generateContinuityMarkdown called, project:', currentContinuityProject);
    if (!currentContinuityProject) {
        showContinuityToast(t('selectProjectFirst') || '请先选择项目', 'error');
        return;
    }

    // 如果没有已生成的摘要，先生成一个
    if (!continuitySummary) {
        showContinuityToast(t('generateSummaryFirst') || '请先生成交接摘要', 'error');
        return;
    }

    try {
        // 传递已生成的摘要数据，避免重新调用 LLM
        const markdown = await window.go.main.App.GenerateContinuityMarkdownWithData(currentContinuityProject, continuitySummary);
        showMarkdownPreview(markdown);
        showContinuityToast(t('markdownGenerated') || 'Markdown 已生成', 'success');
    } catch (error) {
        console.error('生成 Markdown 失败:', error);
        showContinuityToast((t('generateFailed') || '生成失败') + ': ' + error, 'error');
    }
}

// 生成 Prompt
async function generateContinuityPrompt() {
    console.log('[Continuity] generateContinuityPrompt called, project:', currentContinuityProject);
    if (!currentContinuityProject) {
        showContinuityToast(t('selectProjectFirst') || '请先选择项目', 'error');
        return;
    }

    // 如果没有已生成的摘要，先生成一个
    if (!continuitySummary) {
        showContinuityToast(t('generateSummaryFirst') || '请先生成交接摘要', 'error');
        return;
    }

    try {
        // 传递已生成的摘要数据，避免重新调用 LLM
        const prompt = await window.go.main.App.GenerateContinuityPromptWithData(currentContinuityProject, continuitySummary);
        // 兼容 Wails webview 环境的剪贴板写入
        if (navigator.clipboard && navigator.clipboard.writeText) {
            await navigator.clipboard.writeText(prompt);
        } else {
            // fallback: 使用 textarea + execCommand
            const ta = document.createElement('textarea');
            ta.value = prompt;
            ta.style.position = 'fixed';
            ta.style.left = '-9999px';
            document.body.appendChild(ta);
            ta.select();
            document.execCommand('copy');
            document.body.removeChild(ta);
        }
        showContinuityToast((t('copiedToClipboard') || '已复制到剪贴板'), 'success');
    } catch (error) {
        console.error('生成 Prompt 失败:', error);
        showContinuityToast((t('generateFailed') || '生成失败') + ': ' + error, 'error');
    }
}

// 显示 Markdown 预览
function showMarkdownPreview(markdown) {
    const content = document.getElementById('continuityContent');
    if (!content) return;

    let html = '<div class="handoff-header">';
    html += '<h2>' + (t('markdownPreview') || 'Markdown 预览') + '</h2>';
    html += '<div class="handoff-meta">';
    html += '<button class="btn-export" onclick="copyMarkdownToClipboard()"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 0 1-2-2V4a2 2 0 0 1 2-2h9a2 2 0 0 1 2 2v1"/></svg> ' + (t('copyToClipboard') || '复制到剪贴板') + '</button>';
    html += '<button class="btn-export" onclick="renderContinuitySummary()"><svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><line x1="19" y1="12" x2="5" y2="12"/><polyline points="12 19 5 12 12 5"/></svg> ' + (t('back') || '返回') + '</button>';
    html += '</div>';
    html += '</div>';
    html += '<div class="markdown-preview">' + escapeHtml(markdown) + '</div>';

    content.innerHTML = html;

    // 临时保存 markdown 内容
    content.dataset.markdown = markdown;
}

// 复制 Markdown 到剪贴板
async function copyMarkdownToClipboard() {
    const content = document.getElementById('continuityContent');
    if (!content || !content.dataset.markdown) return;

    try {
        await navigator.clipboard.writeText(content.dataset.markdown);
        showContinuityToast((t('copiedToClipboard') || '已复制到剪贴板'), 'success');
    } catch (error) {
        showContinuityToast(t('copyFailed') || '复制失败', 'error');
    }
}

// 显示/隐藏加载状态
function showContinuityLoading(show) {
    const content = document.getElementById('continuityContent');
    if (!content) return;

    if (show) {
        content.innerHTML = `
            <div class="continuity-progress">
                <div class="progress-header">
                    <div class="progress-spinner"></div>
                    <h3>${t('generating') || '正在生成交接摘要'}</h3>
                </div>
                <div class="progress-steps">
                    <div class="progress-step active" id="progressStep1">
                        <div class="step-indicator">
                            <div class="step-dot"></div>
                            <div class="step-line"></div>
                        </div>
                        <div class="step-content">
                            <div class="step-title">${t('stepLoadingSessions') || '加载会话数据'}</div>
                            <div class="step-desc">${t('stepLoadingSessionsDesc') || '读取项目历史会话记录'}</div>
                        </div>
                    </div>
                    <div class="progress-step" id="progressStep2">
                        <div class="step-indicator">
                            <div class="step-dot"></div>
                            <div class="step-line"></div>
                        </div>
                        <div class="step-content">
                            <div class="step-title">${t('stepAnalyzing') || '分析会话内容'}</div>
                            <div class="step-desc">${t('stepAnalyzingDesc') || '提取任务、决策和文件变更'}</div>
                        </div>
                    </div>
                    <div class="progress-step" id="progressStep3">
                        <div class="step-indicator">
                            <div class="step-dot"></div>
                            <div class="step-line"></div>
                        </div>
                        <div class="step-content">
                            <div class="step-title">${t('stepValidating') || '验证数据完整性'}</div>
                            <div class="step-desc">${t('stepValidatingDesc') || '交叉验证 Git 提交记录'}</div>
                        </div>
                    </div>
                    <div class="progress-step" id="progressStep4">
                        <div class="step-indicator">
                            <div class="step-dot"></div>
                        </div>
                        <div class="step-content">
                            <div class="step-title">${t('stepGenerating') || '生成交接摘要'}</div>
                            <div class="step-desc">${t('stepGeneratingDesc') || '组织最终报告'}</div>
                        </div>
                    </div>
                </div>
                <div class="progress-bar-container">
                    <div class="progress-bar" id="continuityProgressBar" style="width: 0%"></div>
                </div>
            </div>`;
        animateProgressSteps();
    } else {
        // 如果有摘要则渲染摘要，否则恢复空状态
        if (continuitySummary) {
            renderContinuitySummary();
        } else {
            content.innerHTML = `
                <div class="continuity-empty">
                    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/>
                        <circle cx="9" cy="7" r="4"/>
                        <path d="M23 21v-2a4 4 0 0 0-3-3.87"/>
                        <path d="M16 3.13a4 4 0 0 1 0 7.75"/>
                    </svg>
                    <h3>${t('continuityEmpty') || '多会话接力引擎'}</h3>
                    <p>${t('continuityEmptyDesc') || '选择项目并点击"生成交接摘要"来分析最近的会话'}</p>
                </div>`;
        }
    }
}

// 动画进度步骤
let progressAnimTimer = null;
function animateProgressSteps() {
    let currentStep = 1;
    const totalSteps = 4;

    function advanceStep() {
        if (currentStep > totalSteps) return;

        // Mark current step as completed
        const stepEl = document.getElementById('progressStep' + currentStep);
        if (stepEl) {
            stepEl.classList.remove('active');
            stepEl.classList.add('completed');
        }

        // Update progress bar
        const bar = document.getElementById('continuityProgressBar');
        if (bar) {
            bar.style.width = (currentStep / totalSteps * 100) + '%';
        }

        currentStep++;

        // Activate next step
        if (currentStep <= totalSteps) {
            const nextStep = document.getElementById('progressStep' + currentStep);
            if (nextStep) {
                nextStep.classList.add('active');
            }
        }

        progressAnimTimer = setTimeout(advanceStep, 800 + Math.random() * 600);
    }

    progressAnimTimer = setTimeout(advanceStep, 500);
}

function stopProgressAnimation() {
    if (progressAnimTimer) {
        clearTimeout(progressAnimTimer);
        progressAnimTimer = null;
    }
}

// 显示 toast 消息（continuity 面板专用，不覆盖全局 showToast）
function showContinuityToast(message, type) {
    console.log('[Continuity] showToast:', type, message);
    // 移除现有的 toast
    const existing = document.querySelector('.continuity-toast');
    if (existing) {
        existing.remove();
    }

    const toast = document.createElement('div');
    toast.className = 'continuity-toast ' + (type || '');
    toast.textContent = message;
    toast.style.cssText = 'position:fixed;bottom:20px;right:20px;padding:12px 24px;background:' +
        (type === 'error' ? '#ef4444' : type === 'success' ? '#10b981' : '#1f2937') +
        ';color:#fff;border-radius:8px;font-size:14px;z-index:99999;box-shadow:0 4px 12px rgba(0,0,0,0.3);max-width:400px;word-break:break-word;animation:toastIn 0.3s ease;';
    document.body.appendChild(toast);

    // 3 秒后自动消失
    setTimeout(() => {
        toast.remove();
    }, 3000);
}

// 辅助函数
function truncate(text, maxLen) {
    if (!text) return '';
    if (text.length <= maxLen) return text;
    return text.substring(0, maxLen) + '...';
}

function truncatePath(path) {
    if (!path) return '';
    if (path.length <= 50) return path;
    const parts = path.split('/');
    if (parts.length <= 2) return path;
    return '.../' + parts.slice(-2).join('/');
}

function formatTime(timeStr) {
    if (!timeStr) return '';
    try {
        const date = new Date(timeStr);
        if (isNaN(date.getTime())) return timeStr;
        return date.toLocaleString();
    } catch {
        return timeStr;
    }
}
