// @ts-check
// Agent Tree Viewer — Agent 调用树可视化（基于 Agent 聚合，而非逐消息）

// ============================================
// State
// ============================================

let agentTreeData = null;
let selectedAgentId = null;
let expandedNodes = new Set();

// ============================================
// Load Agent Tree
// ============================================

/**
 * 加载 Agent 调用树
 * @param {string} sessionID - 会话 ID
 */
async function loadAgentTree(sessionID) {
    const container = document.getElementById('agentTreeContent');
    if (!container) return;

    container.innerHTML = `<div class="agent-tree-empty"><p>${t('loadingAgentTree') || '加载 Agent 树...'}</p></div>`;

    try {
        agentTreeData = await window.go.main.App.GetSessionAgentTree(sessionID);
        renderAgentTree(agentTreeData);
    } catch (error) {
        console.error('Failed to load agent tree:', error);
        container.innerHTML = `<div class="agent-tree-empty"><p>${t('loadFailed')}: ${escapeHtml(String(error))}</p></div>`;
    }
}

// ============================================
// Render Agent Tree
// ============================================

/**
 * 渲染 Agent 树
 * @param {object} tree - AgentTree 数据
 */
function renderAgentTree(tree) {
    const container = document.getElementById('agentTreeContent');
    if (!container) return;

    // 更新统计信息
    const statsEl = document.getElementById('agentTreeStats');
    if (statsEl && tree) {
        statsEl.innerHTML = `
            <span>${t('totalAgents') || '总 Agents'}: <span class="stat-value">${tree.totalAgents || 0}</span></span>
            <span>${t('maxDepth') || '最大深度'}: <span class="stat-value">${tree.maxDepth || 0}</span></span>
        `;
    }

    const roots = tree.roots || (tree.root ? [tree.root] : []);

    if (!roots || roots.length === 0) {
        container.innerHTML = `
            <div class="agent-tree-empty">
                <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <path d="M17 21v-2a4 4 0 0 0-4-4H5a4 4 0 0 0-4 4v2"/>
                    <circle cx="9" cy="7" r="4"/>
                    <path d="M23 21v-2a4 4 0 0 0-3-3.87"/>
                    <path d="M16 3.13a4 4 0 0 1 0 7.75"/>
                </svg>
                <p>${t('noAgentActivity') || '无 Agent 活动数据'}</p>
            </div>
        `;
        return;
    }

    // 默认展开所有根节点（第一层）
    expandedNodes.clear();
    for (const root of roots) {
        expandedNodes.add(root.uuid);
    }

    // 渲染所有根节点
    container.innerHTML = roots.map(root => renderNode(root)).join('');
    bindNodeEvents();
}

/**
 * 渲染单个 Agent 节点
 * @param {object} node - AgentNode
 * @returns {string} HTML 字符串
 */
