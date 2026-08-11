// @ts-check
// CLAUDE.md Generator — 从项目自动生成 CLAUDE.md

// ============================================
// State
// ============================================

let generatorProjects = [];
let selectedProjectForGen = null;
let generatedContent = '';

// ============================================
// Generator Modal
// ============================================

/**
 * 打开自动生成模态框
 */
async function openGeneratorModal() {
    const modal = document.getElementById('generatorModal');
    if (!modal) return;

    modal.style.display = 'flex';

    // 加载项目列表
    await loadGeneratorProjects();
}

/**
 * 关闭自动生成模态框
 */
function closeGeneratorModal() {
    const modal = document.getElementById('generatorModal');
    if (modal) {
        modal.style.display = 'none';
    }
    selectedProjectForGen = null;
    generatedContent = '';
}

/**
 * 加载项目列表
 */
async function loadGeneratorProjects() {
    const list = document.getElementById('generatorProjectList');
    if (!list) return;

    list.innerHTML = `<div class="empty-state"><p>${t('loading') || '加载中...'}</p></div>`;

    try {
        // 获取所有有 CLAUDE.md 的项目
        const projects = await window.go.main.App.GetCLAUDEMDProjects();
        generatorProjects = projects;

        if (projects.length === 0) {
            list.innerHTML = `<div class="empty-state"><p>${t('noProjects') || '未找到项目'}</p></div>`;
            return;
        }

        list.innerHTML = projects.map((proj, idx) => `
            <div class="generator-project-item" onclick="selectProjectForGen(${idx})">
                <div class="project-icon">
                    <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z"/>
                    </svg>
                </div>
                <div class="project-info">
                    <div class="project-name">${escapeHtml(formatProjectNameForGen(proj.name))}</div>
                    <div class="project-status">
                        ${proj.hasClaudeMD
                            ? `<span class="status-has">${t('hasCLAUDE') || '已有 CLAUDE.md'}</span>`
                            : `<span class="status-none">${t('noCLAUDE') || '无 CLAUDE.md'}</span>`}
                    </div>
                </div>
            </div>
        `).join('');
    } catch (error) {
        console.error('Failed to load projects:', error);
        list.innerHTML = `<div class="empty-state"><p>${t('loadFailed') || '加载失败'}</p></div>`;
    }
}

/**
 * 选择项目进行生成
 * @param {number} index - 项目索引
 */
async function selectProjectForGen(index) {
    const proj = generatorProjects[index];
    if (!proj) return;

    selectedProjectForGen = proj;

    // 更新选中状态
    document.querySelectorAll('.generator-project-item').forEach((item, idx) => {
        item.classList.toggle('active', idx === index);
    });

    // 检测项目信息
    const preview = document.getElementById('generatorPreview');
    if (!preview) return;

    preview.innerHTML = `<div class="empty-state"><p>${t('detecting') || '检测中...'}</p></div>`;

    try {
        // 如果有 rootDir，使用它；否则尝试从路径推断
        const projectDir = proj.rootDir || '';

        if (projectDir) {
            // 检测项目信息
            const info = await window.go.main.App.DetectProjectInfo(projectDir);
            renderProjectInfo(info);
        }

        // 生成 CLAUDE.md
        const content = await window.go.main.App.GenerateClaudeMDFromProject(projectDir);
        generatedContent = content;

        // 显示预览
        renderGeneratedPreview(content);
    } catch (error) {
        console.error('Failed to detect project:', error);
        preview.innerHTML = `<div class="empty-state"><p>${t('detectionFailed') || '检测失败'}</p></div>`;
    }
}

/**
 * 渲染项目检测信息
 * @param {object} info - 项目信息
 */
function renderProjectInfo(info) {
    const infoEl = document.getElementById('generatorProjectInfo');
    if (!infoEl) return;

    infoEl.innerHTML = `
        <div class="project-detect-result">
            <div class="detect-row">
                <span class="detect-label">${t('language') || '语言'}:</span>
                <span class="detect-value">${info.languageIcon} ${escapeHtml(info.language)}</span>
            </div>
            ${info.framework ? `
            <div class="detect-row">
                <span class="detect-label">${t('framework') || '框架'}:</span>
                <span class="detect-value">${escapeHtml(info.framework)}</span>
            </div>` : ''}
            ${info.buildTool ? `
            <div class="detect-row">
                <span class="detect-label">${t('buildTool') || '构建工具'}:</span>
                <span class="detect-value">${escapeHtml(info.buildTool)}</span>
            </div>` : ''}
            <div class="detect-row">
                <span class="detect-label">${t('features') || '特性'}:</span>
                <span class="detect-value">
                    ${info.hasTests ? '✓ Tests' : '✗ Tests'}
                    ${info.hasCI ? '✓ CI' : '✗ CI'}
                    ${info.hasDocker ? '✓ Docker' : '✗ Docker'}
                </span>
            </div>
        </div>
    `;
}

/**
 * 渲染生成的 CLAUDE.md 预览
 * @param {string} content - 生成的内容
 */
function renderGeneratedPreview(content) {
    const preview = document.getElementById('generatorPreview');
    if (!preview) return;

    preview.innerHTML = `
        <div class="generated-preview markdown-body">
            ${renderMarkdown(content)}
        </div>
    `;
}

// ============================================
// Actions
// ============================================

/**
 * 复制生成的内容到剪贴板
 */
async function copyGeneratedContent() {
    if (!generatedContent) return;

    try {
        await navigator.clipboard.writeText(generatedContent);
        showToast(t('copiedToClipboard') || '已复制到剪贴板');
    } catch (error) {
        console.error('Failed to copy:', error);
        showToast(t('copyFailed') || '复制失败');
    }
}

/**
 * 将生成的内容保存到项目
 */
async function saveGeneratedToProject() {
    if (!generatedContent || !selectedProjectForGen) return;

    try {
        const path = await window.go.main.App.CreateKnowledgeDocument(
            'claudemd',
            'CLAUDE.md',
            generatedContent,
            selectedProjectForGen.name
        );
        showToast(t('savedToProject') || '已保存到项目');
        closeGeneratorModal();

        // 刷新知识库列表
        if (typeof loadKnowledgeDocuments === 'function') {
            loadKnowledgeDocuments('claudemd');
        }
    } catch (error) {
        console.error('Failed to save:', error);
        showToast(t('saveFailed') || '保存失败');
    }
}

// ============================================
// Utility
// ============================================

/**
 * 格式化项目名称用于显示
 * @param {string} name - 原始项目名
 * @returns {string} 格式化后的名称
 */
function formatProjectNameForGen(name) {
    if (name === 'global') return 'Global (~/.claude/)';
    return formatProjectName(name);
}
