# Codex webview model patchers

This repo provides Python, Node.js, and Go scripts to patch the VS Code Codex webview bundle.

## What it does

- Updates model lists for both apikey and chatgpt (OAuth) auth flows
- Clears `CHAT_GPT_AUTH_ONLY_MODELS` so all models can be used via apikey
- Creates `.bak` backups before patching

## Usage

Python:

```
python3 patch_models.py --auto
python3 patch_models.py --auto --include-mini
python3 patch_models.py /path/to/index-foo.js /path/to/index-bar.js
python3 patch_models.py --restore
python3 patch_models.py --restore /path/to/index-foo.js.bak
```

Node.js:

```
node patch_models.js --auto
node patch_models.js --auto --include-mini
node patch_models.js /path/to/index-foo.js /path/to/index-bar.js
node patch_models.js --restore
node patch_models.js --restore /path/to/index-foo.js.bak
```

Go:

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

## 中文说明

这个仓库提供 Python、Node.js 和 Go 版本的脚本，用于 patch VS Code Codex 插件的 webview bundle。

功能说明：

- 同时更新 apikey 和 chatgpt (OAuth) 的模型列表
- 清空 `CHAT_GPT_AUTH_ONLY_MODELS`，确保 apikey 可用全部模型
- patch 前自动生成 `.bak` 备份

用法示例：

Python:

```
python3 patch_models.py --auto
python3 patch_models.py --auto --include-mini
python3 patch_models.py /path/to/index-foo.js /path/to/index-bar.js
python3 patch_models.py --restore
python3 patch_models.py --restore /path/to/index-foo.js.bak
```

Node.js:

```
node patch_models.js --auto
node patch_models.js --auto --include-mini
node patch_models.js /path/to/index-foo.js /path/to/index-bar.js
node patch_models.js --restore
node patch_models.js --restore /path/to/index-foo.js.bak
```

Go:

```
go run patch_models.go --auto
go run patch_models.go --auto --include-mini
go run patch_models.go /path/to/index-foo.js /path/to/index-bar.js
go run patch_models.go --restore
go run patch_models.go --restore /path/to/index-foo.js.bak
```

提示：patch 完成后请重启 VS Code 插件以加载新资源。
