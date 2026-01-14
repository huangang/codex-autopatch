# Codex webview model patchers

English: [`README.md`](README.md) | 中文: [`README.zh.md`](README.zh.md)

This repo provides Shell, Python, Node.js, and Go scripts to patch the VS Code Codex webview bundle.

## What it does

- Updates model lists for both apikey and chatgpt (OAuth) auth flows
- Clears `CHAT_GPT_AUTH_ONLY_MODELS` so all models can be used via apikey
- Creates `.bak` backups before patching

## Usage

### Quick Start (no download required)

Run directly via curl:

```bash
# Auto-patch (recommended)
curl -sL https://raw.githubusercontent.com/huangang/codex-autopatch/main/patch_models.sh | bash

# Include mini models
curl -sL https://raw.githubusercontent.com/huangang/codex-autopatch/main/patch_models.sh | bash -s -- --include-mini

# Restore from backup
curl -sL https://raw.githubusercontent.com/huangang/codex-autopatch/main/patch_models.sh | bash -s -- --restore
```

### Shell

```bash
./patch_models.sh                          # Auto-discover and patch
./patch_models.sh --include-mini           # Include -mini models
./patch_models.sh /path/to/index-foo.js    # Patch specific file
./patch_models.sh --restore                # Restore from .bak backup
```

### Python

```
python3 patch_models.py --auto
python3 patch_models.py --auto --include-mini
python3 patch_models.py /path/to/index-foo.js /path/to/index-bar.js
python3 patch_models.py --restore
python3 patch_models.py --restore /path/to/index-foo.js.bak
```

### Node.js

```
node patch_models.js --auto
node patch_models.js --auto --include-mini
node patch_models.js /path/to/index-foo.js /path/to/index-bar.js
node patch_models.js --restore
node patch_models.js --restore /path/to/index-foo.js.bak
```

### Go

```
go run patch_models.go --auto
go run patch_models.go --auto --include-mini
go run patch_models.go /path/to/index-foo.js /path/to/index-bar.js
go run patch_models.go --restore
go run patch_models.go --restore /path/to/index-foo.js.bak
```

## Notes

- `--auto` scans default extension locations like `~/.vscode/extensions/openai.chatgpt*`
- After patching, restart VS Code to load the updated webview assets
