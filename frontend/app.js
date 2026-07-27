// @ts-check

// ============================================
// Global State
// ============================================
let currentSessionId = null;
let currentDiffMode = 'uncommitted';
let currentTab = 'dashboard'; // 'dashboard' | 'sessions' | 'knowledge' | 'continuity'
let sessions = [];
let sessionsLoaded = false;

// Session Management Enhancement State
let batchMode = false;
let selectedSessions = new Set();
let currentFilter = 'all';
let allTags = [];
let currentMeta = null;

// ============================================
// Initialization
// ============================================
document.addEventListener('DOMContentLoaded', () => {
    setupEventListeners();
    initI18n();
    initMonitoring();
    initTheme();
    initContinuity();

    // Default to dashboard tab
    switchTab('dashboard');

    // Silent refresh sessions every 2 minutes
    setInterval(silentRefreshSessions, 120000);

    // Hide splash screen after app initialization
    hideSplashScreen();
});

// ============================================
// Splash Screen Control
// ============================================
function hideSplashScreen() {
    const splashScreen = document.getElementById('splashScreen');
    if (!splashScreen) return;

    // 等待后端就绪（最多 10 秒），然后隐藏 splash screen
    let attempts = 0;
    const maxAttempts = 20; // 20 * 500ms = 10s

    function checkBackend() {
        attempts++;
        // 尝试调用一个轻量级 API 来检测后端是否就绪
        if (window.go && window.go.main && window.go.main.App && window.go.main.App.GetSessions) {
            splashScreen.classList.add('hidden');
            setTimeout(() => splashScreen.remove(), 600);
        } else if (attempts < maxAttempts) {
            setTimeout(checkBackend, 500);
        } else {
            // 超时后强制隐藏
            splashScreen.classList.add('hidden');
            setTimeout(() => splashScreen.remove(), 600);
        }
    }

    // 首次延迟 500ms 后开始检查（给后端启动时间）
    setTimeout(checkBackend, 500);
}

// ============================================
// Tab Navigation
// ============================================
function switchTab(tab) {
    currentTab = tab;

    // Update nav tab active state
    document.querySelectorAll('.nav-tab').forEach(btn => {
        btn.classList.toggle('active', btn.getAttribute('data-tab') === tab);
    });

    // Show/hide panels
    document.getElementById('dashboardPanel').classList.toggle('active', tab === 'dashboard');
    document.getElementById('sessionsPanel').classList.toggle('active', tab === 'sessions');
    document.getElementById('knowledgePanel').classList.toggle('active', tab === 'knowledge');
    document.getElementById('continuityPanel').classList.toggle('active', tab === 'continuity');
    document.getElementById('pluginStudioPanel').classList.toggle('active', tab === 'pluginStudio');

    // Load data for the selected tab
    if (tab === 'dashboard') {
        loadDashboard();
    } else if (tab === 'sessions' && !sessionsLoaded) {
        loadSessions();
        sessionsLoaded = true;
    } else if (tab === 'knowledge') {
        loadKnowledgeDocuments();
    } else if (tab === 'continuity') {
        initContinuity();
    } else if (tab === 'pluginStudio') {
        initPluginStudio();
    }
}

// ============================================
// Session Management (existing logic)
// ============================================

async function silentRefreshSessions() {
    try {
        const newSessions = await window.go.main.App.GetSessions();
        if (sessionsChanged(sessions, newSessions)) {
            sessions = newSessions;
            if (currentTab === 'sessions') {
                renderSessionList(sessions);
            }
        }
    } catch (error) {
        // Silent fail
    }
}

function sessionsChanged(oldSessions, newSessions) {
    if (oldSessions.length !== newSessions.length) return true;
    if (oldSessions.length === 0) return false;
    for (let i = 0; i < oldSessions.length; i++) {
        if (oldSessions[i].id !== newSessions[i].id) return true;
    }
    return false;
}

async function loadSessions() {
    const sessionList = document.getElementById('sessionList');
    sessionList.innerHTML = `<div class="loading">${t('loading')}</div>`;

    try {
        sessions = await window.go.main.App.GetSessions();
        renderSessionList(sessions);
    } catch (error) {
        sessionList.innerHTML = `<div class="loading">${escapeHtml(t('loadFailed'))}: ${escapeHtml(String(error))}</div>`;
    }
}

