// @ts-check
// CLAUDE.md Section Editor — 分节编辑器逻辑

// ============================================
// State
// ============================================

let claudeMDSections = [];
let currentSectionIndex = 0;
let claudeMDPath = '';
let claudeMDProjectName = '';
let isClaudeMDPreviewMode = false;
let isClaudeMDDirty = false;

// 分节 ID 到显示名称的映射
const sectionDisplayNames = {
    overview: { zh: '概述', en: 'Overview', icon: '📋' },
    techstack: { zh: '技术栈', en: 'Tech Stack', icon: '🔧' },
    conventions: { zh: '代码规范', en: 'Conventions', icon: '📏' },
    architecture: { zh: '架构', en: 'Architecture', icon: '🏗️' },
    commands: { zh: '常用命令', en: 'Commands', icon: '⚡' },
};

// ============================================
// Initialization
// ============================================

/**
 * 加载 CLAUDE.md 并解析为分节
 * @param {string} path - CLAUDE.md 文件路径
 * @param {string} projectName - 项目名称
 */
async function loadClaudeMDSections(path, projectName) {
    try {
        const sections = await window.go.main.App.ParseClaudeMDSections(path);
        claudeMDSections = sections;
        claudeMDPath = path;
        claudeMDProjectName = projectName || '';
        currentSectionIndex = 0;
        isClaudeMDDirty = false;
        isClaudeMDPreviewMode = false;

        renderSectionNav(sections);
        if (sections.length > 0) {
            switchSection(0);
        }
    } catch (error) {
        console.error('Failed to load CLAUDE.md sections:', error);
        showToast(t('loadFailed') || '加载失败');
    }
}

// ============================================
// Section Navigation
// ============================================

/**
 * 渲染分节导航
 * @param {Array} sections - 分节数组
 */
function renderSectionNav(sections) {
    const nav = document.getElementById('sectionNav');
    if (!nav) return;

    if (!sections || sections.length === 0) {
        nav.innerHTML = `
            <div class="section-nav-item" style="opacity: 0.5; cursor: default;">
                ${t('noSections') || '无分节'}
            </div>
        `;
        return;
    }

    // 按 order 排序
    const sorted = [...sections].sort((a, b) => a.order - b.order);

    nav.innerHTML = sorted.map((section, idx) => {
        const display = sectionDisplayNames[section.id] || { zh: section.title, en: section.title, icon: '📄' };
        const lang = getCurrentLang();
        const displayName = lang === 'zh' ? display.zh : display.en;

        return `
            <div class="section-nav-item ${idx === currentSectionIndex ? 'active' : ''}"
                 onclick="switchSection(${idx})"
                 title="${escapeHtml(section.title)}">
                <span class="section-icon">${display.icon}</span>
                <span>${escapeHtml(displayName)}</span>
            </div>
        `;
    }).join('');
}

/**
 * 切换到指定分节
 * @param {number} index - 分节索引
 */
function switchSection(index) {
    if (index < 0 || index >= claudeMDSections.length) return;

    // 保存当前分节内容
    saveCurrentSectionContent();

    currentSectionIndex = index;
    isClaudeMDPreviewMode = false;

    // 更新导航高亮
    document.querySelectorAll('.section-nav-item').forEach((item, idx) => {
        item.classList.toggle('active', idx === index);
    });

    renderSectionEditor(claudeMDSections[index]);
}

// ============================================
// Section Editor
// ============================================

/**
 * 渲染分节编辑区
 * @param {object} section - 当前分节
 */
