// @ts-check
// Skills Tab — Agent Skills 管理页面逻辑

// Skills 状态
let currentSkills = [];
let currentSkill = null;
let currentSkillScope = 'all';
let isSkillEditing = false;

// ============================================
// Initialization
// ============================================

async function loadSkills(scope = 'all', project = '') {
    try {
        const skills = await window.go.main.App.GetSkills(scope, project);
        currentSkills = skills;
        renderSkillList(skills);
    } catch (error) {
        console.error('Failed to load skills:', error);
        showToast(t('loadFailed') || '加载失败');
    }
}

// ============================================
// Skill List
// ============================================

function renderSkillList(skills) {
    const container = document.getElementById('skillDocList');
    if (!container) return;

    if (!skills || skills.length === 0) {
        container.innerHTML = `
            <div class="empty-state">
                <p>${t('noSkills') || '暂无 Skills'}</p>
                <p class="empty-hint">${t('skillsEmptyHint') || '创建您的第一个 Agent Skill'}</p>
            </div>
        `;
        return;
    }

    // 按作用域分组
    const groups = {};
    skills.forEach(skill => {
        const scope = skill.scope || 'user';
        if (!groups[scope]) {
            groups[scope] = [];
        }
        groups[scope].push(skill);
    });

    // 渲染分组列表
    let html = '';
    const scopeLabels = {
        'user': 'User Skills',
        'project': 'Project Skills',
        'plugin': 'Plugin Skills'
    };

    for (const [scope, scopeSkills] of Object.entries(groups)) {
        const displayName = scopeLabels[scope] || scope;
        html += `
            <div class="skill-group">
                <div class="group-header" onclick="toggleSkillGroup(this)">
                    <svg class="group-icon" width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                        <path d="M6 9l6 6 6-6"/>
                    </svg>
                    <span class="group-name">${escapeHtml(displayName)}</span>
                    <span class="group-count">${scopeSkills.length}</span>
                </div>
                <div class="group-items">
                    ${scopeSkills.map(skill => renderSkillItem(skill)).join('')}
                </div>
            </div>
        `;
    }

    container.innerHTML = html;

    // 默认折叠所有分组
    container.querySelectorAll('.skill-group').forEach(group => {
        group.classList.add('collapsed');
    });
}

// 渲染单个 Skill 项
function renderSkillItem(skill) {
    const hasDescription = skill.meta && skill.meta.description;
    const description = hasDescription ? skill.meta.description : t('noDescription') || '无描述';

    return `
        <div class="skill-doc-item ${currentSkill?.path === skill.path ? 'active' : ''}"
             data-path="${escapeHtmlAttr(skill.path)}"
             onclick="selectSkill(this.dataset.path)">
            <div class="doc-icon skill-icon">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
                    <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/>
                </svg>
            </div>
            <div class="doc-info">
                <div class="doc-title">${escapeHtml(skill.meta?.name || skill.name || 'Unnamed Skill')}</div>
                <div class="doc-meta">
                    <span class="skill-description">${escapeHtml(description)}</span>
                    <span class="skill-scope-badge ${skill.scope}">${skill.scope}</span>
                </div>
            </div>
        </div>
    `;
}

// 切换分组展开/折叠
function toggleSkillGroup(header) {
    const group = header.closest('.skill-group');
    if (group) {
        group.classList.toggle('collapsed');
    }
}

// ============================================
// Skill Selection
// ============================================

async function selectSkill(path) {
    try {
        const skill = await window.go.main.App.GetSkill(path);
        currentSkill = skill;

        // 更新列表高亮
        document.querySelectorAll('.skill-doc-item').forEach(item => {
            const itemPath = item.getAttribute('data-path');
            item.classList.toggle('active', itemPath === path);
        });

        // 更新工具栏
        const skillName = document.getElementById('skillDocName');
        const skillScope = document.getElementById('skillDocScope');
        if (skillName) skillName.textContent = skill.meta?.name || skill.name || 'Unnamed Skill';
        if (skillScope) {
            skillScope.textContent = skill.scope;
            skillScope.className = `skill-scope-badge ${skill.scope}`;
        }

        // 显示预览
        renderSkillPreview(skill);
        exitSkillEdit();
    } catch (error) {
        console.error('Failed to load skill:', error);
    }
}

