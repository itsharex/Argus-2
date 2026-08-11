// @ts-check
// Knowledge Tab — 知识库页面逻辑

// 知识库状态
let currentKnowledgeDocs = [];
let currentKnowledgeDoc = null;
let currentKnowledgeType = 'plans';
let isKnowledgeEditing = false;
let isKnowledgeComplianceView = false;

// ============================================
// Context Menu Event Delegation
// ============================================

// 初始化右键菜单事件委托
function initKnowledgeContextMenu() {
    const container = document.getElementById('knowledgeDocList');
    if (!container) return;

    // 使用事件委托处理右键菜单
    container.addEventListener('contextmenu', (e) => {
        const docItem = e.target.closest('.knowledge-doc-item');
        if (docItem) {
            // 使用 getAttribute 获取原始路径
            const path = docItem.getAttribute('data-path');
            if (path) {
                e.preventDefault();
                e.stopPropagation();
                showKnowledgeContextMenu(e, path);
            }
        }
    });
}

// 在 DOM 加载完成后初始化
document.addEventListener('DOMContentLoaded', () => {
    initKnowledgeContextMenu();
});

// ============================================
// Initialization
// ============================================

async function loadKnowledgeDocuments(type = 'plans', project = '') {
    try {
        const docs = await window.go.main.App.GetKnowledgeDocuments(type, project);
        currentKnowledgeDocs = docs;
        renderKnowledgeDocList(docs);
    } catch (error) {
        console.error('Failed to load knowledge documents:', error);
        showToast(t('loadFailed') || '加载失败');
    }
}

// ============================================
// Document List
// ============================================