function renderSectionEditor(section) {
    const header = document.getElementById('sectionEditorTitle');
    const content = document.getElementById('sectionEditorContent');
    const footer = document.getElementById('sectionEditorFooter');

    if (header) {
        const display = sectionDisplayNames[section.id] || { zh: section.title, en: section.title };
        const lang = getCurrentLang();
        header.textContent = lang === 'zh' ? display.zh : display.en;
    }

    if (content) {
        content.innerHTML = `
            <textarea id="sectionTextarea" class="section-textarea"
                      placeholder="${t('inputMarkdown') || '输入 Markdown 内容...'}"
                      oninput="onSectionContentChange()">${escapeHtml(section.content)}</textarea>
        `;
    }

    if (footer) {
        const charCount = section.content.length;
        footer.innerHTML = `
            <span>${charCount} ${t('characters') || '字符'}</span>
            <button class="toggle-preview-btn" onclick="toggleSectionPreview()">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/>
                    <circle cx="12" cy="12" r="3"/>
                </svg>
                ${t('preview') || '预览'}
            </button>
        `;
    }
}

/**
 * 内容变化时标记为 dirty
 */
function onSectionContentChange() {
    isClaudeMDDirty = true;
    updateSaveButtonState();
}

/**
 * 切换预览/编辑模式
 */
function toggleSectionPreview() {
    isClaudeMDPreviewMode = !isClaudeMDPreviewMode;

    const content = document.getElementById('sectionEditorContent');
    const footerToggleBtn = document.querySelector('.toggle-preview-btn');
    const headerEditBtn = document.getElementById('claudeMDEditBtn');

    if (!content) return;

    if (isClaudeMDPreviewMode) {
        // 保存当前内容
        saveCurrentSectionContent();

        // 显示预览
        const section = claudeMDSections[currentSectionIndex];
        content.innerHTML = `<div class="section-preview markdown-body">${renderMarkdown(section.content)}</div>`;
        if (footerToggleBtn) footerToggleBtn.classList.add('active');
        // 更新头部按钮为编辑模式
        if (headerEditBtn) {
            headerEditBtn.querySelector('span').textContent = t('edit') || '编辑';
            headerEditBtn.querySelector('svg').innerHTML = '<path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/>';
        }
    } else {
        // 显示编辑器
        const section = claudeMDSections[currentSectionIndex];
        content.innerHTML = `
            <textarea id="sectionTextarea" class="section-textarea"
                      placeholder="${t('inputMarkdown') || '输入 Markdown 内容...'}"
                      oninput="onSectionContentChange()">${escapeHtml(section.content)}</textarea>
        `;
        if (footerToggleBtn) footerToggleBtn.classList.remove('active');
        // 更新头部按钮为预览模式
        if (headerEditBtn) {
            headerEditBtn.querySelector('span').textContent = t('preview') || '预览';
            headerEditBtn.querySelector('svg').innerHTML = '<path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z"/><circle cx="12" cy="12" r="3"/>';
        }
    }
}

// ============================================
// Save Operations
// ============================================

/**
 * 保存当前分节内容到内存
 */
function saveCurrentSectionContent() {
    const textarea = document.getElementById('sectionTextarea');
    if (textarea && claudeMDSections[currentSectionIndex]) {
        claudeMDSections[currentSectionIndex].content = textarea.value;
    }
}

/**
 * 保存所有分节到文件
 */
async function saveAllSections() {
    // 先保存当前分节
    saveCurrentSectionContent();

    try {
        await window.go.main.App.SaveClaudeMDSections(
            claudeMDPath,
            claudeMDProjectName,
            claudeMDSections
        );
        isClaudeMDDirty = false;
        updateSaveButtonState();
        showToast(t('documentSaved') || '文档已保存');
    } catch (error) {
        console.error('Failed to save CLAUDE.md sections:', error);
        showToast(t('saveFailed') || '保存失败');
    }
}

/**
 * 更新保存按钮状态
 */
function updateSaveButtonState() {
    const saveBtn = document.getElementById('claudeMDSaveBtn');
    if (saveBtn) {
        saveBtn.disabled = !isClaudeMDDirty;
        saveBtn.style.opacity = isClaudeMDDirty ? '1' : '0.5';
    }
}

// ============================================
// Utility
// ============================================

/**
 * 获取当前语言
 * @returns {string} 'zh' 或 'en'
 */
function getCurrentLang() {
    return typeof currentLang !== 'undefined' ? currentLang : 'zh';
}