function renderSessionList(sessionData) {
    const sessionList = document.getElementById('sessionList');

    if (!sessionData || sessionData.length === 0) {
        sessionList.innerHTML = `
            <div class="empty-state">
                <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <path d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
                </svg>
                <p>${t('noSessions')}</p>
            </div>
        `;
        return;
    }

    const groups = {};
    sessionData.forEach(session => {
        const project = session.projectName || session.projectDir || 'unknown';
        if (!groups[project]) {
            groups[project] = { dir: session.projectDir, name: project, sessions: [] };
        }
        groups[project].sessions.push(session);
    });

    const sortedGroups = Object.values(groups).sort((a, b) => b.sessions.length - a.sessions.length);

    sessionList.innerHTML = sortedGroups.map((group, groupIndex) => `
        <div class="session-group" data-group="${groupIndex}">
            <div class="group-header" onclick="toggleGroup(${groupIndex})">
                <svg class="group-icon" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M6 9l6 6 6-6"/>
                </svg>
                <span class="group-name">${escapeHtml(group.name)}</span>
                <span class="group-count">${group.sessions.length}</span>
            </div>
            <div class="group-sessions">
                ${group.sessions.map(session => `
                    <div class="session-item ${batchMode ? 'batch-mode' : ''}" data-id="${session.id}">
                        ${batchMode ? `<input type="checkbox" class="session-checkbox" ${selectedSessions.has(session.id) ? 'checked' : ''}>` : ''}
                        <div class="session-main">
                            <div class="session-id">${session.id.substring(0, 8)}</div>
                            <div class="session-model">${session.model || '-'}</div>
                            <div class="session-prompt">${session.prompt || '-'}</div>
                            <div class="session-meta">
                                <span>${t('files')} ${session.fileCount}</span>
                                <span>${t('actions')} ${session.actionCount}</span>
                                <span class="session-time">${formatSessionTime(session.startedAt)}</span>
                            </div>
                        </div>
                    </div>
                `).join('')}
            </div>
        </div>
    `).join('');

    // 绑定点击事件
    document.querySelectorAll('.session-item').forEach(item => {
        item.addEventListener('click', (e) => {
            // 如果是复选框，不触发选择会话
            if (e.target.classList.contains('session-checkbox')) {
                const sessionId = item.getAttribute('data-id');
                if (e.target.checked) {
                    selectedSessions.add(sessionId);
                } else {
                    selectedSessions.delete(sessionId);
                }
                updateBatchActionBar();
                return;
            }

            if (batchMode) {
                // 批量模式下，点击整个项切换选中
                const checkbox = item.querySelector('.session-checkbox');
                const sessionId = item.getAttribute('data-id');
                if (checkbox) {
                    checkbox.checked = !checkbox.checked;
                    if (checkbox.checked) {
                        selectedSessions.add(sessionId);
                    } else {
                        selectedSessions.delete(sessionId);
                    }
                    updateBatchActionBar();
                }
            } else {
                selectSession(item.getAttribute('data-id'));
            }
        });
    });

    // 默认全部折叠，让用户按需展开
    sortedGroups.forEach((group, index) => {
        toggleGroup(index);
    });
}

function toggleGroup(groupIndex) {
    const group = document.querySelector(`.session-group[data-group="${groupIndex}"]`);
    if (group) group.classList.toggle('collapsed');
}

async function selectSession(sessionId) {
    currentSessionId = sessionId;

    document.querySelectorAll('.session-item').forEach(item => {
        item.classList.toggle('active', item.getAttribute('data-id') === sessionId);
    });

    try {
        const detail = await window.go.main.App.GetSession(sessionId);
        renderSessionDetail(detail);

        // 设置当前项目目录
        if (detail.projectDir) {
            window.currentProjectDir = detail.projectDir;
        }

        // 加载对话记录
        await loadConversation(sessionId);
    } catch (error) {
        console.error('Failed to load session detail:', error);
    }
}

