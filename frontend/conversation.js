// @ts-check
// Conversation Viewer — 对话记录查看器

// ============================================
// State
// ============================================

let conversationMessages = [];
let isLoadingConversation = false;
let currentSessionID = '';
let currentSessionTab = 'conversation';

// ============================================
// Tab Switching
// ============================================

/**
 * 切换会话标签页
 * @param {string} tabName - 标签页名称 ('conversation', 'files')
 */
function switchSessionTab(tabName) {
    currentSessionTab = tabName;

    // 更新标签按钮状态
    document.querySelectorAll('.session-tab').forEach(tab => {
        tab.classList.toggle('active', tab.getAttribute('data-tab') === tabName);
    });

    // 更新标签内容显示
    document.querySelectorAll('.session-tab-content').forEach(content => {
        content.style.display = 'none';
        content.classList.remove('active');
    });

    const targetTab = document.getElementById(tabName + 'Tab');
    if (targetTab) {
        targetTab.style.display = 'flex';
        targetTab.classList.add('active');
    }
}

// ============================================
// Load Conversation
// ============================================

/**
 * 加载对话记录
 * @param {string} sessionID - 会话 ID
 */
async function loadConversation(sessionID) {
    if (isLoadingConversation) return;

    currentSessionID = sessionID;
    isLoadingConversation = true;

    const container = document.getElementById('conversationList');
    const countEl = document.getElementById('conversationTabCount');

    if (container) {
        container.innerHTML = `<div class="empty-state"><p>${t('loadingConversation') || '加载对话记录...'}</p></div>`;
    }

    try {
        const messages = await window.go.main.App.GetSessionMessages(sessionID);
        conversationMessages = messages || [];

        if (countEl) {
            countEl.textContent = conversationMessages.length;
        }

        renderConversation(conversationMessages);
    } catch (error) {
        console.error('Failed to load conversation:', error);
        if (container) {
            container.innerHTML = `<div class="empty-state"><p>${t('noConversation') || '暂无对话记录'}</p></div>`;
        }
    } finally {
        isLoadingConversation = false;
    }
}

// ============================================
// Render Conversation
// ============================================

/**
 * 渲染对话记录
 * @param {Array} messages - 消息列表
 */
function renderConversation(messages) {
    const container = document.getElementById('conversationList');
    if (!container) return;

    if (!messages || messages.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <path d="M21 15a2 2 0 0 1-2 2H7l-4 4V5a2 2 0 0 1 2-2h14a2 2 0 0 1 2 2z"/>
                </svg>
                <p>${t('noConversation') || '暂无对话记录'}</p>
            </div>
        `;
        return;
    }

    container.innerHTML = messages.map(msg => renderMessage(msg)).join('');
}

/**
 * 渲染单条消息
 * @param {object} msg - 消息对象
 * @returns {string} HTML 字符串
 */
function renderMessage(msg) {
    const time = formatConversationTime(msg.timestamp);

    switch (msg.type) {
        case 'user':
            return renderUserMessage(msg, time);
        case 'assistant':
            return renderAssistantMessage(msg, time);
        default:
            return '';
    }
}

/**
 * 渲染用户消息
 * @param {object} msg - 消息对象
 * @param {string} time - 格式化后的时间
 * @returns {string} HTML 字符串
 */
function renderUserMessage(msg, time) {
    const textBlocks = (msg.content || []).filter(c => c.type === 'text');
    const text = textBlocks.map(c => c.text).join('\n');

    // 检查是否有 tool_result
    const toolResults = (msg.content || []).filter(c => c.type === 'tool_result');

    let html = `
        <div class="msg-user">
            <div class="msg-header">
                <span class="msg-role">${t('userMessage') || '用户'}</span>
                <span class="msg-time">${time}</span>
            </div>
            <div class="msg-body">${escapeHtml(text)}</div>
        </div>
    `;

    // 渲染工具结果
    for (const result of toolResults) {
        html += `
            <div class="msg-tool-result">
                <div class="msg-header">
                    <span class="msg-role">${t('toolResult') || '工具'}</span>
                    <span class="msg-time">${time}</span>
                </div>
                <div class="msg-body"><pre>${escapeHtml(result.result)}</pre></div>
            </div>
        `;
    }

    return html;
}

/**
 * 渲染 AI 消息
 * @param {object} msg - 消息对象
 * @param {string} time - 格式化后的时间
 * @returns {string} HTML 字符串
 */
function renderAssistantMessage(msg, time) {
    const blocks = (msg.content || []).map(block => {
        switch (block.type) {
            case 'thinking':
                return `<div class="msg-thinking"><span class="thinking-label">${t('thinking') || '思考过程'}</span>${escapeHtml(block.thinking)}</div>`;
            case 'text':
                return `<div class="msg-text">${escapeHtml(block.text)}</div>`;
            case 'tool_use':
                return `<div class="msg-tool-use"><span class="tool-label">[Tool: ${escapeHtml(block.toolName)}]</span></div>`;
            default:
                return '';
        }
    }).filter(Boolean).join('');

    return `
        <div class="msg-assistant">
            <div class="msg-header">
                <span class="msg-role">${t('aiMessage') || 'AI'}</span>
                <span class="msg-time">${time}</span>
            </div>
            <div class="msg-body">${blocks}</div>
        </div>
    `;
}

// ============================================
// Utility
// ============================================

/**
 * 格式化对话时间
 * @param {string} timeStr - ISO 时间字符串
 * @returns {string} 格式化后的时间
 */
function formatConversationTime(timeStr) {
    if (!timeStr) return '';
    const date = new Date(timeStr);
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    const hours = String(date.getHours()).padStart(2, '0');
    const minutes = String(date.getMinutes()).padStart(2, '0');
    const seconds = String(date.getSeconds()).padStart(2, '0');
    return `${year}/${month}/${day} ${hours}:${minutes}:${seconds}`;
}

