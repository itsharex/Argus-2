// @ts-check
// Plugin Studio — 插件工作室主逻辑

// ============================================
// State
// ============================================

let pluginSettings = { hooks: [], mcpServers: [] };
let hookTemplates = [];
let currentProjectDir = '';
let isPluginDirty = false;

// ============================================
// Initialization
// ============================================

/**
 * 初始化插件工作室
 */
async function initPluginStudio() {
    try {
        // 获取当前项目目录
        currentProjectDir = await getCurrentProjectDir();

        // 加载配置
        await loadPluginSettings();
        await loadHookTemplates();

        // 渲染 UI
        renderHooksList();
        renderMCPServersList();
        renderTemplatesList();
    } catch (error) {
        console.error('Failed to initialize plugin studio:', error);
        showToast(t('initFailed') || '初始化失败');
    }
}

/**
 * 获取当前项目目录
 */
async function getCurrentProjectDir() {
    try {
        // 尝试从 URL 或全局状态获取
        if (typeof window.currentProjectDir !== 'undefined') {
            return window.currentProjectDir;
        }
        return '';
    } catch (error) {
        return '';
    }
}

// ============================================
// Data Loading
// ============================================

/**
 * 加载插件配置
 */
async function loadPluginSettings() {
    try {
        pluginSettings = await window.go.main.App.GetPluginSettings(currentProjectDir);
        isPluginDirty = false;
        updatePluginSaveButtonState();
    } catch (error) {
        console.error('Failed to load plugin settings:', error);
        showToast(t('loadFailed') || '加载失败');
    }
}

/**
 * 加载 Hook 模板
 */
async function loadHookTemplates() {
    try {
        hookTemplates = await window.go.main.App.GetHookTemplates();
    } catch (error) {
        console.error('Failed to load hook templates:', error);
    }
}

// ============================================
// Hooks Management
// ============================================

/**
 * 渲染 Hooks 列表
 */
