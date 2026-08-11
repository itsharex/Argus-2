# Frontend Directory Structure

This directory contains the frontend code for Argus desktop application.

## Directory Organization

```
frontend/
├── index.html                    # Main entry point
├── assets/                       # Static assets
│   ├── appicon.png              # Application icon
│   └── vendor/                  # Third-party libraries
│       └── chart.umd.min.js    # Chart.js library
├── core/                        # Core application files
│   ├── app.js                  # Main application logic
│   ├── i18n.js                 # Internationalization
│   ├── resizer.js              # Resize utility
│   └── style.css               # Global styles
├── modules/                     # Feature modules (organized by functionality)
│   ├── dashboard/              # Dashboard module
│   │   ├── dashboard.js
│   │   └── dashboard.css
│   ├── sessions/               # Session management module
│   │   ├── conversation.js
│   │   ├── conversation.css
│   │   ├── search.js
│   │   ├── search.css
│   │   └── hook-monitor.js
│   ├── knowledge/              # Knowledge base module
│   │   ├── knowledge.js
│   │   └── knowledge.css
│   ├── claude-md/              # CLAUDE.md editor module
│   │   ├── claudemd-editor.js
│   │   ├── claudemd-editor.css
│   │   └── claudemd-generator.js
│   ├── continuity/             # Cross-session handoff module
│   │   ├── continuity.js
│   │   └── continuity.css
│   ├── compliance/             # Compliance audit module
│   │   ├── compliance.js
│   │   └── compliance.css
│   ├── plugin-studio/          # Plugin studio module
│   │   ├── plugin-studio.js
│   │   └── plugin-studio.css
│   ├── skills/                 # Skills management module
│   │   ├── skills.js
│   │   └── skills.css
│   ├── context-health/         # Context health module
│   │   └── context-health.js
│   ├── productivity/           # Productivity module
│   │   ├── productivity.js
│   │   └── productivity.css
│   └── agent-tree/             # Agent tree module
│       ├── agent-tree.js
│       └── agent-tree.css
├── components/                  # Reusable components
│   └── splash/                 # Splash screen
│       └── splash.css
└── wailsjs/                     # Wails bindings (auto-generated)
    ├── go/
    └── runtime/
```

## Module Descriptions

- **core/**: Core application files (main logic, i18n, global styles)
- **modules/dashboard/**: Dashboard with token usage analytics
- **modules/sessions/**: Session management, conversation replay, search
- **modules/knowledge/**: Knowledge base document management
- **modules/claude-md/**: CLAUDE.md file editor and generator
- **modules/continuity/**: Cross-session handoff summary (requires LLM)
- **modules/compliance/**: CLAUDE.md rule compliance audit (requires LLM)
- **modules/plugin-studio/**: Hook and MCP server configuration
- **modules/skills/**: Agent skills management
- **modules/context-health/**: Context health analysis and scoring
- **modules/productivity/**: Productivity metrics and tracking
- **modules/agent-tree/**: Agent tree visualization
- **components/splash/**: Application splash screen

## Development

This is a vanilla JavaScript project with no build step. All frontend code runs directly in the browser via Wails.

### Key Files

- `index.html` - Main HTML entry point
- `core/app.js` - Main application logic and initialization
- `core/i18n.js` - Internationalization (Chinese/English)
- `core/style.css` - Global CSS styles

### Adding New Modules

1. Create a new directory under `modules/`
2. Add your JavaScript and CSS files
3. Update `index.html` to include the new files
4. Follow the existing module structure pattern

### Wails Integration

The `wailsjs/` directory contains auto-generated Wails bindings. Do not modify these files directly - they are regenerated during build.