function renderSessionDetail(detail) {
    document.getElementById('statusSession').innerHTML = `${t('session')}: ${detail.id.substring(0, 8)}`;
    document.getElementById('statusBranch').innerHTML = `${t('branch')}: ${detail.branch || '-'}`;
    document.getElementById('statusTokens').innerHTML = `Token: ${formatNumber(detail.tokenUsage.inputTokens)} ${t('tokenIn')} / ${formatNumber(detail.tokenUsage.outputTokens)} ${t('tokenOut')}`;

    const fileTableBody = document.getElementById('fileTableBody');
    const fileCount = document.getElementById('fileCount');
    const filesTabCount = document.getElementById('filesTabCount');

    // 更新文件标签计数
    if (filesTabCount) {
        filesTabCount.textContent = detail.fileChanges ? detail.fileChanges.length : 0;
    }

    if (!detail.fileChanges || detail.fileChanges.length === 0) {
        fileTableBody.innerHTML = `
            <tr>
                <td colspan="4" class="empty-state">
                    <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                        <path d="M9 12h6m-6 4h6m2 5H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z"/>
                    </svg>
                    <p>${t('noChanges')}</p>
                </td>
            </tr>
        `;
        fileCount.textContent = '0';
        return;
    }

    fileCount.textContent = detail.fileChanges.length;

    fileTableBody.innerHTML = detail.fileChanges.map((file, index) => `
        <tr data-index="${index}" data-path="${escapeHtmlAttr(file.path)}">
            <td><span class="risk-badge risk-${(file.risk || 'review').toLowerCase()}">${t((file.risk || 'review').toLowerCase())}</span></td>
            <td title="${escapeHtmlAttr(file.path)}">${truncatePath(file.path)}</td>
            <td><span class="change-badge change-${(file.changeType || 'modified').toLowerCase()}">${t((file.changeType || 'modified').toLowerCase())}</span></td>
            <td>${file.actionCount || 0}</td>
        </tr>
    `).join('');

    document.querySelectorAll('#fileTableBody tr').forEach(row => {
        row.addEventListener('click', () => selectFile(row.getAttribute('data-path')));
    });

    if (detail.fileChanges.length > 0) {
        selectFile(detail.fileChanges[0].path);
    }
}

async function selectFile(filePath) {
    document.querySelectorAll('#fileTableBody tr').forEach(row => {
        row.classList.toggle('active', row.getAttribute('data-path') === filePath);
    });

    document.getElementById('diffFileName').textContent = filePath;

    try {
        let diff = '';
        if (currentDiffMode === 'session') {
            const diffInfo = await window.go.main.App.GetSessionDiff(currentSessionId, 'session');
            if (diffInfo && diffInfo.diffs) {
                const fileDiff = diffInfo.diffs.find(d => d.filePath === filePath);
                if (fileDiff) diff = fileDiff.patch;
            }
        } else {
            diff = await window.go.main.App.GetDiff(currentSessionId, filePath);
        }
        renderDiff(diff, filePath);
    } catch (error) {
        document.getElementById('diffView').innerHTML = `<code>${escapeHtml(t('loadFailed'))}: ${escapeHtml(String(error))}</code>`;
    }
}

function renderDiff(diff, filePath) {
    const diffView = document.getElementById('diffView');
    if (!diff) {
        diffView.innerHTML = `<code>${t('noDiff')}</code>`;
        return;
    }
    diffView.innerHTML = `<code>${highlightDiff(diff)}</code>`;
}

function highlightDiff(diff) {
    return diff.split('\n').map(line => {
        if (line.startsWith('+')) {
            return `<span class="diff-add">${escapeHtml(line)}</span>`;
        } else if (line.startsWith('-')) {
            return `<span class="diff-remove">${escapeHtml(line)}</span>`;
        } else if (line.startsWith('@@') || line.startsWith('diff') || line.startsWith('index')) {
            return `<span class="diff-header-line">${escapeHtml(line)}</span>`;
        }
        return escapeHtml(line);
    }).join('\n');
}

// ============================================
// Export
// ============================================
async function exportSession() {
    if (!currentSessionId) {
        showToast(t('selectSessionFirst'));
        return;
    }
    try {
        const path = await window.go.main.App.SelectDirectory();
        if (!path) return;
        showToast(t('exporting'));
        const result = await window.go.main.App.ExportSession(currentSessionId, 'markdown', path);
        if (result && result.filePath) showToast(t('exportSuccess'));
    } catch (error) {
        console.error('Export failed:', error);
        showToast(t('exportFailed') + ': ' + error);
    }
}

// ============================================
// i18n
// ============================================
function initI18n() {
    updateLangToggle();
    updateUI();
}

function updateLangToggle() {
    const toggle = document.getElementById('langToggle');
    if (!toggle) return;
    const knob = toggle.querySelector('.toggle-knob');
    if (!knob) return;
    const isEn = getCurrentLang() === 'en';
    toggle.classList.toggle('active', isEn);
    knob.textContent = isEn ? '中' : 'EN';
}

function updateUI() {
    updateLangToggle();

    document.querySelectorAll('[data-i18n]').forEach(el => {
        const key = el.getAttribute('data-i18n');
        if (key) el.textContent = t(key);
    });

    document.querySelectorAll('[data-i18n-placeholder]').forEach(el => {
        const key = el.getAttribute('data-i18n-placeholder');
        if (key) el.placeholder = t(key);
    });

    // Re-render current tab content
    if (currentTab === 'dashboard') {
        loadDashboard();
    } else if (sessions.length > 0) {
        renderSessionList(sessions);
    }
}

// ============================================
// Monitoring
// ============================================
let isMonitoring = false;

