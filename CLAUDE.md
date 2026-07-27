# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Argus is a desktop application that monitors Claude Code sessions, providing cross-session visibility into token usage, file changes, conversation history, and session continuity. It runs as an external observer outside Claude Code itself.

## Tech Stack

- **Backend**: Go 1.23 + Wails v2 (desktop framework)
- **Frontend**: Vanilla HTML/CSS/JS (no framework), Chart.js for dashboards
- **Platform**: Windows (Wails v2 supports cross-platform, but currently Windows-only)
- **Storage**: Local JSON files in `~/.argus/`

## Development Commands

```bash
# Install dependencies
go mod tidy

# Start dev mode (with hot reload)
wails dev

# Build production binary
wails build

# Run tests
go test ./...

# Run tests for a specific package
go test ./internal/session/...
```

## Architecture

```
~/.claude/projects/**/*.jsonl  ← Claude Code session files
    │
    ├──→ [session/claude/reader] ──→ Session ──→ [app.go API] ──→ Frontend UI
    ├──→ [analytics] ──→ Token statistics
    ├──→ [diff/engine] ──→ Git diff
    ├──→ [knowledge] ──→ Plans/Memory/CLAUDE.md
    ├──→ [continuity] ──→ Session handoff summaries (requires LLM)
    ├──→ [compliance] ──→ CLAUDE.md rule compliance audit (LLM-powered)
    ├──→ [contexthealth] ──→ Context health analysis (peak context, health scoring)
    ├──→ [plugin] ──→ Hook/MCP configuration
    └──→ [monitor] ──→ Real-time file watching (fsnotify)
```

### Key Internal Packages

- `internal/session/` — Core session models and Claude Code JSONL reader
- `internal/analytics/` — Token usage analytics engine
- `internal/diff/` — Git diff engine (platform-specific exec)
- `internal/knowledge/` — CLAUDE.md, Plans, Memory document management
- `internal/continuity/` — Cross-session handoff summary generation (requires LLM)
- `internal/compliance/` — LLM-powered CLAUDE.md rule compliance auditing (extracts rules, audits sessions, caches results)
- `internal/plugin/` — Hook and MCP server configuration management
- `internal/risk/` — File change risk assessment engine
- `internal/monitor/` — fsnotify-based file system watcher
- `internal/settings/` — Application settings manager
- `internal/contexthealth/` — Context health analysis engine (peak context estimation, health scoring, degradation alerts)
- `internal/llm/` — Generic OpenAI-compatible LLM client (used by continuity and compliance)
- `internal/export/` — Session export (HTML/Markdown)

### Frontend Structure

All frontend code lives in `frontend/` — plain HTML/CSS/JS with no build step:
- `app.js` — Main app logic, API calls to Go backend via Wails
- `i18n.js` — Chinese/English internationalization
- `conversation.js` — Session conversation replay
- `dashboard.js` — Token analytics dashboard with Chart.js
- `knowledge.js` — Knowledge base document management
- `claudemd-editor.js` — CLAUDE.md section-based editor
- `continuity.js` — Session handoff summary UI (requires LLM)
- `plugin-studio.js` — Hook/MCP configuration UI
- `compliance.js` — LLM-powered compliance audit UI
- `context-health.js` — Context health dashboard (trend charts, session health table, score visualization)

### Data Flow

Wails binds Go methods on the `App` struct to the frontend. The frontend calls these via `window.go.main.App.MethodName()`. All session data is read from `~/.claude/projects/` — Argus never writes to Claude Code's directories.

### LLM Requirements

Some features require LLM (Large Language Model) to be configured in settings:
- **Cross-session handoff summary** (`continuity`): Requires LLM for generating intelligent session summaries
- **CLAUDE.md compliance audit** (`compliance`): Requires LLM for analyzing rule compliance

## Code Conventions

- Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/) format (e.g., `feat:`, `fix:`, `docs:`)
- Go code follows standard Go conventions
- Chinese comments are used throughout (project is Chinese-first)
- Error messages in Go use `fmt.Errorf("描述: %w", err)` pattern