// ============================================
// Preview / Editor
// ============================================

function renderSkillPreview(skill) {
    const preview = document.getElementById('skillPreview');
    if (!preview) return;

    if (!skill) {
        preview.innerHTML = `
            <div class="empty-state">
                <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                    <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/>
                </svg>
                <p>${t('selectSkill') || '选择 Skill 查看内容'}</p>
            </div>
        `;
        return;
    }

    // 解析 frontmatter 并提取 body
    const { frontmatter, body } = parseSkillFrontmatter(skill.content);

    // 构建预览 HTML
    let html = '';

    // 元数据卡片
    html += '<div class="skill-meta-card">';
    html += `<div class="skill-meta-title">${escapeHtml(skill.meta?.name || 'Unnamed Skill')}</div>`;

    if (frontmatter && Object.keys(frontmatter).length > 0) {
        html += '<div class="skill-meta-fields">';
        for (const [key, value] of Object.entries(frontmatter)) {
            if (key !== 'metadata' && value) {
                html += `<div class="skill-meta-field"><span class="field-key">${escapeHtml(key)}:</span> <span class="field-value">${escapeHtml(value)}</span></div>`;
            }
        }
        html += '</div>';
    }

    // 显示特殊字段
    if (skill.meta?.paths && skill.meta.paths.length > 0) {
        html += `<div class="skill-meta-field"><span class="field-key">paths:</span> <span class="field-value">${skill.meta.paths.map(p => escapeHtml(p)).join(', ')}</span></div>`;
    }
    if (skill.meta?.allowedTools && skill.meta.allowedTools.length > 0) {
        html += `<div class="skill-meta-field"><span class="field-key">allowed-tools:</span> <span class="field-value">${skill.meta.allowedTools.map(t => escapeHtml(t)).join(', ')}</span></div>`;
    }
    if (skill.meta?.disallowedTools && skill.meta.disallowedTools.length > 0) {
        html += `<div class="skill-meta-field"><span class="field-key">disallowed-tools:</span> <span class="field-value">${skill.meta.disallowedTools.map(t => escapeHtml(t)).join(', ')}</span></div>`;
    }

    html += '</div>';

    // 渲染 Markdown body
    html += '<div class="skill-content markdown-body">';
    html += renderSkillMarkdown(body);
    html += '</div>';

    preview.innerHTML = html;
}

// 解析 Skill 的 YAML frontmatter
function parseSkillFrontmatter(content) {
    const frontmatter = {};
    let body = content;

    if (!content || !content.startsWith('---')) {
        return { frontmatter, body };
    }

    const endIndex = content.indexOf('---', 3);
    if (endIndex === -1) {
        return { frontmatter, body };
    }

    const fmContent = content.substring(3, endIndex);
    body = content.substring(endIndex + 3).trim();

    const lines = fmContent.split('\n');
    for (const line of lines) {
        const trimmed = line.trim();
        if (trimmed === '' || trimmed.startsWith('#')) {
            continue;
        }

        const colonIndex = trimmed.indexOf(':');
        if (colonIndex > 0) {
            const key = trimmed.substring(0, colonIndex).trim();
            const value = trimmed.substring(colonIndex + 1).trim();
            frontmatter[key] = value;
        }
    }

    return { frontmatter, body };
}

// 渲染 Skill Markdown
function renderSkillMarkdown(content) {
    if (!content) return '';

    // 先转义 HTML 特殊字符
    let escaped = content
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');

    // 按行处理 Markdown
    const lines = escaped.split('\n');
    let html = '';
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

        // 处理标题
        const trimmedLine = line.replace(/^ {1,8}/, '');
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

        // 处理空行
        if (line.trim() === '') {
            html += '<br>';
            continue;
        }

        // 处理普通段落
        html += `<p>${processSkillInlineMarkdown(line)}</p>`;
    }

    return html;
}