function renderKnowledgeDocList(docs) {
    const container = document.getElementById('knowledgeDocList');
    if (!container) return;

    // 记忆和CLAUDE.md模式下显示所有项目
    if (currentKnowledgeType === 'memory' || currentKnowledgeType === 'claudemd') {
        renderMemoryProjectList(docs);
        return;
    }

    if (!docs || docs.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <p>${t('noDocuments') || '暂无文档'}</p>
            </div>
        `;
        return;
    }

    // 按项目分组
    const groups = {};
    docs.forEach(doc => {
        const project = doc.project || 'plans';
        if (!groups[project]) {
            groups[project] = [];
        }
        groups[project].push(doc);
    });

    // 渲染分组列表
    let html = '';
    for (const [project, projectDocs] of Object.entries(groups)) {
        const displayName = formatProjectName(project);
        html += `
            <div class="knowledge-group">
                <div class="group-header" onclick="toggleKnowledgeGroup(this)">
                    <svg class="group-icon" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M6 9l6 6 6-6"/>
                    </svg>
                    <span class="group-name">${escapeHtml(displayName)}</span>
                    <span class="group-count">${projectDocs.length}</span>
                </div>
                <div class="group-items">
                    ${projectDocs.map(doc => renderDocItem(doc)).join('')}
                </div>
            </div>
        `;
    }

    container.innerHTML = html;

    // 默认折叠所有分组
    container.querySelectorAll('.knowledge-group').forEach(group => {
        group.classList.add('collapsed');
    });
}

// 渲染单个文档项
function renderDocItem(doc) {
    return `
        <div class="knowledge-doc-item ${currentKnowledgeDoc?.path === doc.path ? 'active' : ''}"
             data-path="${escapeHtmlAttr(doc.path)}"
             onclick="selectKnowledgeDoc(this.dataset.path)">
            <div class="doc-icon">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    ${doc.type === 'plans'
                        ? '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/>'
                        : doc.type === 'claudemd'
                        ? '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/><polyline points="14 2 14 8 20 8"/><line x1="16" y1="13" x2="8" y2="13"/><line x1="16" y1="17" x2="8" y2="17"/>'
                        : '<path d="M4 19.5A2.5 2.5 0 0 1 6.5 17H20"/><path d="M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2z"/>'}
                </svg>
            </div>
            <div class="doc-info">
                <div class="doc-title">${escapeHtml(doc.name)}</div>
                <div class="doc-meta">
                    <span class="doc-type">${doc.type === 'plans' ? 'Plan' : doc.type === 'claudemd' ? 'CLAUDE.md' : 'Memory'}</span>
                    <span class="doc-time">${formatKnowledgeTime(doc.createdAt)}</span>
                </div>
            </div>
        </div>
    `;
}

// ============================================
// Context Menu (右键菜单)
// ============================================

let currentContextMenu = null;

/**
 * 显示知识库文档的右键菜单
 * @param {MouseEvent} event
 * @param {string} filePath - 文档文件路径
 */
function showKnowledgeContextMenu(event, filePath) {
    event.preventDefault();
    event.stopPropagation();

    // 隐藏已有的菜单
    hideKnowledgeContextMenu();

    // 创建菜单元素
    const menu = document.createElement('div');
    menu.className = 'knowledge-context-menu';

    const menuItem = document.createElement('div');
    menuItem.className = 'context-menu-item';
    menuItem.innerHTML = `
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
            <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
        </svg>
        <span>打开文件位置</span>
    `;
    menuItem.addEventListener('click', () => {
        openFileLocation(filePath);
    });
    menu.appendChild(menuItem);

    // 定位菜单
    const x = event.clientX;
    const y = event.clientY;
    menu.style.left = x + 'px';
    menu.style.top = y + 'px';

    // 添加到页面
    document.body.appendChild(menu);
    currentContextMenu = menu;

    // 调整菜单位置，确保不超出视口
    requestAnimationFrame(() => {
        const rect = menu.getBoundingClientRect();
        if (rect.right > window.innerWidth) {
            menu.style.left = (x - rect.width) + 'px';
        }
        if (rect.bottom > window.innerHeight) {
            menu.style.top = (y - rect.height) + 'px';
        }
    });

    // 点击其他地方隐藏菜单
    setTimeout(() => {
        document.addEventListener('click', hideKnowledgeContextMenu, { once: true });
        document.addEventListener('contextmenu', hideKnowledgeContextMenu, { once: true });
    }, 0);
}

/**
 * 隐藏右键菜单
 */
function hideKnowledgeContextMenu() {
    if (currentContextMenu) {
        currentContextMenu.remove();
        currentContextMenu = null;
    }
}

/**
 * 打开文件所在位置
 * @param {string} filePath - 文件路径
 */
async function openFileLocation(filePath) {
    hideKnowledgeContextMenu();
    try {
        console.log('Opening file location:', filePath);
        await window.go.main.App.OpenFileLocation(filePath);
    } catch (error) {
        console.error('Failed to open file location:', error);
        console.error('File path:', filePath);
        const errorMsg = error.message || String(error);
        showToast(t('openFileLocationFailed') + ': ' + errorMsg);
    }
}

// 渲染记忆项目列表（显示所有项目）
async function renderMemoryProjectList(docs) {
    const container = document.getElementById('knowledgeDocList');
    if (!container) return;

    try {
        const projects = await window.go.main.App.GetKnowledgeProjects();

        if (!projects || projects.length === 0) {
            container.innerHTML = `
                <div class="empty-state">
                    <p>暂无项目</p>
                </div>
            `;
            return;
        }

        // 按项目分组记忆文档
        const memoryByProject = {};
        docs.forEach(doc => {
            if (!memoryByProject[doc.project]) {
                memoryByProject[doc.project] = [];
            }
            memoryByProject[doc.project].push(doc);
        });

        // 渲染所有项目
        let html = '';
        projects.forEach(project => {
            const projectDocs = memoryByProject[project] || [];
            const displayName = formatProjectName(project);

            html += `
                <div class="knowledge-group">
                    <div class="group-header" onclick="toggleKnowledgeGroup(this)">
                        <svg class="group-icon" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                            <path d="M6 9l6 6 6-6"/>
                        </svg>
                        <span class="group-name">${escapeHtml(displayName)}</span>
                        <span class="group-count">${projectDocs.length}</span>
                        <button class="group-add-btn" onclick="event.stopPropagation(); createMemoryForProject('${escapeHtmlAttr(project)}')" title="新建记忆">
                            <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                                <line x1="12" y1="5" x2="12" y2="19"/>
                                <line x1="5" y1="12" x2="19" y2="12"/>
                            </svg>
                        </button>
                    </div>
                    <div class="group-items">
                        ${projectDocs.length > 0
                            ? projectDocs.map(doc => renderDocItem(doc)).join('')
                            : '<div class="empty-project-hint">暂无记忆，点击 + 新建</div>'}
                    </div>
                </div>
            `;
        });

        container.innerHTML = html;

        // 默认折叠所有分组
        container.querySelectorAll('.knowledge-group').forEach(group => {
            group.classList.add('collapsed');
        });
    } catch (error) {
        console.error('Failed to load projects:', error);
        container.innerHTML = `
            <div class="empty-state">
                <p>加载失败</p>
            </div>
        `;
    }
}

// 格式化项目名称（与后端 formatProjectName 保持一致）
function formatProjectName(dirName) {
    if (dirName === 'plans') return 'Plans';
    // 去掉开头的连字符，过滤空字符串，取最后两个段
    const name = dirName.replace(/^-/, '');
    const parts = name.split('-').filter(p => p.length > 0);
    if (parts.length >= 2) {
        return parts.slice(-2).join('-');
    }
    if (parts.length === 1) {
        return parts[0];
    }
    return dirName;
}

// 切换分组展开/折叠
function toggleKnowledgeGroup(header) {
    const group = header.closest('.knowledge-group');
    if (group) {
        group.classList.toggle('collapsed');
    }
}

// ============================================
// Document Selection
// ============================================

async function selectKnowledgeDoc(path) {
    try {
        const doc = await window.go.main.App.GetKnowledgeDocument(path);
        currentKnowledgeDoc = doc;

        // 关闭合规审计视图（如果打开的话）
        closeKnowledgeComplianceAudit();

        // 更新列表高亮（使用 data-path 属性精确匹配）
        document.querySelectorAll('.knowledge-doc-item').forEach(item => {
            const itemPath = item.getAttribute('data-path');
            item.classList.toggle('active', itemPath === path);
        });

        // 更新工具栏
        const docName = document.getElementById('knowledgeDocName');
        const docType = document.getElementById('knowledgeDocType');
        const auditBtn = document.getElementById('knowledgeAuditBtn');
        if (docName) docName.textContent = doc.name;
        if (docType) {
            const typeLabels = { plans: 'Plan', memory: 'Memory', claudemd: 'CLAUDE.md' };
            docType.textContent = typeLabels[doc.type] || doc.type;
            docType.className = `doc-type-badge ${doc.type}`;
        }

        // 判断是否为 CLAUDE.md 文档（类型或文件名匹配）
        const isClaudeMD = doc.type === 'claudemd' ||
            (doc.name && doc.name.toUpperCase() === 'CLAUDE.MD') ||
            (doc.path && doc.path.toUpperCase().endsWith('/CLAUDE.MD'));

        // CLAUDE.md 文档显示合规审计按钮
        if (auditBtn) {
            auditBtn.style.display = isClaudeMD ? 'inline-flex' : 'none';
        }

        // 根据文档类型切换编辑器
        if (isClaudeMD) {
            // CLAUDE.md 使用分节编辑器
            hideDefaultEditors();
            showClaudeMDEditor();
            loadClaudeMDSections(path, doc.project);
        } else {
            // 其他文档使用普通编辑器
            hideClaudeMDEditor();
            showDefaultEditors();
            renderKnowledgePreview(doc.content);
            exitKnowledgeEdit();
        }
    } catch (error) {
        console.error('Failed to load document:', error);
    }
}

// ============================================
// Preview / Editor
// ============================================

function renderKnowledgePreview(content) {
    const preview = document.getElementById('knowledgePreview');
    if (!preview) return;

    if (!content) {
        preview.innerHTML = `
            <div class="empty-state">
                <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                    <polyline points="14 2 14 8 20 8"/>
                    <line x1="16" y1="13" x2="8" y2="13"/>
                    <line x1="16" y1="17" x2="8" y2="17"/>
                </svg>
                <p>${t('selectDocument') || '选择文档查看内容'}</p>
            </div>
        `;
        return;
    }

    // 解析 frontmatter 并提取 body
    const { frontmatter, body } = parseFrontmatter(content);

    // 构建预览 HTML
    let html = '';

    // 如果有 frontmatter，显示为元数据卡片
    if (frontmatter && Object.keys(frontmatter).length > 0) {
        html += '<div class="frontmatter-card">';
        for (const [key, value] of Object.entries(frontmatter)) {
            if (key !== 'metadata' && value) {
                html += `<div class="frontmatter-item"><span class="frontmatter-key">${escapeHtml(key)}:</span> <span class="frontmatter-value">${escapeHtml(value)}</span></div>`;
            }
        }
        html += '</div>';
    }

    // 渲染 Markdown body
    html += '<div class="markdown-content">';
    html += renderMarkdown(body);
    html += '</div>';

    preview.innerHTML = html;
}

// 解析 YAML frontmatter
function parseFrontmatter(content) {
    const frontmatter = {};
    let body = content;

    // 检查是否以 --- 开头
    if (!content.startsWith('---')) {
        return { frontmatter, body };
    }

    // 查找结束标记
    const endIndex = content.indexOf('---', 3);
    if (endIndex === -1) {
        return { frontmatter, body };
    }

    // 提取 frontmatter 部分
    const fmContent = content.substring(3, endIndex);
    body = content.substring(endIndex + 3).trim();

    // 简单解析 YAML（支持 key: value 格式）
    const lines = fmContent.split('\n');
    let currentKey = '';
    let currentValue = '';

    for (const line of lines) {
        const trimmed = line.trim();
        if (trimmed === '' || trimmed.startsWith('#')) {
            continue;
        }

        // 检查是否是新的 key: value 对
        const colonIndex = trimmed.indexOf(':');
        if (colonIndex > 0) {
            // 保存上一个 key-value
            if (currentKey) {
                frontmatter[currentKey] = currentValue.trim();
            }
            currentKey = trimmed.substring(0, colonIndex).trim();
            currentValue = trimmed.substring(colonIndex + 1).trim();
        } else if (currentKey) {
            // 继续上一个 value（多行值）
            currentValue += ' ' + trimmed;
        }
    }

    // 保存最后一个 key-value
    if (currentKey) {
        frontmatter[currentKey] = currentValue.trim();
    }

    return { frontmatter, body };
}

function renderMarkdown(content) {
    if (!content) return '';

    // 先转义 HTML 特殊字符，防止 XSS
    let escaped = content
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');

    // 按行处理 Markdown
    const lines = escaped.split('\n');
    let html = '';
    let inUnorderedList = false;
    let inOrderedList = false;
    let inCodeBlock = false;
    let codeBlockContent = '';

    for (let i = 0; i < lines.length; i++) {
        let line = lines[i];

        // 处理代码块
        if (line.trim().startsWith('```')) {
            if (inCodeBlock) {
                html += `<pre><code>${codeBlockContent}</code></pre>`;
                codeBlockContent = '';
                inCodeBlock = false;
            } else {
                inCodeBlock = true;
            }
            continue;
        }

        if (inCodeBlock) {
            codeBlockContent += (codeBlockContent ? '\n' : '') + line;
            continue;
        }

        // 关闭无序列表
        if (inUnorderedList && !line.match(/^\s*[-*]\s/)) {
            html += '</ul>';
            inUnorderedList = false;
        }

        // 关闭有序列表
        if (inOrderedList && !line.match(/^\s*\d+\.\s/)) {
            html += '</ol>';
            inOrderedList = false;
        }

        // 去除行首缩进后再匹配标题（支持缩进行的标题渲染）
        const trimmedLine = line.replace(/^ {1,8}/, '');

        // 处理标题（支持缩进）
        if (trimmedLine.match(/^#### /)) {
            html += `<h4>${trimmedLine.substring(5)}</h4>`;
            continue;
        }
        if (trimmedLine.match(/^### /)) {
            html += `<h3>${trimmedLine.substring(4)}</h3>`;
            continue;
        }
        if (trimmedLine.match(/^## /)) {
            html += `<h2>${trimmedLine.substring(3)}</h2>`;
            continue;
        }
        if (trimmedLine.match(/^# /)) {
            html += `<h1>${trimmedLine.substring(2)}</h1>`;
            continue;
        }

        // 处理水平线
        if (line.match(/^[\s]*-{3,}$/)) {
            html += '<hr>';
            continue;
        }

        // 处理引用块
        if (line.match(/^[\s]*&gt;\s?/)) {
            const quoteContent = line.replace(/^[\s]*&gt;\s?/, '');
            html += `<blockquote>${processInlineMarkdown(quoteContent)}</blockquote>`;
            continue;
        }

        // 处理无序列表项
        if (line.match(/^\s*[-*]\s/)) {
            if (!inUnorderedList) {
                html += '<ul>';
                inUnorderedList = true;
            }
            const listContent = line.replace(/^\s*[-*]\s/, '');
            html += `<li>${processInlineMarkdown(listContent)}</li>`;
            continue;
        }

        // 处理有序列表项
        if (line.match(/^\s*\d+\.\s/)) {
            if (!inOrderedList) {
                html += '<ol>';
                inOrderedList = true;
            }
            const listContent = line.replace(/^\s*\d+\.\s/, '');
            html += `<li>${processInlineMarkdown(listContent)}</li>`;
            continue;
        }

        // 处理空行
        if (line.trim() === '') {
            html += '<br>';
            continue;
        }

        // 处理普通段落
        html += `<p>${processInlineMarkdown(line)}</p>`;
    }

    // 关闭未关闭的列表
    if (inUnorderedList) {
        html += '</ul>';
    }
    if (inOrderedList) {
        html += '</ol>';
    }

    return html;
}

// 处理行内 Markdown（粗体、斜体、代码、链接）
function processInlineMarkdown(text) {
    return text
        // 行内代码（先处理，避免被其他规则影响）
        .replace(/`(.*?)`/g, '<code>$1</code>')
        // 粗体
        .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
        // 斜体
        .replace(/\*(.*?)\*/g, '<em>$1</em>')
        // 链接（过滤 javascript: URI）
        .replace(/\[(.*?)\]\((.*?)\)/g, (match, linkText, url) => {
            const trimmedUrl = url.trim().toLowerCase();
            if (trimmedUrl.startsWith('javascript:') || trimmedUrl.startsWith('data:')) {
                return match; // 返回原始文本，不创建链接
            }
            return `<a href="${url}" target="_blank">${linkText}</a>`;
        });
}

function toggleKnowledgeEdit() {
    if (!currentKnowledgeDoc) return;

    isKnowledgeEditing = true;
    const preview = document.getElementById('knowledgePreview');
    const editor = document.getElementById('knowledgeEditor');
    const editBtn = document.getElementById('knowledgeEditBtn');
    const saveBtn = document.getElementById('knowledgeSaveBtn');
    const textarea = document.getElementById('knowledgeEditorContent');

    if (preview) preview.style.display = 'none';
    if (editor) editor.style.display = 'flex';
    if (editBtn) editBtn.style.display = 'none';
    if (saveBtn) saveBtn.style.display = 'inline-flex';
    if (textarea) textarea.value = currentKnowledgeDoc.content;
}

function exitKnowledgeEdit() {
    isKnowledgeEditing = false;
    const preview = document.getElementById('knowledgePreview');
    const editor = document.getElementById('knowledgeEditor');
    const editBtn = document.getElementById('knowledgeEditBtn');
    const saveBtn = document.getElementById('knowledgeSaveBtn');

    if (preview) preview.style.display = 'block';
    if (editor) editor.style.display = 'none';
    if (editBtn) editBtn.style.display = 'inline-flex';
    if (saveBtn) saveBtn.style.display = 'none';
}

// ============================================
// Editor Visibility Helpers
// ============================================

/**
 * 隐藏默认编辑器（预览 + textarea 编辑器）
 */
function hideDefaultEditors() {
    const preview = document.getElementById('knowledgePreview');
    const editor = document.getElementById('knowledgeEditor');
    if (preview) preview.style.display = 'none';
    if (editor) editor.style.display = 'none';

    // 隐藏默认工具栏按钮
    const editBtn = document.getElementById('knowledgeEditBtn');
    const saveBtn = document.getElementById('knowledgeSaveBtn');
    if (editBtn) editBtn.style.display = 'none';
    if (saveBtn) saveBtn.style.display = 'none';
}

/**
 * 显示默认编辑器
 */
function showDefaultEditors() {
    const preview = document.getElementById('knowledgePreview');
    if (preview) preview.style.display = 'block';

    const editBtn = document.getElementById('knowledgeEditBtn');
    if (editBtn) editBtn.style.display = 'inline-flex';
}

/**
 * 显示 CLAUDE.md 分节编辑器
 */
function showClaudeMDEditor() {
    const editor = document.getElementById('claudeMDEditor');
    if (editor) editor.style.display = 'flex';
}

/**
 * 隐藏 CLAUDE.md 分节编辑器
 */
function hideClaudeMDEditor() {
    const editor = document.getElementById('claudeMDEditor');
    if (editor) editor.style.display = 'none';
}

// ============================================
// Refresh Documents
// ============================================

async function refreshKnowledge() {
    await loadKnowledgeDocuments(currentKnowledgeType);
    showToast(t('refreshed') || '已刷新');
}

// ============================================
// Document Operations
// ============================================

async function saveKnowledgeDoc() {
    if (!currentKnowledgeDoc) return;

    const textarea = document.getElementById('knowledgeEditorContent');
    if (!textarea) return;

    const content = textarea.value;
    try {
        await window.go.main.App.SaveKnowledgeDocument(currentKnowledgeDoc.path, content);
        currentKnowledgeDoc.content = content;
        exitKnowledgeEdit();
        renderKnowledgePreview(content);
        showToast(t('documentSaved') || '文档已保存');
    } catch (error) {
        console.error('Failed to save document:', error);
        showToast(t('saveFailed') || '保存失败');
    }
}

async function deleteKnowledgeDoc() {
    if (!currentKnowledgeDoc) return;

    const confirmMsg = t('confirmDeleteDocument') || '确定要删除这个文档吗？';
    if (!confirm(confirmMsg)) return;

    try {
        await window.go.main.App.DeleteKnowledgeDocument(currentKnowledgeDoc.path);
        currentKnowledgeDoc = null;
        loadKnowledgeDocuments(currentKnowledgeType);
        showToast(t('documentDeleted') || '文档已删除');

        // 清空预览区
        const preview = document.getElementById('knowledgePreview');
        if (preview) {
            preview.innerHTML = `
                <div class="empty-state">
                    <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z"/>
                        <polyline points="14 2 14 8 20 8"/>
                        <line x1="16" y1="13" x2="8" y2="13"/>
                        <line x1="16" y1="17" x2="8" y2="17"/>
                    </svg>
                    <p>${t('selectDocument') || '选择文档查看内容'}</p>
                </div>
            `;
        }

        // 清空工具栏
        const docName = document.getElementById('knowledgeDocName');
        const docType = document.getElementById('knowledgeDocType');
        if (docName) docName.textContent = t('selectDocument') || '选择文档查看';
        if (docType) docType.textContent = '';

        // 隐藏 CLAUDE.md 编辑器
        hideClaudeMDEditor();
        showDefaultEditors();
    } catch (error) {
        console.error('Failed to delete document:', error);
        showToast(t('deleteFailed') || '删除失败');
    }
}

async function createNewDocument() {
    // 打开新建文档对话框
    openCreateDocModal();
}

// ============================================
// Create Document Modal
// ============================================

let selectedProject = '';

async function openCreateDocModal() {
    const modal = document.getElementById('createDocModal');
    if (!modal) return;

    modal.style.display = 'flex';
    // 添加 show 类以触发动画
    setTimeout(() => modal.classList.add('show'), 10);
    selectedProject = '';

    // 根据当前筛选类型设置默认文档类型
    const memoryRadio = document.querySelector('input[name="docType"][value="memory"]');
    const plansRadio = document.querySelector('input[name="docType"][value="plans"]');

    if (currentKnowledgeType === 'plans') {
        if (plansRadio) plansRadio.checked = true;
    } else {
        if (memoryRadio) memoryRadio.checked = true;
    }

    toggleProjectSelect();
    await loadProjectSelectList();
}

function closeCreateDocModal() {
    const modal = document.getElementById('createDocModal');
    if (modal) {
        modal.classList.remove('show');
        setTimeout(() => modal.style.display = 'none', 200);
    }
    selectedProject = '';
}

function toggleProjectSelect() {
    const docType = document.querySelector('input[name="docType"]:checked')?.value;
    const projectGroup = document.getElementById('projectSelectGroup');
    if (projectGroup) {
        projectGroup.style.display = docType === 'memory' ? 'block' : 'none';
    }
}

async function loadProjectSelectList() {
    const container = document.getElementById('projectSelectList');
    if (!container) return;

    try {
        const projects = await window.go.main.App.GetKnowledgeProjects();
        if (!projects || projects.length === 0) {
            container.innerHTML = '<div class="empty-state"><p>暂无项目</p></div>';
            return;
        }

        container.innerHTML = projects.map(p => `
            <div class="project-select-item" data-project="${escapeHtmlAttr(p)}" onclick="selectProjectItem(this)">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
                </svg>
                <span>${escapeHtml(formatProjectName(p))}</span>
            </div>
        `).join('');
    } catch (error) {
        console.error('Failed to load projects:', error);
        container.innerHTML = '<div class="empty-state"><p>加载失败</p></div>';
    }
}

function selectProjectItem(item) {
    document.querySelectorAll('.project-select-item').forEach(el => el.classList.remove('active'));
    item.classList.add('active');
    selectedProject = item.dataset.project;
}

async function confirmCreateDoc() {
    const docType = document.querySelector('input[name="docType"]:checked')?.value;
    if (!docType) {
        showToast('请选择文档类型');
        return;
    }

    if ((docType === 'memory' || docType === 'claudemd') && !selectedProject) {
        showToast('请选择项目');
        return;
    }

    try {
        const path = await window.go.main.App.CreateKnowledgeDocument(docType, '', '', selectedProject, '');
        closeCreateDocModal();
        await loadKnowledgeDocuments(currentKnowledgeType);
        await selectKnowledgeDoc(path);
        if (docType !== 'claudemd') {
            toggleKnowledgeEdit();
        }
    } catch (error) {
        console.error('Failed to create document:', error);
        showToast(t('createFailed') || '创建失败');
    }
}

// 为指定项目创建记忆
async function createMemoryForProject(project) {
    selectedProject = project;
    const docType = 'memory';

    try {
        const path = await window.go.main.App.CreateKnowledgeDocument(docType, '', '', project, '');
        await loadKnowledgeDocuments(currentKnowledgeType);
        await selectKnowledgeDoc(path);
        toggleKnowledgeEdit();
    } catch (error) {
        console.error('Failed to create document:', error);
        showToast(t('createFailed') || '创建失败');
    }
}

// ============================================
// Filtering & Search
// ============================================

function filterByType(type) {
    currentKnowledgeType = type;
    document.querySelectorAll('.knowledge-filters .filter-btn').forEach(btn => {
        btn.classList.toggle('active', btn.getAttribute('data-type') === type);
    });

    // 如果选择的是 skills 类型，切换到 Skills 视图
    if (type === 'skills') {
        showSkillsView();
        loadSkills('all');
    } else {
        showKnowledgeView();
        loadKnowledgeDocuments(type);
    }
}

// 显示 Skills 视图
function showSkillsView() {
    const knowledgeDocList = document.getElementById('knowledgeDocList');
    const skillDocList = document.getElementById('skillDocList');
    const knowledgeDocName = document.getElementById('knowledgeDocName');
    const knowledgeDocType = document.getElementById('knowledgeDocType');
    const knowledgeDocNameInput = document.getElementById('knowledgeDocNameInput');
    const skillDocName = document.getElementById('skillDocName');
    const skillDocScope = document.getElementById('skillDocScope');
    const knowledgeEditBtn = document.getElementById('knowledgeEditBtn');
    const knowledgeSaveBtn = document.getElementById('knowledgeSaveBtn');
    const skillEditBtn = document.getElementById('skillEditBtn');
    const skillSaveBtn = document.getElementById('skillSaveBtn');
    const skillDeleteBtn = document.getElementById('skillDeleteBtn');
    const knowledgePreview = document.getElementById('knowledgePreview');
    const skillPreview = document.getElementById('skillPreview');
    const knowledgeEditor = document.getElementById('knowledgeEditor');
    const skillEditor = document.getElementById('skillEditor');

    if (knowledgeDocList) knowledgeDocList.style.display = 'none';
    if (skillDocList) skillDocList.style.display = 'block';
    if (knowledgeDocName) knowledgeDocName.style.display = 'none';
    if (knowledgeDocType) knowledgeDocType.style.display = 'none';
    if (knowledgeDocNameInput) knowledgeDocNameInput.style.display = 'none';
    if (skillDocName) skillDocName.style.display = 'inline';
    if (skillDocScope) skillDocScope.style.display = 'inline';
    if (knowledgeEditBtn) knowledgeEditBtn.style.display = 'none';
    if (knowledgeSaveBtn) knowledgeSaveBtn.style.display = 'none';
    if (skillEditBtn) skillEditBtn.style.display = 'inline-flex';
    if (skillSaveBtn) skillSaveBtn.style.display = 'none';
    if (skillDeleteBtn) skillDeleteBtn.style.display = 'inline-flex';
    if (knowledgePreview) knowledgePreview.style.display = 'none';
    if (skillPreview) skillPreview.style.display = 'block';
    if (knowledgeEditor) knowledgeEditor.style.display = 'none';
    if (skillEditor) skillEditor.style.display = 'none';
}

// 显示知识库视图
function showKnowledgeView() {
    const knowledgeDocList = document.getElementById('knowledgeDocList');
    const skillDocList = document.getElementById('skillDocList');
    const knowledgeDocName = document.getElementById('knowledgeDocName');
    const knowledgeDocType = document.getElementById('knowledgeDocType');
    const skillDocName = document.getElementById('skillDocName');
    const skillDocScope = document.getElementById('skillDocScope');
    const knowledgeEditBtn = document.getElementById('knowledgeEditBtn');
    const skillEditBtn = document.getElementById('skillEditBtn');
    const skillSaveBtn = document.getElementById('skillSaveBtn');
    const skillDeleteBtn = document.getElementById('skillDeleteBtn');
    const knowledgePreview = document.getElementById('knowledgePreview');
    const skillPreview = document.getElementById('skillPreview');

    if (knowledgeDocList) knowledgeDocList.style.display = 'block';
    if (skillDocList) skillDocList.style.display = 'none';
    if (knowledgeDocName) knowledgeDocName.style.display = 'inline';
    if (knowledgeDocType) knowledgeDocType.style.display = 'inline';
    if (skillDocName) skillDocName.style.display = 'none';
    if (skillDocScope) skillDocScope.style.display = 'none';
    if (knowledgeEditBtn) knowledgeEditBtn.style.display = 'inline-flex';
    if (skillEditBtn) skillEditBtn.style.display = 'none';
    if (skillSaveBtn) skillSaveBtn.style.display = 'none';
    if (skillDeleteBtn) skillDeleteBtn.style.display = 'none';
    if (knowledgePreview) knowledgePreview.style.display = 'block';
    if (skillPreview) skillPreview.style.display = 'none';
}

let knowledgeSearchTimeout;
function searchKnowledge() {
    clearTimeout(knowledgeSearchTimeout);
    knowledgeSearchTimeout = setTimeout(async () => {
        const input = document.getElementById('knowledgeSearchInput');
        if (!input) return;

        const query = input.value;
        try {
            const docs = await window.go.main.App.SearchKnowledgeDocuments(query, [], []);
            renderKnowledgeDocList(docs);
        } catch (error) {
            console.error('Failed to search:', error);
        }
    }, 300);
}

// ============================================
// Utility Functions
// ============================================

function formatKnowledgeTime(timeStr) {
    if (!timeStr) return '';
    const date = new Date(timeStr);
    const now = new Date();
    const diff = now - date;

    // 小于 1 小时
    if (diff < 3600000) {
        const minutes = Math.floor(diff / 60000);
        return `${minutes || 1} ${t('minutesAgo') || '分钟前'}`;
    }

    // 小于 24 小时
    if (diff < 86400000) {
        const hours = Math.floor(diff / 3600000);
        return `${hours} ${t('hoursAgo') || '小时前'}`;
    }

    // 小于 7 天
    if (diff < 604800000) {
        const days = Math.floor(diff / 86400000);
        return `${days} ${t('daysAgo') || '天前'}`;
    }

    // 超过 7 天，显示日期
    return date.toLocaleDateString();
}

// ============================================
// Rename Document
// ============================================

function startRenameDoc() {
    if (!currentKnowledgeDoc) return;

    const docName = document.getElementById('knowledgeDocName');
    const docNameInput = document.getElementById('knowledgeDocNameInput');

    if (docName && docNameInput) {
        docName.style.display = 'none';
        docNameInput.style.display = 'block';
        docNameInput.value = currentKnowledgeDoc.name;
        docNameInput.focus();
        docNameInput.select();
    }
}

async function finishRenameDoc() {
    const docName = document.getElementById('knowledgeDocName');
    const docNameInput = document.getElementById('knowledgeDocNameInput');

    if (!docName || !docNameInput || !currentKnowledgeDoc) return;

    const newName = docNameInput.value.trim();
    if (newName && newName !== currentKnowledgeDoc.name) {
        try {
            // 获取新的文件路径
            const newPath = await window.go.main.App.RenameKnowledgeDocument(currentKnowledgeDoc.path, newName);
            // 更新当前文档的路径和名称
            currentKnowledgeDoc.path = newPath;
            currentKnowledgeDoc.name = newName;
            docName.textContent = newName;
            // 刷新文档列表
            await loadKnowledgeDocuments(currentKnowledgeType);
            showToast('名称已修改');
        } catch (error) {
            console.error('Failed to rename document:', error);
            showToast(t('renameFailed') || '重命名失败');
        }
    }

    docName.style.display = 'block';
    docNameInput.style.display = 'none';
}

function handleRenameKeydown(event) {
    if (event.key === 'Enter') {
        event.target.blur();
    } else if (event.key === 'Escape') {
        const docName = document.getElementById('knowledgeDocName');
        const docNameInput = document.getElementById('knowledgeDocNameInput');
        if (docName && docNameInput) {
            docNameInput.value = currentKnowledgeDoc?.name || '';
            docName.style.display = 'block';
            docNameInput.style.display = 'none';
        }
    }
}

// ============================================
// Knowledge Sidebar Resizer
// ============================================

(function() {
    const resizer = document.getElementById('knowledgeResizer');
    const sidebar = document.querySelector('.knowledge-sidebar');

    if (!resizer || !sidebar) return;

    let isResizing = false;
    let startX = 0;
    let startWidth = 0;

    resizer.addEventListener('mousedown', (e) => {
        isResizing = true;
        startX = e.clientX;
        startWidth = sidebar.offsetWidth;
        resizer.classList.add('active');
        document.body.style.cursor = 'col-resize';
        document.body.style.userSelect = 'none';
    });

    document.addEventListener('mousemove', (e) => {
        if (!isResizing) return;

        const diff = e.clientX - startX;
        const newWidth = Math.min(Math.max(startWidth + diff, 200), 500);
        sidebar.style.width = newWidth + 'px';
    });

    document.addEventListener('mouseup', () => {
        if (!isResizing) return;

        isResizing = false;
        resizer.classList.remove('active');
        document.body.style.cursor = '';
        document.body.style.userSelect = '';
    });
})();

// ============================================
// Compliance Audit Sub-View (知识库内嵌)
// ============================================

/**
 * 打开合规审计子视图（从知识库工具栏触发）
 */
function openKnowledgeComplianceAudit() {
    isKnowledgeComplianceView = true;

    // 隐藏文档编辑器相关视图
    const preview = document.getElementById('knowledgePreview');
    const editor = document.getElementById('knowledgeEditor');
    const claudeEditor = document.getElementById('claudeMDEditor');
    const complianceView = document.getElementById('knowledgeComplianceView');

    if (preview) preview.style.display = 'none';
    if (editor) editor.style.display = 'none';
    if (claudeEditor) claudeEditor.style.display = 'none';
    if (complianceView) complianceView.style.display = 'flex';

    // 隐藏工具栏右侧按钮
    const editBtn = document.getElementById('knowledgeEditBtn');
    const saveBtn = document.getElementById('knowledgeSaveBtn');
    const auditBtn = document.getElementById('knowledgeAuditBtn');
    if (editBtn) editBtn.style.display = 'none';
    if (saveBtn) saveBtn.style.display = 'none';
    if (auditBtn) auditBtn.style.display = 'none';

    // 设置当前审计的项目名
    if (currentKnowledgeDoc) {
        currentProjectForAudit = currentKnowledgeDoc.project || '';
    }

    // 初始化合规审计（使用当前 CLAUDE.md 路径）
    initComplianceForKnowledge();
}

/**
 * 关闭合规审计子视图，恢复文档视图
 */
function closeKnowledgeComplianceAudit() {
    if (!isKnowledgeComplianceView) return;
    isKnowledgeComplianceView = false;

    const complianceView = document.getElementById('knowledgeComplianceView');
    if (complianceView) complianceView.style.display = 'none';

    // 恢复文档视图
    if (currentKnowledgeDoc) {
        const isClaudeMD = currentKnowledgeDoc.type === 'claudemd' ||
            (currentKnowledgeDoc.name && currentKnowledgeDoc.name.toUpperCase() === 'CLAUDE.MD') ||
            (currentKnowledgeDoc.path && currentKnowledgeDoc.path.toUpperCase().endsWith('/CLAUDE.MD'));

        if (isClaudeMD) {
            showClaudeMDEditor();
        } else {
            showDefaultEditors();
            const editBtn = document.getElementById('knowledgeEditBtn');
            if (editBtn) editBtn.style.display = 'inline-flex';
        }

        // 恢复合规审计按钮（仅 CLAUDE.md）
        const auditBtn = document.getElementById('knowledgeAuditBtn');
        if (auditBtn && isClaudeMD) {
            auditBtn.style.display = 'inline-flex';
        }
    }
}

/**
 * 为知识库初始化合规审计（设置审计目标路径和项目名）
 */
async function initComplianceForKnowledge() {
    // 重置审计按钮状态
    updateAuditButton(false);

    // 判断是否为 CLAUDE.md（类型或文件名匹配）
    const isClaudeMD = currentKnowledgeDoc &&
        (currentKnowledgeDoc.type === 'claudemd' ||
         (currentKnowledgeDoc.name && currentKnowledgeDoc.name.toUpperCase() === 'CLAUDE.MD') ||
         (currentKnowledgeDoc.path && currentKnowledgeDoc.path.toUpperCase().endsWith('/CLAUDE.MD')));

    // 如果有当前选中的 CLAUDE.md，设置路径和项目名
    if (isClaudeMD) {
        currentClaudeMDPathForAudit = currentKnowledgeDoc.path;
        currentProjectForAudit = currentKnowledgeDoc.project || '';
    } else {
        // 尝试获取默认 CLAUDE.md 路径
        const path = await getCurrentClaudeMDPath();
        currentClaudeMDPathForAudit = path;
        currentProjectForAudit = '';
    }
}

// 存储当前审计使用的 CLAUDE.md 路径和项目名
let currentClaudeMDPathForAudit = null;
let currentProjectForAudit = '';
