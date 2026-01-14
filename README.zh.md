# Codex webview model patchers

English: [`README.md`](README.md) | 中文: [`README.zh.md`](README.zh.md)

这个仓库提供 Shell、Python、Node.js 和 Go 版本的脚本，用于 patch VS Code Codex 插件的 webview bundle。

## 功能说明

- 同时更新 apikey 和 chatgpt (OAuth) 的模型列表
- 清空 `CHAT_GPT_AUTH_ONLY_MODELS`，确保 apikey 可用全部模型
- patch 前自动生成 `.bak` 备份

## 用法

### 快速开始（无需下载）

通过 curl 直接执行：

```bash
# 自动发现并 patch（推荐）
curl -sL https://raw.githubusercontent.com/huangang/codex-autopatch/main/patch_models.sh | bash

# 包含 mini 模型
curl -sL https://raw.githubusercontent.com/huangang/codex-autopatch/main/patch_models.sh | bash -s -- --include-mini

# 从备份恢复
curl -sL https://raw.githubusercontent.com/huangang/codex-autopatch/main/patch_models.sh | bash -s -- --restore
```

### Shell

```bash
./patch_models.sh                          # 自动发现并 patch
./patch_models.sh --include-mini           # 包含 -mini 模型
./patch_models.sh /path/to/index-foo.js    # 指定文件
./patch_models.sh --restore                # 从 .bak 恢复
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

## 说明

- `--auto` 会扫描默认扩展目录，例如 `~/.vscode/extensions/openai.chatgpt*`
- patch 完成后请重启 VS Code 插件以加载新资源