function renderHooksList() {
    const container = document.getElementById('hooksList');
    if (!container) return;

    if (!pluginSettings.hooks || pluginSettings.hooks.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <div class="empty-icon">🔗</div>
                <div class="empty-text">${t('noHooks') || '暂无 Hook 配置'}</div>
                <div class="empty-hint">${t('addHookHint') || '点击上方按钮添加 Hook'}</div>
            </div>
        `;
        return;
    }

    container.innerHTML = pluginSettings.hooks.map((hook, idx) => `
        <div class="config-card ${hook.enabled ? '' : 'disabled'}" data-index="${idx}">
            <div class="card-header">
                <div class="card-title">
                    <span class="hook-type-badge">${escapeHtml(hook.type)}</span>
                    <span class="hook-matcher">${escapeHtml(hook.matcher)}</span>
                </div>
                <div class="card-actions">
                    <button class="icon-btn" onclick="toggleHookEnabled(${idx})" title="${hook.enabled ? t('disable') : t('enable')}">
                        ${hook.enabled ? '✓' : '○'}
                    </button>
                    <button class="icon-btn" onclick="editHook(${idx})" title="${t('edit')}">✏️</button>
                    <button class="icon-btn danger" onclick="removeHook(${idx})" title="${t('delete')}">🗑️</button>
                </div>
            </div>
            <div class="card-body">
                <div class="hook-commands">
                    ${hook.commands.map(cmd => `<code>${escapeHtml(cmd)}</code>`).join('')}
                </div>
            </div>
        </div>
    `).join('');
}

/**
 * 切换 Hook 启用状态
 */
async function toggleHookEnabled(index) {
    if (index < 0 || index >= pluginSettings.hooks.length) return;

    pluginSettings.hooks[index].enabled = !pluginSettings.hooks[index].enabled;
    isPluginDirty = true;
    updatePluginSaveButtonState();
    renderHooksList();
}

/**
 * 编辑 Hook
 */
function editHook(index) {
    if (index < 0 || index >= pluginSettings.hooks.length) return;

    const hook = pluginSettings.hooks[index];
    openHookEditor(hook, index);
}

/**
 * 删除 Hook
 */
async function removeHook(index) {
    if (index < 0 || index >= pluginSettings.hooks.length) return;

    if (!confirm(t('confirmDeleteConfig') || '确定要删除这个 Hook 吗？')) {
        return;
    }

    try {
        await window.go.main.App.RemovePluginHook(currentProjectDir, index);
        await loadPluginSettings();
        renderHooksList();
        showToast(t('hookDeleted') || 'Hook 已删除');
    } catch (error) {
        console.error('Failed to remove hook:', error);
        showToast(t('deleteFailed') || '删除失败');
    }
}

/**
 * 打开 Hook 编辑器
 */
function openHookEditor(hook = null, index = -1) {
    const modal = document.getElementById('hookEditorModal');
    const form = document.getElementById('hookForm');
    const title = document.getElementById('hookEditorTitle');

    if (!modal || !form) return;

    // 设置标题
    if (title) {
        title.textContent = hook ? (t('editHook') || '编辑 Hook') : (t('addHook') || '添加 Hook');
    }

    // 填充表单
    if (hook) {
        form.elements.type.value = hook.type;
        form.elements.matcher.value = hook.matcher;
        form.elements.commands.value = hook.commands.join('\n');
        form.elements.enabled.checked = hook.enabled;
    } else {
        form.reset();
        form.elements.enabled.checked = true;
    }

    // 存储当前编辑的索引
    modal.dataset.editIndex = index;

    // 显示模态框
    modal.style.display = 'flex';
}

/**
 * 关闭 Hook 编辑器
 */
function closeHookEditor() {
    const modal = document.getElementById('hookEditorModal');
    if (modal) {
        modal.style.display = 'none';
    }
}

/**
 * 保存 Hook
 */
async function saveHook() {
    const form = document.getElementById('hookForm');
    const modal = document.getElementById('hookEditorModal');
    if (!form || !modal) return;

    const hook = {
        type: form.elements.type.value,
        matcher: form.elements.matcher.value,
        commands: form.elements.commands.value.split('\n').filter(c => c.trim()),
        enabled: form.elements.enabled.checked,
    };

    // 验证
    if (!hook.matcher.trim()) {
        showToast(t('matcherRequired') || '请输入匹配模式', 'error');
        return;
    }

    if (hook.commands.length === 0) {
        showToast(t('commandsRequired') || '请输入至少一个命令', 'error');
        return;
    }

    try {
        const index = parseInt(modal.dataset.editIndex, 10);
        if (index >= 0) {
            // 更新现有 Hook
            await window.go.main.App.UpdatePluginHook(currentProjectDir, index, hook);
        } else {
            // 添加新 Hook
            await window.go.main.App.AddPluginHook(currentProjectDir, hook);
        }

        await loadPluginSettings();
        renderHooksList();
        closeHookEditor();
        showToast(t('hookSaved') || 'Hook 已保存');
    } catch (error) {
        console.error('Failed to save hook:', error);
        showToast(t('saveFailed') || '保存失败');
    }
}

// ============================================
// MCP Servers Management
// ============================================

/**
 * 渲染 MCP 服务器列表
 */
function renderMCPServersList() {
    const container = document.getElementById('mcpServersList');
    if (!container) return;

    if (!pluginSettings.mcpServers || pluginSettings.mcpServers.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <div class="empty-icon">🔌</div>
                <div class="empty-text">${t('noMCPServers') || '暂无 MCP 服务器'}</div>
                <div class="empty-hint">${t('addMCPHint') || '点击上方按钮添加 MCP 服务器'}</div>
            </div>
        `;
        return;
    }

    container.innerHTML = pluginSettings.mcpServers.map((server, idx) => `
        <div class="config-card ${server.enabled ? '' : 'disabled'}" data-index="${idx}">
            <div class="card-header">
                <div class="card-title">
                    <span class="mcp-transport-badge">${escapeHtml(server.transport)}</span>
                    <span class="mcp-name">${escapeHtml(server.name)}</span>
                </div>
                <div class="card-actions">
                    <button class="icon-btn" onclick="toggleMCPServerEnabled(${idx})" title="${server.enabled ? t('disable') : t('enable')}">
                        ${server.enabled ? '✓' : '○'}
                    </button>
                    <button class="icon-btn" onclick="editMCPServer(${idx})" title="${t('edit')}">✏️</button>
                    <button class="icon-btn danger" onclick="removeMCPServer(${idx})" title="${t('delete')}">🗑️</button>
                </div>
            </div>
            <div class="card-body">
                <div class="mcp-details">
                    ${server.transport === 'stdio'
                        ? `<code>${escapeHtml(server.command)} ${escapeHtml((server.args || []).join(' '))}</code>`
                        : `<code>${escapeHtml(server.url)}</code>`
                    }
                </div>
            </div>
        </div>
    `).join('');
}

/**
 * 切换 MCP 服务器启用状态
 */
async function toggleMCPServerEnabled(index) {
    if (index < 0 || index >= pluginSettings.mcpServers.length) return;

    pluginSettings.mcpServers[index].enabled = !pluginSettings.mcpServers[index].enabled;
    isPluginDirty = true;
    updatePluginSaveButtonState();
    renderMCPServersList();
}

