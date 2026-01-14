#!/usr/bin/env bash
#
# 自动为 VS Code Codex 扩展的 webview bundle 注入最新 codex-max 模型，
# 同时处理 apikey 和 chatgpt (OAuth) 两种认证方式的模型列表，
# 并清空 CHAT_GPT_AUTH_ONLY_MODELS，确保所有模型都能用 apikey 访问。
#
# 用法示例：
#   # 通过 curl 直接执行（无需下载）
#   curl -sL https://raw.githubusercontent.com/huangang/codex-autopatch/main/patch_models.sh | bash
#   curl -sL https://raw.githubusercontent.com/huangang/codex-autopatch/main/patch_models.sh | bash -s -- --include-mini
#
#   # 本地执行
#   ./patch_models.sh                          # 自动发现并 patch
#   ./patch_models.sh --include-mini           # 包含 -mini 模型
#   ./patch_models.sh /path/to/index-foo.js    # 指定文件
#   ./patch_models.sh --restore                # 从 .bak 恢复
#
set -euo pipefail

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info()  { echo -e "${BLUE}[info]${NC}    $*"; }
log_ok()    { echo -e "${GREEN}[ok]${NC}      $*"; }
log_warn()  { echo -e "${YELLOW}[warn]${NC}    $*"; }
log_err()   { echo -e "${RED}[error]${NC}   $*"; }

# 默认参数
INCLUDE_MINI=false
RESTORE_MODE=false
FILES=()

# 解析命令行参数
parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --include-mini)
                INCLUDE_MINI=true
                shift
                ;;
            --restore)
                RESTORE_MODE=true
                shift
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            -*)
                log_err "未知选项: $1"
                show_help
                exit 1
                ;;
            *)
                FILES+=("$1")
                shift
                ;;
        esac
    done
}

show_help() {
    cat << 'EOF'
用法: patch_models.sh [选项] [文件...]

自动为 VS Code Codex 扩展 patch 最新模型列表。

选项:
  --include-mini    包含 -mini 模型（默认排除）
  --restore         从 .bak 备份恢复原文件
  -h, --help        显示此帮助信息

示例:
  # 通过 curl 直接执行（推荐）
  curl -sL https://raw.githubusercontent.com/huangang/codex-autopatch/main/patch_models.sh | bash

  # 包含 mini 模型
  curl -sL https://raw.githubusercontent.com/huangang/codex-autopatch/main/patch_models.sh | bash -s -- --include-mini

  # 本地执行
  ./patch_models.sh                          # 自动发现并 patch
  ./patch_models.sh --include-mini           # 包含 -mini 模型
  ./patch_models.sh /path/to/index-foo.js    # 指定文件
  ./patch_models.sh --restore                # 从 .bak 恢复
EOF
}

# 自动发现扩展目录中的 index-*.js 文件
auto_discover() {
    local found=()
    local extensions_dir="$HOME/.vscode/extensions"

    if [[ ! -d "$extensions_dir" ]]; then
        return
    fi

    # 查找所有 openai.chatgpt* 目录
    while IFS= read -r -d '' ext_dir; do
        local webview_dir="$ext_dir/webview/assets"
        if [[ -d "$webview_dir" ]]; then
            while IFS= read -r -d '' js_file; do
                found+=("$js_file")
            done < <(find "$webview_dir" -maxdepth 1 -name 'index-*.js' -print0 2>/dev/null)
        fi
    done < <(find "$extensions_dir" -maxdepth 1 -type d -name 'openai.chatgpt*' -print0 2>/dev/null)

    printf '%s\n' "${found[@]}"
}

# 自动发现 .bak 备份文件
auto_discover_baks() {
    local found=()
    local extensions_dir="$HOME/.vscode/extensions"

    if [[ ! -d "$extensions_dir" ]]; then
        return
    fi

    while IFS= read -r -d '' ext_dir; do
        local webview_dir="$ext_dir/webview/assets"
        if [[ -d "$webview_dir" ]]; then
            while IFS= read -r -d '' bak_file; do
                found+=("$bak_file")
            done < <(find "$webview_dir" -maxdepth 1 -name 'index-*.js.bak' -print0 2>/dev/null)
        fi
    done < <(find "$extensions_dir" -maxdepth 1 -type d -name 'openai.chatgpt*' -print0 2>/dev/null)

    printf '%s\n' "${found[@]}"
}