async function initMonitoring() {
    try {
        isMonitoring = await window.go.main.App.IsMonitoring();
        updateMonitoringUI();
    } catch (error) {
        console.error('Failed to check monitoring status:', error);
    }

    window.runtime.EventsOn('session-updated', () => {
        silentRefreshSessions();
        showToast(t('sessionUpdated'));
    });
}

async function toggleMonitoring() {
    try {
        if (isMonitoring) {
            await window.go.main.App.StopMonitoring();
            isMonitoring = false;
            showToast(t('monitoringStopped'));
        } else {
            const started = await window.go.main.App.StartMonitoring();
            if (started) {
                isMonitoring = true;
                showToast(t('monitoringStarted'));
            }
        }
        updateMonitoringUI();
    } catch (error) {
        console.error('Failed to toggle monitoring:', error);
        showToast(t('monitoringFailed'));
    }
}

function updateMonitoringUI() {
    const indicator = document.getElementById('monitorIndicator');
    if (indicator) {
        indicator.classList.toggle('active', isMonitoring);
        indicator.title = isMonitoring ? t('monitoringActive') : t('monitoringInactive');
    }
}

// ============================================
// Theme
// ============================================
async function initTheme() {
    try {
        const settings = await window.go.main.App.GetSettings();
        if (settings && settings.theme) applyTheme(settings.theme);
    } catch (error) {
        // Silent fail
    }
}

function applyTheme(theme) {
    if (theme === 'auto') {
        const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
        document.documentElement.setAttribute('data-theme', prefersDark ? 'dark' : 'light');
    } else {
        document.documentElement.setAttribute('data-theme', theme);
    }
}

// ============================================
// Toast
// ============================================
function showToast(message, severity) {
    const toast = document.createElement('div');
    toast.className = 'toast' + (severity === 'error' ? ' toast-error' : severity === 'success' ? ' toast-success' : '');
    toast.textContent = message;
    document.body.appendChild(toast);
    setTimeout(() => toast.classList.add('show'), 10);
    setTimeout(() => {
        toast.classList.remove('show');
        setTimeout(() => toast.remove(), 300);
    }, 3000);
}

// ============================================
// Settings Panel
// ============================================
let currentSettings = null;
let settingsModalAnimating = false;

async function openSettings() {
    if (settingsModalAnimating) return;
    const modal = document.getElementById('settingsModal');
    settingsModalAnimating = true;
    modal.style.display = 'flex';
    setTimeout(() => {
        modal.classList.add('show');
        settingsModalAnimating = false;
    }, 10);

    try {
        currentSettings = await window.go.main.App.GetSettings();
        renderSettings();
    } catch (error) {
        console.error('Failed to load settings:', error);
    }
}

function closeSettings() {
    if (settingsModalAnimating) return;
    const modal = document.getElementById('settingsModal');
    settingsModalAnimating = true;
    modal.classList.remove('show');
    setTimeout(() => {
        modal.style.display = 'none';
        settingsModalAnimating = false;
    }, 200);
}

function renderSettings() {
    if (!currentSettings) return;
    document.querySelectorAll('.theme-btn').forEach(btn => {
        btn.classList.toggle('active', btn.getAttribute('data-theme') === currentSettings.theme);
    });
    renderCustomRules();
    renderSettingsLLMConfig();
}

async function setTheme(theme) {
    try {
        await window.go.main.App.UpdateTheme(theme);
        currentSettings.theme = theme;
        document.querySelectorAll('.theme-btn').forEach(btn => {
            btn.classList.toggle('active', btn.getAttribute('data-theme') === theme);
        });
        applyTheme(theme);
        showToast(t('theme') + ': ' + t('theme' + theme.charAt(0).toUpperCase() + theme.slice(1)));
    } catch (error) {
        console.error('Failed to set theme:', error);
    }
}

// ==============================
// LLM 配置（设置面板）
// ==============================

function renderSettingsLLMConfig() {
    window.go.main.App.GetLLMConfig().then(cfg => {
        if (cfg) {
            document.getElementById('settingsLLMEnabled').checked = cfg.enabled || false;
            document.getElementById('settingsLLMProviderName').value = cfg.provider || '';
            document.getElementById('settingsLLMAPIKey').value = cfg.apiKey || '';
            document.getElementById('settingsLLMBaseURL').value = cfg.baseUrl || '';
            document.getElementById('settingsLLMModel').value = cfg.model || '';
        }
        onSettingsLLMEnabledChange();
    }).catch(err => {
        console.error('Failed to load LLM config:', err);
    });
}

function onSettingsLLMEnabledChange() {
    var enabled = document.getElementById('settingsLLMEnabled').checked;
    document.getElementById('settingsLLMConfigFields').style.display = enabled ? 'block' : 'none';
}