/**
 * 编辑 MCP 服务器
 */
function editMCPServer(index) {
    if (index < 0 || index >= pluginSettings.mcpServers.length) return;

    const server = pluginSettings.mcpServers[index];
    openMCPServerEditor(server, index);
}

/**
 * 删除 MCP 服务器
 */
async function removeMCPServer(index) {
    if (index < 0 || index >= pluginSettings.mcpServers.length) return;

    if (!confirm(t('confirmDeleteConfig') || '确定要删除这个 MCP 服务器吗？')) {
        return;
    }

    try {
        await window.go.main.App.RemoveMCPServer(currentProjectDir, index);
        await loadPluginSettings();
        renderMCPServersList();
        showToast(t('mcpDeleted') || 'MCP 服务器已删除');
    } catch (error) {
        console.error('Failed to remove MCP server:', error);
        showToast(t('deleteFailed') || '删除失败');
    }
}

/**
 * 打开 MCP 服务器编辑器
 */
function openMCPServerEditor(server = null, index = -1) {
    const modal = document.getElementById('mcpEditorModal');
    const form = document.getElementById('mcpForm');
    const title = document.getElementById('mcpEditorTitle');

    if (!modal || !form) return;

    // 设置标题
    if (title) {
        title.textContent = server ? (t('editMCP') || '编辑 MCP 服务器') : (t('addMCP') || '添加 MCP 服务器');
    }

    // 填充表单
    if (server) {
        form.elements.name.value = server.name;
        form.elements.transport.value = server.transport;
        form.elements.command.value = server.command || '';
        form.elements.url.value = server.url || '';
        form.elements.args.value = (server.args || []).join('\n');
        form.elements.enabled.checked = server.enabled;

        // 设置环境变量
        const envContainer = document.getElementById('mcpEnvContainer');
        if (envContainer) {
            envContainer.innerHTML = '';
            if (server.env) {
                Object.entries(server.env).forEach(([key, value]) => {
                    addEnvVarRow(key, value);
                });
            }
        }
    } else {
        form.reset();
        form.elements.transport.value = 'stdio';
        form.elements.enabled.checked = true;

        // 清空环境变量
        const envContainer = document.getElementById('mcpEnvContainer');
        if (envContainer) {
            envContainer.innerHTML = '';
        }
    }

    // 更新传输类型 UI
    onTransportTypeChange();

    // 存储当前编辑的索引
    modal.dataset.editIndex = index;

    // 显示模态框
    modal.style.display = 'flex';
}

/**
 * 关闭 MCP 服务器编辑器
 */
function closeMCPServerEditor() {
    const modal = document.getElementById('mcpEditorModal');
    if (modal) {
        modal.style.display = 'none';
    }
}

/**
 * 保存 MCP 服务器
 */
async function saveMCPServer() {
    const form = document.getElementById('mcpForm');
    const modal = document.getElementById('mcpEditorModal');
    if (!form || !modal) return;

    const transport = form.elements.transport.value;
    const env = {};

    // 收集环境变量
    document.querySelectorAll('.env-var-row').forEach(row => {
        const key = row.querySelector('.env-key').value.trim();
        const value = row.querySelector('.env-value').value.trim();
        if (key) {
            env[key] = value;
        }
    });

    const server = {
        name: form.elements.name.value.trim(),
        transport: transport,
        command: transport === 'stdio' ? form.elements.command.value.trim() : '',
        url: transport !== 'stdio' ? form.elements.url.value.trim() : '',
        args: form.elements.args.value.split('\n').filter(a => a.trim()),
        env: env,
        enabled: form.elements.enabled.checked,
    };

    // 验证
    if (!server.name) {
        showToast(t('nameRequired') || '请输入服务器名称', 'error');
        return;
    }

    if (transport === 'stdio' && !server.command) {
        showToast(t('commandRequired') || '请输入命令', 'error');
        return;
    }

    if (transport !== 'stdio' && !server.url) {
        showToast(t('urlRequired') || '请输入 URL', 'error');
        return;
    }

    try {
        const index = parseInt(modal.dataset.editIndex, 10);
        if (index >= 0) {
            // 更新现有服务器
            await window.go.main.App.UpdateMCPServer(currentProjectDir, index, server);
        } else {
            // 添加新服务器
            await window.go.main.App.AddMCPServer(currentProjectDir, server);
        }

        await loadPluginSettings();
        renderMCPServersList();
        closeMCPServerEditor();
        showToast(t('mcpSaved') || 'MCP 服务器已保存');
    } catch (error) {
        console.error('Failed to save MCP server:', error);
        showToast(t('saveFailed') || '保存失败');
    }
}