function renderNode(node) {
    if (!node) return '';

    const hasChildren = node.children && node.children.length > 0;
    const isExpanded = expandedNodes.has(node.uuid);
    const isSelected = selectedAgentId === node.agentId || selectedAgentId === node.uuid;

    // 节点名称：优先使用 Name 字段，回退到 type 判断
    let name = node.name || '';
    if (!name) {
        if (node.isSubAgent) {
            name = t('subAgent') || '子 Agent';
        } else if (node.type === 'user') {
            name = t('userMessage') || '用户消息';
        } else {
            name = t('mainAgent') || '主 Agent';
        }
    }

    // 显示短 AgentID
    let shortId = '';
    if (node.agentId) {
        shortId = node.agentId.length > 12 ? node.agentId.substring(0, 12) + '...' : node.agentId;
    }

    // 状态
    const status = node.status || 'completed';

    // 格式化时长
    const duration = formatDuration(node.duration);

    // 格式化 token
    const totalTokens = (node.inputTokens || 0) + (node.outputTokens || 0);

    // 节点类型标签
    const typeLabel = node.isSubAgent ? (t('subAgent') || '子 Agent') : (t('mainAgent') || '主 Agent');

    let html = `
        <div class="agent-node" data-uuid="${escapeHtmlAttr(node.uuid)}" data-agent-id="${escapeHtmlAttr(node.agentId || '')}">
            <div class="agent-node-card ${isSelected ? 'selected' : ''} ${node.isSubAgent ? 'sub-agent' : 'main-agent'}" onclick="selectAgentNode('${escapeHtmlAttr(node.uuid)}', '${escapeHtmlAttr(node.agentId || '')}')">
                ${hasChildren ? `
                    <span class="agent-node-toggle ${isExpanded ? 'expanded' : ''}" onclick="event.stopPropagation(); toggleAgentNode('${escapeHtmlAttr(node.uuid)}')">
                        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M9 18l6-6-6-6"/>
                        </svg>
                    </span>
                ` : '<span style="width: 16px;"></span>'}
                <span class="agent-status-dot ${status}"></span>
                <div class="agent-node-info">
                    <div class="agent-node-name">${escapeHtml(name)}</div>
                    <div class="agent-node-type">
                        <span class="agent-type-badge ${node.isSubAgent ? 'sub' : 'main'}">${escapeHtml(typeLabel)}</span>
                        ${shortId ? `<span class="agent-id-hint">${escapeHtml(shortId)}</span>` : ''}
                    </div>
                </div>
                <span class="agent-depth-badge">L${node.depth || 0}</span>
                <div class="agent-node-metrics">
                    <span class="agent-metric" title="${t('messageCount') || '消息数'}">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/></svg>
                        ${node.messageCount || 0}
                    </span>
                    <span class="agent-metric" title="${t('inputTokens') || '输入 Token'} / ${t('outputTokens') || '输出 Token'}">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M12 2v20M17 5H9.5a3.5 3.5 0 0 0 0 7h5a3.5 3.5 0 0 1 0 7H6"/></svg>
                        ${formatNumber(totalTokens)}
                    </span>
                    <span class="agent-metric" title="${t('toolCalls') || '工具调用'}">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/></svg>
                        ${node.toolCalls || 0}
                    </span>
                    <span class="agent-metric" title="${t('duration') || '耗时'}">
                        <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><circle cx="12" cy="12" r="10"/><polyline points="12 6 12 12 16 14"/></svg>
                        ${duration}
                    </span>
                </div>
            </div>
    `;

    // 详情面板
    if (isSelected) {
        html += `
            <div class="agent-detail-panel">
                <div class="agent-detail-row">
                    <span class="agent-detail-label">${t('agentId') || 'Agent ID'}</span>
                    <span class="agent-detail-value">${escapeHtml(node.agentId || '-')}</span>
                </div>
                <div class="agent-detail-row">
                    <span class="agent-detail-label">${t('name') || '名称'}</span>
                    <span class="agent-detail-value">${escapeHtml(name)}</span>
                </div>
                <div class="agent-detail-row">
                    <span class="agent-detail-label">${t('uuid') || 'UUID'}</span>
                    <span class="agent-detail-value">${escapeHtml(node.uuid || '-')}</span>
                </div>
                <div class="agent-detail-row">
                    <span class="agent-detail-label">${t('messageCount') || '消息数'}</span>
                    <span class="agent-detail-value">${node.messageCount || 0}</span>
                </div>
                <div class="agent-detail-row">
                    <span class="agent-detail-label">${t('inputTokens') || '输入 Token'}</span>
                    <span class="agent-detail-value">${formatNumber(node.inputTokens || 0)}</span>
                </div>
                <div class="agent-detail-row">
                    <span class="agent-detail-label">${t('outputTokens') || '输出 Token'}</span>
                    <span class="agent-detail-value">${formatNumber(node.outputTokens || 0)}</span>
                </div>
                <div class="agent-detail-row">
                    <span class="agent-detail-label">${t('toolCalls') || '工具调用'}</span>
                    <span class="agent-detail-value">${node.toolCalls || 0}</span>
                </div>
                <div class="agent-detail-row">
                    <span class="agent-detail-label">${t('depth') || '深度'}</span>
                    <span class="agent-detail-value">${node.depth || 0}</span>
                </div>
                <div class="agent-detail-row">
                    <span class="agent-detail-label">${t('status') || '状态'}</span>
                    <span class="agent-detail-value">${status}</span>
                </div>
            </div>
        `;
    }

    // 子节点
    if (hasChildren && isExpanded) {
        html += `<div class="agent-node-children">`;
        for (const child of node.children) {
            html += renderNode(child);
        }
        html += `</div>`;
    }

    html += `</div>`;
    return html;
}

// ============================================
// Interaction
// ============================================

/**
 * 选择 Agent 节点
 * @param {string} uuid - 节点 UUID
 * @param {string} agentId - Agent ID
 */
function selectAgentNode(uuid, agentId) {
    selectedAgentId = agentId || uuid;
    // 重新加载树以更新选中状态
    if (agentTreeData) {
        renderAgentTree(agentTreeData);
    }
}

/**
 * 展开/折叠节点
 * @param {string} uuid - 节点 UUID
 */
function toggleAgentNode(uuid) {
    if (expandedNodes.has(uuid)) {
        expandedNodes.delete(uuid);
    } else {
        expandedNodes.add(uuid);
    }
    // 重新渲染（不需要重新加载数据）
    if (agentTreeData) {
        renderAgentTree(agentTreeData);
    }
}

// ============================================
// Helpers
// ============================================

/**
 * 格式化时长
 * @param {number} ms - 毫秒
 * @returns {string}
 */
function formatDuration(ms) {
    if (!ms || ms <= 0) return '0s';
    if (ms < 1000) return ms + 'ms';
    if (ms < 60000) return (ms / 1000).toFixed(1) + 's';
    const minutes = Math.floor(ms / 60000);
    const seconds = Math.floor((ms % 60000) / 1000);
    return `${minutes}m ${seconds}s`;
}

/**
 * 绑定节点事件
 */
function bindNodeEvents() {
    // 事件已在 HTML 中通过 onclick 绑定
}