function toggleLLMEye(inputId, btn) {
    var input = document.getElementById(inputId);
    if (input.type === 'password') {
        input.type = 'text';
        btn.querySelector('.eye-open').style.display = 'none';
        btn.querySelector('.eye-closed').style.display = '';
    } else {
        input.type = 'password';
        btn.querySelector('.eye-open').style.display = '';
        btn.querySelector('.eye-closed').style.display = 'none';
    }
}

async function saveSettingsLLMConfig() {
    var provider = document.getElementById('settingsLLMProviderName').value.trim();
    var apiKey = document.getElementById('settingsLLMAPIKey').value.trim();
    var baseURL = document.getElementById('settingsLLMBaseURL').value.trim();
    var model = document.getElementById('settingsLLMModel').value.trim();
    var enabled = document.getElementById('settingsLLMEnabled').checked;

    try {
        await window.go.main.App.SaveLLMConfig(provider, apiKey, baseURL, model, enabled);
        showToast(t('llmConfigSaved'));
    } catch (error) {
        console.error('保存LLM配置失败:', error);
        showToast(t('llmConfigFailed') + ': ' + error);
    }
}

async function testSettingsLLMConnection() {
    var provider = document.getElementById('settingsLLMProviderName').value.trim();
    var apiKey = document.getElementById('settingsLLMAPIKey').value.trim();
    var baseURL = document.getElementById('settingsLLMBaseURL').value.trim();
    var model = document.getElementById('settingsLLMModel').value.trim();
    var resultEl = document.getElementById('settingsLLMTestResult');

    if (!apiKey) {
        showToast(t('llmEnterAPIKey'));
        return;
    }

    resultEl.className = 'llm-test-result loading';
    resultEl.textContent = t('llmTesting');

    try {
        await window.go.main.App.TestLLMConnection(provider, apiKey, baseURL, model);
        resultEl.className = 'llm-test-result success';
        resultEl.textContent = t('llmTestSuccess');
    } catch (error) {
        resultEl.className = 'llm-test-result error';
        resultEl.textContent = t('llmTestFailed') + ': ' + error;
    }
}

function renderCustomRules() {
    const container = document.getElementById('customRulesList');
    if (!container || !currentSettings) return;

    const customRules = currentSettings.customRules || [];
    if (customRules.length === 0) {
        container.innerHTML = `<p style="color: var(--text-tertiary); font-size: 13px;">${t('noRules') || '暂无自定义规则'}</p>`;
        return;
    }

    container.innerHTML = customRules.map((rule, index) => `
        <div class="rule-item">
            <div class="rule-info">
                <div class="rule-name">${escapeHtml(rule.name)}</div>
                <div class="rule-desc">${escapeHtml(rule.description)}</div>
            </div>
            <span class="rule-level ${(rule.level || 'review').toLowerCase()}">${rule.level || 'Review'}</span>
            <button class="rule-delete" data-rule-index="${index}" title="${t('deleteRule')}">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <line x1="18" y1="6" x2="6" y2="18"/>
                    <line x1="6" y1="6" x2="18" y2="18"/>
                </svg>
            </button>
        </div>
    `).join('');

    container.querySelectorAll('.rule-delete').forEach(btn => {
        btn.addEventListener('click', () => {
            const index = parseInt(btn.getAttribute('data-rule-index'));
            if (!isNaN(index) && currentSettings.customRules[index]) {
                deleteRule(currentSettings.customRules[index].name);
            }
        });
    });
}

function showAddRuleForm() {
    const name = prompt(t('ruleNamePlaceholder') || '规则名称:');
    if (!name) return;
    const description = prompt(t('ruleDescPlaceholder') || '规则描述:');
    if (!description) return;
    const level = prompt(t('ruleLevelPlaceholder') || '风险等级 (safe/review/danger):', 'review');
    if (!level || !['safe', 'review', 'danger'].includes(level.toLowerCase())) {
        showToast(t('invalidLevel') || '无效的风险等级');
        return;
    }
    const pattern = prompt(t('rulePatternPlaceholder') || '文件路径匹配模式:');
    if (!pattern) return;
    addRule(name, description, level.toLowerCase(), pattern);
}

async function addRule(name, description, level, pattern) {
    try {
        await window.go.main.App.AddCustomRule(name, description, level, pattern);
        currentSettings.customRules.push({ name, description, level, pattern, enabled: true });
        renderCustomRules();
        showToast(t('ruleAdded') || '规则已添加');
    } catch (error) {
        console.error('Failed to add rule:', error);
    }
}

