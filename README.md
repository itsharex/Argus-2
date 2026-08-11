<p align="center">
  <img src="docs/images/Logo.png" alt="Argus" width="200">
</p>

<p align="center">
  <strong>Argus</strong> — 你的 Claude Code 全局观测站
</p>

<p align="center">
  <img src="https://img.shields.io/badge/go-1.23+-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/Wails-v2-5C2D91?style=flat-square" alt="Wails">
  <img src="https://img.shields.io/badge/windows-✓-lightgrey?style=flat-square&logo=windows&logoColor=white" alt="Windows">
  <img src="https://img.shields.io/badge/license-MIT-green?style=flat-square" alt="MIT">
</p>

---

Claude Code 只能看到当前会话。Token 花了多少？改了哪些文件？规则遵守了吗？跨会话的上下文在哪？

Argus 在外部静默观察，替你回答这一切。

## 为什么是 Argus

Argus 读取 Claude Code 的会话文件（`~/.claude/projects/**/*.jsonl`），**只读，不写入**。所有数据存在 `~/.argus/`，与 Claude Code 完全隔离。纯本地运行，无需网络。

## 能做什么

**仪表盘** — Token 概览、30 天趋势、项目 & 模型维度拆解、月度环比

**上下文健康** — 峰值估算、健康评分、缓存命中率、退化预警

**生产力** — DORA 风格指标：日均会话、平均时长、文件改动、周趋势

**会话管理** — 搜索 / 筛选 / 标签 / 收藏 / 批量操作 / 实时监控

**对话回放** — 用户消息 → AI 思考 → 工具调用链，精确到秒

**全局搜索** — `Ctrl+K` 跨项目全文检索，倒排索引

**Agent 追踪** — 解析 `parentUuid` / `agentId`，构建子 Agent 调用树

**知识库** — CLAUDE.md 分节编辑器 + 智能生成 + Plans & Memory 管理

**会话交接** — 从历史会话提取任务与决策，一键导入新会话 *（需 LLM）*

**合规审计** — 自动提取 CLAUDE.md 规则，逐会话审计遵守情况 *（需 LLM）*

**插件工作室** — Hook & MCP 配置可视化编辑 + 执行监控

**Agent Skills** — 用户级 / 项目级 / 插件级 Skills 统一管理

**风险评估** — 文件改动自动打分（Danger / Review / Safe）

**导出** — HTML / Markdown，支持批量

**体验** — 浅色 / 深色 / 跟随系统 · 中英文 · 骨架屏加载 · 完全离线

> 标注 *（需 LLM）* 的功能需要你在设置中配置 OpenAI 兼容的 API，不配置也完全不影响其他功能。

## 快速开始

从 [Releases](https://github.com/foxpup11/argus/releases/latest) 下载 `argus-desktop.exe`，双击运行。

## 开发

```bash
go mod tidy       # 安装依赖
wails dev         # 开发（热重载）
wails build       # 构建
go test ./...     # 测试
```

## 技术选型

| 层 | 选择 |
|---|---|
| 运行时 | Go 1.23 |
| 桌面框架 | Wails v2 |
| 前端 | 原生 HTML / CSS / JS，零构建 |
| 图表 | Chart.js |
| 存储 | 本地 JSON（`~/.argus/`） |
| 文件监控 | fsnotify |
| LLM | OpenAI 兼容接口（可选） |

## 架构

```
~/.claude/projects/**/*.jsonl
    │
    ├── session  ──→ 会话读取 & 索引
    ├── analytics ──→ Token 统计
    ├── diff     ──→ Git 差异
    ├── continuity ──→ 会话交接 (LLM)
    ├── compliance ──→ 合规审计 (LLM)
    ├── contexthealth ──→ 上下文健康
    ├── productivity ──→ 生产力分析
    ├── skills   ──→ Agent Skills 管理
    ├── plugin   ──→ Hook & MCP 配置
    ├── risk     ──→ 文件改动风险评估
    ├── search   ──→ 全文检索
    ├── export   ──→ 会话导出
    └── monitor  ──→ fsnotify 实时监控
```

## 设计原则

Argus 是观察者，不是参与者——读数据，不写数据，不影响被监控的系统。本地优先，离线可用，隐私安全。

---

<p align="center">
  <sub>Made by <a href="https://github.com/foxpup11">foxpup11</a> · MIT License</sub>
</p>