/**
 * 传输类型变更处理
 */
function onTransportTypeChange() {
    const transport = document.getElementById('mcpTransport')?.value;
    const stdioFields = document.getElementById('stdioFields');
    const httpFields = document.getElementById('httpFields');

    if (stdioFields) {
        stdioFields.style.display = transport === 'stdio' ? 'block' : 'none';
    }
    if (httpFields) {
        httpFields.style.display = transport !== 'stdio' ? 'block' : 'none';
    }
}

/**
 * 添加环境变量行
 */
function addEnvVarRow(key = '', value = '') {
    const container = document.getElementById('mcpEnvContainer');
    if (!container) return;

    const row = document.createElement('div');
    row.className = 'env-var-row';
    row.innerHTML = `
        <input type="text" class="env-key" placeholder="${t('envKey') || '变量名'}" value="${escapeHtml(key)}">
        <input type="text" class="env-value" placeholder="${t('envValue') || '值'}" value="${escapeHtml(value)}">
        <button type="button" class="icon-btn danger" onclick="removeEnvVarRow(this)">×</button>
    `;
    container.appendChild(row);
}

/**
 * 删除环境变量行
 */
function removeEnvVarRow(btn) {
    const row = btn.closest('.env-var-row');
    if (row) {
        row.remove();
    }
}

// ============================================
// Templates Management
// ============================================

/**
 * 渲染模板列表
 */
function renderTemplatesList() {
    const container = document.getElementById('templatesList');
    if (!container) return;

    if (!hookTemplates || hookTemplates.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <div class="empty-icon">📦</div>
                <div class="empty-text">${t('noTemplates') || '暂无模板'}</div>
            </div>
        `;
        return;
    }

    // 按分类分组
    const categories = {};
    hookTemplates.forEach(template => {
        if (!categories[template.category]) {
            categories[template.category] = [];
        }
        categories[template.category].push(template);
    });

    container.innerHTML = Object.entries(categories).map(([category, templates]) => `
        <div class="template-category">
            <h4 class="category-title">${escapeHtml(category)}</h4>
            <div class="template-grid">
                ${templates.map(template => `
                    <div class="template-card" onclick="applyTemplate('${escapeHtml(template.name)}')">
                        <div class="template-name">${escapeHtml(template.name)}</div>
                        <div class="template-desc">${escapeHtml(template.description)}</div>
                        <div class="template-hook-type">
                            <span class="hook-type-badge small">${escapeHtml(template.hook.type)}</span>
                            <span class="template-matcher">${escapeHtml(template.hook.matcher)}</span>
                        </div>
                    </div>
                `).join('')}
            </div>
        </div>
    `).join('');
}

/**
 * 应用模板
 */
async function applyTemplate(templateName) {
    const template = hookTemplates.find(t => t.name === templateName);
    if (!template) return;

    if (!confirm(t('confirmApply') || '确定要应用这个模板吗？')) {
        return;
    }

    try {
        await window.go.main.App.ApplyHookTemplate(currentProjectDir, template);
        await loadPluginSettings();
        renderHooksList();
        showToast(t('templateApplied') || '模板已应用');
    } catch (error) {
        console.error('Failed to apply template:', error);
        showToast(t('applyFailed') || '应用失败');
    }
}

// ============================================
// Save Operations
// ============================================

/**
 * 保存所有插件配置
 */
async function saveAllPluginSettings() {
    try {
        await window.go.main.App.SavePluginSettings(currentProjectDir, pluginSettings);
        isPluginDirty = false;
        updatePluginSaveButtonState();
        showToast(t('settingsSaved') || '配置已保存');
    } catch (error) {
        console.error('Failed to save plugin settings:', error);
        showToast(t('saveFailed') || '保存失败');
    }
}

/**
 * 更新保存按钮状态
 */
function updatePluginSaveButtonState() {
    const saveBtn = document.getElementById('pluginSaveBtn');
    if (saveBtn) {
        saveBtn.disabled = !isPluginDirty;
        saveBtn.style.opacity = isPluginDirty ? '1' : '0.5';
    }
}

// ============================================
// Validation
// ============================================

/**
 * 验证配置
 */
async function validatePluginSettings() {
    try {
        const errors = await window.go.main.App.ValidatePluginSettings(pluginSettings);
        if (errors.length > 0) {
            showToast(`${t('validationErrors') || '验证错误'}: ${errors[0].message}`, 'error');
        } else {
            showToast(t('validationPassed') || '验证通过');
        }
    } catch (error) {
        console.error('Failed to validate settings:', error);
    }
}