async function deleteRule(name) {
    if (!confirm(t('confirmDelete') || '确定要删除这个规则吗？')) return;
    try {
        await window.go.main.App.RemoveCustomRule(name);
        currentSettings.customRules = currentSettings.customRules.filter(r => r.name !== name);
        renderCustomRules();
        showToast(t('ruleDeleted') || '规则已删除');
    } catch (error) {
        console.error('Failed to delete rule:', error);
    }
}

// ============================================
// Event Listeners
// ============================================
function setupEventListeners() {
    // Refresh button — refreshes the current tab
    document.getElementById('refreshBtn').addEventListener('click', () => {
        if (currentTab === 'dashboard') {
            loadDashboard();
        } else {
            loadSessions();
        }
    });

    // Language toggle
    document.getElementById('langToggle').addEventListener('click', () => switchLang());

    // Search input
    document.getElementById('searchInput').addEventListener('input', (e) => {
        const keyword = e.target.value.toLowerCase();
        document.querySelectorAll('.session-item').forEach(item => {
            const text = item.textContent.toLowerCase();
            item.style.display = text.includes(keyword) ? '' : 'none';
        });
    });

    // Diff mode buttons
    document.querySelectorAll('.diff-mode-btn').forEach(btn => {
        btn.addEventListener('click', () => switchDiffMode(btn.getAttribute('data-mode')));
    });

    // Close settings modal on backdrop click
    document.addEventListener('click', (e) => {
        if (e.target.id === 'settingsModal') closeSettings();
    });
}

function switchDiffMode(mode) {
    currentDiffMode = mode;
    document.querySelectorAll('.diff-mode-btn').forEach(btn => {
        btn.classList.toggle('active', btn.getAttribute('data-mode') === mode);
    });
    const diffFileName = document.getElementById('diffFileName');
    if (diffFileName && diffFileName.textContent && currentSessionId) {
        selectFile(diffFileName.textContent);
    }
}

// ============================================
// Utility Functions
// ============================================
function escapeHtml(text) {
    if (!text) return '';
    const div = document.createElement('div');
    div.textContent = text;
    return div.innerHTML;
}

function escapeHtmlAttr(text) {
    return text
        .replace(/&/g, '&amp;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#39;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');
}

function truncatePath(path) {
    if (path.length > 40) return '...' + path.slice(-37);
    return path;
}

function formatNumber(num) {
    if (num >= 1000000) return (num / 1000000).toFixed(1) + 'M';
    if (num >= 1000) return (num / 1000).toFixed(1) + 'K';
    return num.toString();
}

// ============================================
// Session Management Enhancement Functions
// ============================================

// 高级搜索
function toggleAdvancedSearch() {
    const panel = document.getElementById('advancedSearchPanel');
    panel.style.display = panel.style.display === 'none' ? 'block' : 'none';

    // 加载标签列表
    if (panel.style.display === 'block') {
        loadTagsForFilter();
    }
}

async function loadTagsForFilter() {
    try {
        allTags = await window.go.main.App.GetAllTags();
        renderTagFilter();
    } catch (error) {
        console.error('Failed to load tags:', error);
    }
}

function renderTagFilter() {
    const container = document.getElementById('tagFilterContainer');
    if (!container) return;

    if (allTags.length === 0) {
        container.innerHTML = `<span class="no-tags">${t('noTags')}</span>`;
        return;
    }

    container.innerHTML = allTags.map(tag => `
        <label class="filter-checkbox">
            <input type="checkbox" class="tag-filter-checkbox" value="${escapeHtmlAttr(tag)}">
            <span>${escapeHtml(tag)}</span>
        </label>
    `).join('');
}

async function applyAdvancedSearch() {
    const keyword = document.getElementById('searchInput').value;

    // 收集搜索字段
    const fields = [];
    const searchPrompt = document.getElementById('searchPrompt');
    const searchModel = document.getElementById('searchModel');
    const searchBranch = document.getElementById('searchBranch');
    const searchTags = document.getElementById('searchTags');

    if (searchPrompt && searchPrompt.checked) fields.push('prompt');
    if (searchModel && searchModel.checked) fields.push('model');
    if (searchBranch && searchBranch.checked) fields.push('branch');
    if (searchTags && searchTags.checked) fields.push('tags');

    // 收集标签筛选
    const tags = [];
    document.querySelectorAll('.tag-filter-checkbox:checked').forEach(cb => {
        tags.push(cb.value);
    });

    // 收藏筛选
    const favoritedCheckbox = document.getElementById('filterFavorited');
    const favorited = favoritedCheckbox && favoritedCheckbox.checked ? true : null;

    try {
        const results = await window.go.main.App.SearchSessions(keyword, fields, tags, favorited);
        const sessionIds = results.map(r => r.sessionId);
        const filteredSessions = sessions.filter(s => sessionIds.includes(s.id));
        renderSessionList(filteredSessions);
    } catch (error) {
        console.error('Search failed:', error);
        showToast(t('searchFailed'));
    }
}

