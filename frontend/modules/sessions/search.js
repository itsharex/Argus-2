// @ts-check
// Global Search — 跨项目全局搜索（Ctrl+K）

// ============================================
// State
// ============================================

let searchOverlayActive = false;
let searchResults = [];
let searchSelectedIndex = -1;
let searchDebounceTimer = null;
const SEARCH_HISTORY_KEY = 'argus_search_history';
const SEARCH_HISTORY_MAX = 20;

// ============================================
// Open / Close
// ============================================

/**
 * 打开全局搜索面板
 */
function openGlobalSearch() {
    const overlay = document.getElementById('searchOverlay');
    if (!overlay) return;

    searchOverlayActive = true;
    overlay.classList.add('active');

    const input = document.getElementById('globalSearchInput');
    if (input) {
        input.value = '';
        input.focus();
    }

    // 清空结果
    searchResults = [];
    searchSelectedIndex = -1;
    renderSearchResults('');
}

/**
 * 关闭全局搜索面板
 */
function closeGlobalSearch() {
    const overlay = document.getElementById('searchOverlay');
    if (!overlay) return;

    searchOverlayActive = false;
    overlay.classList.remove('active');

    // 清除防抖定时器
    if (searchDebounceTimer) {
        clearTimeout(searchDebounceTimer);
        searchDebounceTimer = null;
    }
}

/**
 * 切换搜索面板
 */
function toggleGlobalSearch() {
    if (searchOverlayActive) {
        closeGlobalSearch();
    } else {
        openGlobalSearch();
    }
}

// ============================================
// Search Logic
// ============================================

/**
 * 执行搜索（带防抖）
 * @param {string} keyword - 搜索关键词
 */
function onSearchInput(keyword) {
    if (searchDebounceTimer) {
        clearTimeout(searchDebounceTimer);
    }

    if (!keyword || keyword.trim().length < 2) {
        searchResults = [];
        searchSelectedIndex = -1;
        renderSearchResults(keyword);
        return;
    }

    searchDebounceTimer = setTimeout(() => {
        performSearch(keyword.trim());
    }, 300);
}

/**
 * 执行搜索
 * @param {string} keyword - 搜索关键词
 */
async function performSearch(keyword) {
    const resultsEl = document.getElementById('searchResults');
    if (!resultsEl) return;

    // 显示加载状态
    resultsEl.innerHTML = `
        <div class="search-loading">
            <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                <path d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/>
            </svg>
            ${t('searching') || '搜索中...'}
        </div>
    `;

    try {
        const req = {
            keyword: keyword,
            limit: 50,
        };
        searchResults = await window.go.main.App.GlobalSearch(req);
        searchSelectedIndex = searchResults.length > 0 ? 0 : -1;
        renderSearchResults(keyword);

        // 保存搜索历史
        saveSearchHistory(keyword);
    } catch (error) {
        console.error('Search failed:', error);
        resultsEl.innerHTML = `
            <div class="search-empty">
                <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <circle cx="12" cy="12" r="10"/>
                    <line x1="15" y1="9" x2="9" y2="15"/>
                    <line x1="9" y1="9" x2="15" y2="15"/>
                </svg>
                <p>${t('searchFailed') || '搜索失败'}: ${escapeHtml(String(error))}</p>
            </div>
        `;
    }
}

// ============================================
// Render
// ============================================

/**
 * 渲染搜索结果
 * @param {string} keyword - 当前搜索关键词（用于高亮）
 */
function renderSearchResults(keyword) {
    const resultsEl = document.getElementById('searchResults');
    if (!resultsEl) return;

    if (!keyword || keyword.trim().length < 2) {
        // 显示搜索历史或空状态
        const history = getSearchHistory();
        if (history.length > 0) {
            resultsEl.innerHTML = `
                <div class="search-empty" style="padding: 16px 20px; align-items: flex-start;">
                    <div style="font-size: 12px; color: var(--text-secondary); margin-bottom: 8px;">${t('searchHistory') || '搜索历史'}</div>
                    ${history.map(h => `
                        <div class="search-result-item" onclick="document.getElementById('globalSearchInput').value='${escapeHtmlAttr(h)}'; performSearch('${escapeHtmlAttr(h)}');" style="padding: 6px 12px;">
                            <span style="font-size: 13px; color: var(--text-primary);">${escapeHtml(h)}</span>
                        </div>
                    `).join('')}
                </div>
            `;
        } else {
            resultsEl.innerHTML = `
                <div class="search-empty">
                    <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                        <circle cx="11" cy="11" r="8"/>
                        <path d="M21 21l-4.35-4.35"/>
                    </svg>
                    <p>${t('globalSearchHint') || '输入关键词搜索所有会话内容'}</p>
                </div>
            `;
        }
        return;
    }

    if (searchResults.length === 0) {
        resultsEl.innerHTML = `
            <div class="search-empty">
                <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <circle cx="11" cy="11" r="8"/>
                    <path d="M21 21l-4.35-4.35"/>
                </svg>
                <p>${t('noSearchResults') || '未找到匹配结果'}</p>
            </div>
        `;
        return;
    }

    resultsEl.innerHTML = searchResults.map((result, index) => {
        const isSelected = index === searchSelectedIndex;
        const highlighted = highlightKeyword(result.matchContent, keyword);
        const timeStr = result.timestamp ? formatSearchTime(result.timestamp) : '';

        return `
            <div class="search-result-item ${isSelected ? 'selected' : ''}"
                 onclick="onSearchResultClick('${escapeHtmlAttr(result.sessionId)}')"
                 onmouseenter="searchSelectedIndex=${index}; renderSearchResults('${escapeHtmlAttr(keyword)}');">
                <div class="search-result-header">
                    <span class="search-result-type ${result.matchType}">${result.matchType}</span>
                    <span class="search-result-session">${escapeHtml(truncateSessionId(result.sessionId))}</span>
                    <span class="search-result-project">${escapeHtml(result.projectDir)}</span>
                </div>
                <div class="search-result-content">${highlighted}</div>
                ${timeStr ? `<div class="search-result-meta"><span>${timeStr}</span></div>` : ''}
            </div>
        `;
    }).join('');
}