// 处理行内 Markdown
function processSkillInlineMarkdown(text) {
    return text
        .replace(/`(.*?)`/g, '<code>$1</code>')
        .replace(/\*\*(.*?)\*\*/g, '<strong>$1</strong>')
        .replace(/\*(.*?)\*/g, '<em>$1</em>');
}

// ============================================
// Editor
// ============================================

function toggleSkillEdit() {
    if (!currentSkill) return;

    isSkillEditing = true;
    const preview = document.getElementById('skillPreview');
    const editor = document.getElementById('skillEditor');
    const editBtn = document.getElementById('skillEditBtn');
    const saveBtn = document.getElementById('skillSaveBtn');
    const textarea = document.getElementById('skillEditorContent');

    if (preview) preview.style.display = 'none';
    if (editor) editor.style.display = 'flex';
    if (editBtn) editBtn.style.display = 'none';
    if (saveBtn) saveBtn.style.display = 'inline-flex';
    if (textarea) textarea.value = currentSkill.content;
}

function exitSkillEdit() {
    isSkillEditing = false;
    const preview = document.getElementById('skillPreview');
    const editor = document.getElementById('skillEditor');
    const editBtn = document.getElementById('skillEditBtn');
    const saveBtn = document.getElementById('skillSaveBtn');

    if (preview) preview.style.display = 'block';
    if (editor) editor.style.display = 'none';
    if (editBtn) editBtn.style.display = 'inline-flex';
    if (saveBtn) saveBtn.style.display = 'none';
}

// ============================================
// Document Operations
// ============================================

async function saveSkillDoc() {
    if (!currentSkill) return;

    const textarea = document.getElementById('skillEditorContent');
    if (!textarea) return;

    const content = textarea.value;

    // 验证 Skill
    const errors = await window.go.main.App.ValidateSkill(content);
    if (errors && errors.length > 0) {
        const errorMsg = errors.map(e => `${e.field}: ${e.message}`).join('\n');
        if (!confirm(t('skillValidationWarning') || 'Skill 验证有警告，是否继续保存？\n' + errorMsg)) {
            return;
        }
    }

    try {
        // 提取名称
        const { frontmatter } = parseSkillFrontmatter(content);
        const name = frontmatter.name || currentSkill.meta?.name || 'unnamed';

        await window.go.main.App.SaveSkill(currentSkill.scope, name, content, currentSkill.project || '');
        currentSkill.content = content;
        exitSkillEdit();
        renderSkillPreview(currentSkill);
        showToast(t('skillSaved') || 'Skill 已保存');
    } catch (error) {
        console.error('Failed to save skill:', error);
        showToast(t('saveFailed') || '保存失败');
    }
}

async function deleteSkillDoc() {
    if (!currentSkill) return;

    const confirmMsg = t('confirmDeleteSkill') || '确定要删除这个 Skill 吗？';
    if (!confirm(confirmMsg)) return;

    try {
        await window.go.main.App.DeleteSkill(currentSkill.path);
        currentSkill = null;
        loadSkills(currentSkillScope);
        showToast(t('skillDeleted') || 'Skill 已删除');

        // 清空预览区
        const preview = document.getElementById('skillPreview');
        if (preview) {
            preview.innerHTML = `
                <div class="empty-state">
                    <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5">
                        <path d="M14.7 6.3a1 1 0 0 0 0 1.4l1.6 1.6a1 1 0 0 0 1.4 0l3.77-3.77a6 6 0 0 1-7.94 7.94l-6.91 6.91a2.12 2.12 0 0 1-3-3l6.91-6.91a6 6 0 0 1 7.94-7.94l-3.76 3.76z"/>
                    </svg>
                    <p>${t('selectSkill') || '选择 Skill 查看内容'}</p>
                </div>
            `;
        }

        // 清空工具栏
        const skillName = document.getElementById('skillDocName');
        const skillScope = document.getElementById('skillDocScope');
        if (skillName) skillName.textContent = t('selectSkill') || '选择 Skill 查看';
        if (skillScope) skillScope.textContent = '';
    } catch (error) {
        console.error('Failed to delete skill:', error);
        showToast(t('deleteFailed') || '删除失败');
    }
}

async function createNewSkill() {
    openCreateSkillModal();
}