# 从文件内容中提取所有 gpt-5* 模型
find_gpt5_models() {
    local content="$1"
    echo "$content" | grep -oE 'gpt-5[a-zA-Z0-9._-]*' | sort -u
}

# 从文件内容中提取所有 gpt-X-codex-max 模型
find_codex_max_versions() {
    local content="$1"
    echo "$content" | grep -oE 'gpt-[0-9]+(\.[0-9]+)?-codex-max' | sort -u -t'-' -k2 -rV
}

# 规范化模型名称：gpt-5-1 -> gpt-5.1
normalize_name() {
    local name="$1"
    # 移除引号
    name="${name//\"/}"
    name="${name//\'/}"
    # gpt-5-X -> gpt-5.X
    if [[ "$name" =~ ^gpt-5-([0-9]+)(.*)$ ]]; then
        echo "gpt-5.${BASH_REMATCH[1]}${BASH_REMATCH[2]}"
    else
        echo "$name"
    fi
}

# 排序模型列表：codex-max 优先，然后 codex，然后其他，mini 最后
sort_models() {
    local models="$1"
    local include_mini="$2"

    # 过滤 mini 模型（如果不包含）
    if [[ "$include_mini" != "true" ]]; then
        models=$(echo "$models" | grep -v -i 'mini' || true)
    fi

    # 规范化并去重
    local normalized=()
    while IFS= read -r model; do
        [[ -z "$model" ]] && continue
        normalized+=("$(normalize_name "$model")")
    done <<< "$models"

    # 去重
    local unique_models
    unique_models=$(printf '%s\n' "${normalized[@]}" | sort -u)

    # 分组排序
    local codex_max=()
    local codex=()
    local mini=()
    local others=()

    while IFS= read -r model; do
        [[ -z "$model" ]] && continue
        if [[ "$model" == *"codex-max"* ]]; then
            codex_max+=("$model")
        elif [[ "$model" == *"codex-mini"* ]]; then
            mini+=("$model")
        elif [[ "$model" == *"codex"* ]]; then
            codex+=("$model")
        else
            others+=("$model")
        fi
    done <<< "$unique_models"

    # 版本排序（降序）
    local sorted_codex_max sorted_codex sorted_mini sorted_others
    sorted_codex_max=$(printf '%s\n' "${codex_max[@]}" 2>/dev/null | sort -rV || true)
    sorted_codex=$(printf '%s\n' "${codex[@]}" 2>/dev/null | sort -rV || true)
    sorted_mini=$(printf '%s\n' "${mini[@]}" 2>/dev/null | sort -rV || true)
    sorted_others=$(printf '%s\n' "${others[@]}" 2>/dev/null | sort -rV || true)

    # 按优先级输出
    echo "$sorted_codex_max"
    echo "$sorted_codex"
    echo "$sorted_others"
    echo "$sorted_mini"
}

# 构建模型列表数组字符串
build_model_array() {
    local models="$1"
    local result=""
    local first=true

    while IFS= read -r model; do
        [[ -z "$model" ]] && continue
        if [[ "$first" == "true" ]]; then
            result="\"$model\""
            first=false
        else
            result="$result,\"$model\""
        fi
    done <<< "$models"

    echo "[$result]"
}

# 替换认证方式数组
replace_auth_array() {
    local content="$1"
    local field="$2"
    local new_array="$3"

    # 替换 field:[...] 形式（包括多行）
    # 使用 perl 处理多行正则
    local result
    result=$(echo "$content" | perl -0pe "s/${field}:\\s*\\[[^\\]]*\\]/${field}:${new_array}/s")

    # 如果没有变化，尝试替换变量引用形式 field:VARIABLE_NAME
    if [[ "$result" == "$content" ]]; then
        result=$(echo "$content" | sed -E "s/${field}:[A-Z][A-Z0-9_]*/${field}:${new_array}/")
    fi

    echo "$result"
}