/**
 * 点击搜索结果
 * @param {string} sessionId - 会话 ID
 */
function onSearchResultClick(sessionId) {
    closeGlobalSearch();
    // 切换到会话管理 tab 并加载该会话
    switchTab('sessions');
    // 延迟加载会话，等待 tab 切换完成
    setTimeout(() => {
        if (typeof loadSessionById === 'function') {
            loadSessionById(sessionId);
        } else if (typeof selectSession === 'function') {
            selectSession(sessionId);
        }
    }, 100);
}

// ============================================
// Keyboard Navigation
// ============================================

/**
 * 处理搜索面板键盘事件
 * @param {KeyboardEvent} e
 */
function onSearchKeydown(e) {
    if (!searchOverlayActive) return;

    switch (e.key) {
        case 'Escape':
            e.preventDefault();
            closeGlobalSearch();
            break;
        case 'ArrowDown':
            e.preventDefault();
            if (searchResults.length > 0) {
                searchSelectedIndex = Math.min(searchSelectedIndex + 1, searchResults.length - 1);
                const keyword = document.getElementById('globalSearchInput')?.value || '';
                renderSearchResults(keyword);
                scrollSearchResultIntoView();
            }
            break;
        case 'ArrowUp':
            e.preventDefault();
            if (searchResults.length > 0) {
                searchSelectedIndex = Math.max(searchSelectedIndex - 1, 0);
                const keyword = document.getElementById('globalSearchInput')?.value || '';
                renderSearchResults(keyword);
                scrollSearchResultIntoView();
            }
            break;
        case 'Enter':
            e.preventDefault();
            if (searchSelectedIndex >= 0 && searchSelectedIndex < searchResults.length) {
                const result = searchResults[searchSelectedIndex];
                onSearchResultClick(result.sessionId);
            }
            break;
    }
}

/**
 * 滚动选中的搜索结果到可见区域
 */
function scrollSearchResultIntoView() {
    const container = document.getElementById('searchResults');
    if (!container) return;

    const selected = container.querySelector('.search-result-item.selected');
    if (selected) {
        selected.scrollIntoView({ block: 'nearest' });
    }
}

// ============================================
// Helpers
// ============================================

/**
 * 高亮关键词
 * @param {string} text - 原文
 * @param {string} keyword - 关键词
 * @returns {string} 高亮后的 HTML
 */
function highlightKeyword(text, keyword) {
    if (!text || !keyword) return escapeHtml(text || '');

    const escaped = escapeHtml(text);
    const keywords = keyword.toLowerCase().split(/\s+/).filter(k => k.length >= 2);

    if (keywords.length === 0) return escaped;

    // 构建正则（转义特殊字符）
    const pattern = keywords.map(k => k.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')).join('|');
    const regex = new RegExp(`(${pattern})`, 'gi');

    return escaped.replace(regex, '<mark>$1</mark>');
}

/**
 * 格式化搜索结果时间
 * @param {number} ts - Unix 毫秒
 * @returns {string}
 */
function formatSearchTime(ts) {
    if (!ts) return '';
    const date = new Date(ts);
    const now = new Date();
    const diffMs = now - date;
    const diffMin = Math.floor(diffMs / 60000);
    const diffHour = Math.floor(diffMs / 3600000);
    const diffDay = Math.floor(diffMs / 86400000);

    if (diffMin < 1) return t('justNow') || '刚刚';
    if (diffMin < 60) return `${diffMin}${t('minutesAgo') || '分钟前'}`;
    if (diffHour < 24) return `${diffHour}${t('hoursAgo') || '小时前'}`;
    if (diffDay < 7) return `${diffDay}${t('daysAgo') || '天前'}`;

    return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}-${String(date.getDate()).padStart(2, '0')}`;
}

/**
 * 截断 session ID 用于显示
 * @param {string} sessionId
 * @returns {string}
 */
function truncateSessionId(sessionId) {
    if (!sessionId) return '';
    if (sessionId.length <= 16) return sessionId;
    return sessionId.substring(0, 12) + '...';
}

// ============================================
// Search History (localStorage)
// ============================================

/**
 * 获取搜索历史
 * @returns {string[]}
 */
function getSearchHistory() {
    try {
        const data = localStorage.getItem(SEARCH_HISTORY_KEY);
        return data ? JSON.parse(data) : [];
    } catch {
        return [];
    }
}

/**
 * 保存搜索关键词到历史
 * @param {string} keyword
 */
function saveSearchHistory(keyword) {
    if (!keyword) return;
    try {
        let history = getSearchHistory();
        // 去重（移到最前面）
        history = history.filter(h => h !== keyword);
        history.unshift(keyword);
        // 截断
        if (history.length > SEARCH_HISTORY_MAX) {
            history = history.slice(0, SEARCH_HISTORY_MAX);
        }
        localStorage.setItem(SEARCH_HISTORY_KEY, JSON.stringify(history));
    } catch {
        // localStorage 不可用时静默失败
    }
}

// ============================================
// Escape HTML Helpers
// ============================================

/**
 * 转义 HTML 特殊字符
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

/**
 * 转义 HTML 属性值
 * @param {string} str
 * @returns {string}
 */
function escapeHtmlAttr(str) {
    if (!str) return '';
    return str
        .replace(/&/g, '&amp;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');
}