// 快捷筛选
function setQuickFilter(filter) {
    currentFilter = filter;

    // 更新按钮状态
    document.querySelectorAll('.filter-btn').forEach(btn => {
        btn.classList.toggle('active', btn.getAttribute('data-filter') === filter);
    });

    // 应用筛选
    applyFilter();
}

async function applyFilter() {
    let filteredSessions = [...sessions];

    switch (currentFilter) {
        case 'favorited':
            try {
                const favoriteIds = await window.go.main.App.GetFavoriteSessions();
                filteredSessions = sessions.filter(s => favoriteIds.includes(s.id));
            } catch (error) {
                console.error('Failed to get favorites:', error);
            }
            break;
        case 'today':
            const today = new Date();
            today.setHours(0, 0, 0, 0);
            filteredSessions = sessions.filter(s => new Date(s.startedAt) >= today);
            break;
        case 'all':
        default:
            // 不过滤
            break;
    }

    renderSessionList(filteredSessions);
}

// 批量模式
function toggleBatchMode() {
    batchMode = !batchMode;
    selectedSessions.clear();

    document.getElementById('batchModeBtn').classList.toggle('active', batchMode);
    document.getElementById('batchActionBar').style.display = batchMode ? 'flex' : 'none';

    // 重新渲染会话列表
    renderSessionList(sessions);
    updateBatchActionBar();
}

function cancelBatchMode() {
    batchMode = false;
    selectedSessions.clear();

    document.getElementById('batchModeBtn').classList.remove('active');
    document.getElementById('batchActionBar').style.display = 'none';

    renderSessionList(sessions);
}

function updateBatchActionBar() {
    document.getElementById('batchSelectedCount').textContent = selectedSessions.size;
}

async function batchFavorite() {
    if (selectedSessions.size === 0) {
        showToast(t('selectSessionsFirst'));
        return;
    }

    try {
        const op = {
            action: 'favorite',
            sessionIds: Array.from(selectedSessions)
        };
        const result = await window.go.main.App.BatchOperation(op);
        showToast(t('batchFavoriteSuccess').replace('{count}', result.success));
        cancelBatchMode();
        loadSessions();
    } catch (error) {
        console.error('Batch favorite failed:', error);
        showToast(t('batchOperationFailed'));
    }
}

async function batchExport() {
    if (selectedSessions.size === 0) {
        showToast(t('selectSessionsFirst'));
        return;
    }

    try {
        const path = await window.go.main.App.SelectDirectory();
        if (!path) return;

        showToast(t('exporting'));
        const op = {
            action: 'export',
            sessionIds: Array.from(selectedSessions),
            format: 'markdown',
            outputDir: path
        };
        const result = await window.go.main.App.BatchOperation(op);
        showToast(t('batchExportSuccess').replace('{count}', result.success));
        cancelBatchMode();
    } catch (error) {
        console.error('Batch export failed:', error);
        showToast(t('batchOperationFailed'));
    }
}

async function batchDelete() {
    if (selectedSessions.size === 0) {
        showToast(t('selectSessionsFirst'));
        return;
    }

    if (!confirm(t('confirmBatchDelete').replace('{count}', selectedSessions.size))) {
        return;
    }

    try {
        const op = {
            action: 'delete',
            sessionIds: Array.from(selectedSessions)
        };
        const result = await window.go.main.App.BatchOperation(op);
        showToast(t('batchDeleteSuccess').replace('{count}', result.success));
        cancelBatchMode();
        loadSessions();
    } catch (error) {
        console.error('Batch delete failed:', error);
        showToast(t('batchOperationFailed'));
    }
}

// 会话元数据管理
async function selectSessionWithMeta(sessionId) {
    currentSessionId = sessionId;

    document.querySelectorAll('.session-item').forEach(item => {
        item.classList.toggle('active', item.getAttribute('data-id') === sessionId);
    });

    try {
        // 获取带元数据的会话详情
        const data = await window.go.main.App.GetSessionDetailWithMeta(sessionId);
        currentMeta = data.meta;

        renderSessionDetail(data.detail);
        renderSessionMeta(data.meta);

        // 显示元数据栏
        document.getElementById('sessionMetaBar').style.display = 'flex';
        document.getElementById('sessionNoteBtn').style.display = 'flex';

        // 加载对话记录
        await loadConversation(sessionId);
    } catch (error) {
        console.error('Failed to load session detail:', error);
    }
}