# 清空 CHAT_GPT_AUTH_ONLY_MODELS
clear_auth_only() {
    local content="$1"
    echo "$content" | sed -E 's/CHAT_GPT_AUTH_ONLY_MODELS=new Set\(\[[^]]*\]\)/CHAT_GPT_AUTH_ONLY_MODELS=new Set([])/g'
}

# Patch 单个文件
patch_file() {
    local file="$1"
    local include_mini="$2"

    if [[ ! -f "$file" ]]; then
        log_err "$file 不存在"
        return 1
    fi

    # 创建备份
    local bak_file="${file}.bak"
    if [[ ! -f "$bak_file" ]]; then
        cp "$file" "$bak_file"
        log_info "[backup]  $bak_file"
    fi

    # 读取文件内容
    local content
    content=$(cat "$file")

    # 提取所有模型
    local gpt5_models codex_max_models all_models
    gpt5_models=$(find_gpt5_models "$content")
    codex_max_models=$(find_codex_max_versions "$content")
    all_models=$(printf '%s\n%s' "$gpt5_models" "$codex_max_models" | sort -u)

    # 如果没有找到任何模型，使用默认值
    if [[ -z "$all_models" ]]; then
        all_models="gpt-5.1-codex-max"
    fi

    # 排序模型
    local sorted_models
    sorted_models=$(sort_models "$all_models" "$include_mini")

    # 构建模型数组
    local model_array
    model_array=$(build_model_array "$sorted_models")

    # 替换 apikey 和 chatgpt 数组
    local new_content
    new_content=$(replace_auth_array "$content" "apikey" "$model_array")
    new_content=$(replace_auth_array "$new_content" "chatgpt" "$model_array")

    # 清空 AUTH_ONLY
    new_content=$(clear_auth_only "$new_content")

    # 检查是否有变化
    if [[ "$new_content" != "$content" ]]; then
        echo "$new_content" > "$file"
        log_ok "[patched] $file"
    else
        log_warn "[skip]    $file (已经是最新状态)"
    fi
}

# 恢复文件
restore_files() {
    local bak_files=("$@")

    # 如果没有指定文件，自动发现
    if [[ ${#bak_files[@]} -eq 0 ]]; then
        while IFS= read -r f; do
            [[ -n "$f" ]] && bak_files+=("$f")
        done <<< "$(auto_discover_baks)"
    fi

    if [[ ${#bak_files[@]} -eq 0 ]]; then
        log_warn "没有找到可恢复的 .bak 文件"
        return 1
    fi

    for bak_file in "${bak_files[@]}"; do
        if [[ ! -f "$bak_file" ]]; then
            log_err "$bak_file 不存在"
            continue
        fi

        local orig_file="${bak_file%.bak}"
        cp "$bak_file" "$orig_file"
        log_ok "[restored] $orig_file <- $bak_file"
    done

    log_info "提示：如仍异常，建议重新安装插件或手动替换原文件。"
}

# 主函数
main() {
    parse_args "$@"

    # 恢复模式
    if [[ "$RESTORE_MODE" == "true" ]]; then
        restore_files "${FILES[@]}"
        return $?
    fi

    # 收集目标文件
    local targets=("${FILES[@]}")

    # 如果没有指定文件，自动发现
    if [[ ${#targets[@]} -eq 0 ]]; then
        while IFS= read -r f; do
            [[ -n "$f" ]] && targets+=("$f")
        done <<< "$(auto_discover)"
    fi

    if [[ ${#targets[@]} -eq 0 ]]; then
        log_err "没有找到需要 patch 的文件"
        log_info "请确保已安装 VS Code Codex 扩展，或手动指定文件路径"
        return 1
    fi

    log_info "找到 ${#targets[@]} 个文件需要处理..."

    # Patch 每个文件
    for file in "${targets[@]}"; do
        patch_file "$file" "$INCLUDE_MINI"
    done

    echo ""
    log_ok "操作完成！请重启 VS Code 以加载新资源。"
}

main "$@"
