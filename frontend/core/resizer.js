// 拖拽调整侧边栏宽度
(function() {
    const resizer = document.getElementById('resizer');
    const sidebar = document.getElementById('sidebar');

    if (!resizer || !sidebar) return;

    let isResizing = false;
    let startX = 0;
    let startWidth = 0;

    // 鼠标按下
    resizer.addEventListener('mousedown', (e) => {
        isResizing = true;
        startX = e.clientX;
        startWidth = sidebar.offsetWidth;
        resizer.classList.add('active');
        document.body.style.cursor = 'col-resize';
        document.body.style.userSelect = 'none';
    });

    // 鼠标移动
    document.addEventListener('mousemove', (e) => {
        if (!isResizing) return;

        const diff = e.clientX - startX;
        const newWidth = Math.min(Math.max(startWidth + diff, 240), 480);
        sidebar.style.width = newWidth + 'px';
    });

    // 鼠标释放
    document.addEventListener('mouseup', () => {
        if (!isResizing) return;

        isResizing = false;
        resizer.classList.remove('active');
        document.body.style.cursor = '';
        document.body.style.userSelect = '';
    });
})();