function renderSessionMeta(meta) {
    // 更新收藏按钮状态
    const favoriteBtn = document.getElementById('favoriteBtn');
    favoriteBtn.classList.toggle('active', meta.favorited);

    // 渲染标签
    const tagsContainer = document.getElementById('sessionTags');
    const allTags = [...(meta.tags || []), ...(meta.autoTags || [])];

    if (allTags.length === 0) {
        tagsContainer.innerHTML = '';
        return;
    }

    tagsContainer.innerHTML = allTags.map(tag => `
        <span class="session-tag ${(meta.tags || []).includes(tag) ? 'manual' : 'auto'}">
            ${escapeHtml(tag)}
            ${(meta.tags || []).includes(tag) ? `<button class="tag-remove" onclick="removeTag('${escapeHtmlAttr(tag)}')">&times;</button>` : ''}
        </span>
    `).join('');
}

async function toggleFavorite() {
    if (!currentSessionId || !currentMeta) return;

    try {
        const newState = !currentMeta.favorited;
        await window.go.main.App.SetSessionFavorite(currentSessionId, newState);
        currentMeta.favorited = newState;

        document.getElementById('favoriteBtn').classList.toggle('active', newState);
        showToast(newState ? t('addedToFavorite') : t('removedFromFavorite'));
    } catch (error) {
        console.error('Failed to toggle favorite:', error);
    }
}

async function removeTag(tag) {
    if (!currentSessionId) return;

    try {
        await window.go.main.App.RemoveSessionTag(currentSessionId, tag);
        currentMeta.tags = currentMeta.tags.filter(t => t !== tag);
        renderSessionMeta(currentMeta);
        showToast(t('tagRemoved'));
    } catch (error) {
        console.error('Failed to remove tag:', error);
    }
}

function showAddTagDialog() {
    if (!currentSessionId) {
        showToast(t('selectSessionFirst'));
        return;
    }

    const tag = prompt(t('enterTagName'));
    if (!tag) return;

    addTagToSession(tag);
}

async function addTagToSession(tag) {
    if (!currentSessionId || !tag) return;

    try {
        await window.go.main.App.AddSessionTag(currentSessionId, tag);
        if (!currentMeta.tags) currentMeta.tags = [];
        currentMeta.tags.push(tag);
        renderSessionMeta(currentMeta);
        showToast(t('tagAdded'));
    } catch (error) {
        console.error('Failed to add tag:', error);
    }
}

function showNoteDialog() {
    if (!currentSessionId) {
        showToast(t('selectSessionFirst'));
        return;
    }

    const note = prompt(t('enterNote'), currentMeta?.note || '');
    if (note === null) return;

    setSessionNote(note);
}

async function setSessionNote(note) {
    if (!currentSessionId) return;

    try {
        await window.go.main.App.SetSessionNote(currentSessionId, note);
        if (!currentMeta) currentMeta = {};
        currentMeta.note = note;
        showToast(t('noteSaved'));
    } catch (error) {
        console.error('Failed to set note:', error);
    }
}

// 为会话应用自动标签
async function applyAutoTags() {
    if (!currentSessionId) return;

    try {
        const newTags = await window.go.main.App.ApplyAutoTagsToSession(currentSessionId);
        if (newTags && newTags.length > 0) {
            showToast(t('autoTagsApplied').replace('{tags}', newTags.join(', ')));
            // 重新加载元数据
            const meta = await window.go.main.App.GetSessionMeta(currentSessionId);
            currentMeta = meta;
            renderSessionMeta(meta);
        } else {
            showToast(t('noNewAutoTags'));
        }
    } catch (error) {
        console.error('Failed to apply auto tags:', error);
    }
}

// 更新 selectSession 函数以使用带元数据的版本
selectSession = selectSessionWithMeta;

// ============================================
// Utility Functions (Additional)
// ============================================

// 格式化会话开始时间
function formatSessionTime(timeStr) {
    if (!timeStr) return '';
    const date = new Date(timeStr);
    const now = new Date();
    const diff = now - date;

    // 小于 1 小时
    if (diff < 3600000) {
        const minutes = Math.floor(diff / 60000);
        return `${minutes || 1} ${t('minutesAgo') ?? '分钟前'}`;
    }

    // 小于 24 小时
    if (diff < 86400000) {
        const hours = Math.floor(diff / 3600000);
        return `${hours} ${t('hoursAgo') ?? '小时前'}`;
    }

    // 小于 7 天
    if (diff < 604800000) {
        const days = Math.floor(diff / 86400000);
        return `${days} ${t('daysAgo') ?? '天前'}`;
    }

    // 超过 7 天，显示日期
    return date.toLocaleDateString();
}