// ============================================
// Create Skill Modal
// ============================================

let selectedSkillScope = 'user';

async function openCreateSkillModal() {
    const modal = document.getElementById('createSkillModal');
    if (!modal) return;

    modal.style.display = 'flex';
    setTimeout(() => modal.classList.add('show'), 10);
    selectedSkillScope = 'user';

    // 加载项目列表
    await loadSkillProjectSelectList();
}

function closeCreateSkillModal() {
    const modal = document.getElementById('createSkillModal');
    if (modal) {
        modal.classList.remove('show');
        setTimeout(() => modal.style.display = 'none', 200);
    }
}

function toggleSkillScopeSelect() {
    const scope = document.querySelector('input[name="skillScope"]:checked')?.value;
    const projectGroup = document.getElementById('skillProjectSelectGroup');
    if (projectGroup) {
        projectGroup.style.display = scope === 'project' ? 'block' : 'none';
    }
}

async function loadSkillProjectSelectList() {
    const container = document.getElementById('skillProjectSelectList');
    if (!container) return;

    try {
        const projects = await window.go.main.App.GetKnowledgeProjects();
        if (!projects || projects.length === 0) {
            container.innerHTML = '<div class="empty-state"><p>暂无项目</p></div>';
            return;
        }

        container.innerHTML = projects.map(p => `
            <div class="project-select-item" data-project="${escapeHtmlAttr(p)}" onclick="selectSkillProjectItem(this)">
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

function selectSkillProjectItem(item) {
    document.querySelectorAll('#skillProjectSelectList .project-select-item').forEach(el => el.classList.remove('active'));
    item.classList.add('active');
    selectedSkillScope = item.dataset.project;
}

async function confirmCreateSkill() {
    const scope = document.querySelector('input[name="skillScope"]:checked')?.value;
    if (!scope) {
        showToast('请选择作用域');
        return;
    }

    if (scope === 'project' && !selectedSkillScope) {
        showToast('请选择项目');
        return;
    }

    const nameInput = document.getElementById('newSkillName');
    const descInput = document.getElementById('newSkillDescription');
    const name = nameInput?.value?.trim() || 'my-skill';
    const description = descInput?.value?.trim() || 'Description of what this skill does';

    try {
        // 生成模板
        const content = await window.go.main.App.GenerateSkillTemplate(name, description);

        // 保存
        const path = await window.go.main.App.SaveSkill(scope, name, content, scope === 'project' ? selectedSkillScope : '');

        closeCreateSkillModal();
        await loadSkills(currentSkillScope);
        await selectSkill(path);
        toggleSkillEdit();
    } catch (error) {
        console.error('Failed to create skill:', error);
        showToast(t('createFailed') || '创建失败');
    }
}

// ============================================
// Filtering & Search
// ============================================

function filterSkillByScope(scope) {
    currentSkillScope = scope;
    document.querySelectorAll('.skill-filters .filter-btn').forEach(btn => {
        btn.classList.toggle('active', btn.getAttribute('data-scope') === scope);
    });
    loadSkills(scope);
}

let skillSearchTimeout;
function searchSkills() {
    clearTimeout(skillSearchTimeout);
    skillSearchTimeout = setTimeout(async () => {
        const input = document.getElementById('skillSearchInput');
        if (!input) return;

        const query = input.value.toLowerCase();
        if (!query) {
            renderSkillList(currentSkills);
            return;
        }

        // 本地过滤
        const filtered = currentSkills.filter(skill => {
            const name = skill.meta?.name || '';
            const desc = skill.meta?.description || '';
            return name.toLowerCase().includes(query) || desc.toLowerCase().includes(query);
        });
        renderSkillList(filtered);
    }, 300);
}

// ============================================
// Utility Functions
// ============================================

function escapeHtml(str) {
    if (!str) return '';
    return str
        .replace(/&/g, '&amp;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;')
        .replace(/"/g, '&quot;')
        .replace(/'/g, '&#039;');
}

function escapeHtmlAttr(str) {
    return escapeHtml(str);
}

function formatProjectName(dirName) {
    if (dirName === 'plans') return 'Plans';
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
