#!/usr/bin/env python3
r"""
自动为 VS Code Codex 扩展的 webview bundle 注入最新 + 上一版 codex-max 模型，
并移除 CHAT_GPT_AUTH_ONLY_MODELS 对 codex 系列的限制，确保 apikey 也能选择。

特性：
- 自动发现（--auto）插件目录：
  - macOS/Linux: $HOME/.vscode/extensions/openai.chatgpt*
  - Windows: %USERPROFILE%\.vscode\extensions\openai.chatgpt*
  （WSL/特殊安装请手动传文件路径）
- 每个目标文件会生成同目录 .bak 备份，可用 --restore 回滚。
- 补丁完成后请重启 VS Code 以加载新资源。

用法示例：
  python3 patch_models.py --auto
  python3 patch_models.py --auto --include-mini    # 包含 -mini 模型
  python3 patch_models.py /path/to/index-foo.js /path/to/index-bar.js
  python3 patch_models.py --restore
  python3 patch_models.py --restore /path/to/index-foo.js.bak ...
"""
from __future__ import annotations

import argparse
import os
import re
import shutil
import sys
from pathlib import Path
from typing import Iterable, List, Tuple


def parse_default_order(text: str) -> List[str]:
    """Extract DEFAULT_MODEL_ORDER; fallback为空列表，最终会用实际搜索结果填充."""
    m = re.search(r"DEFAULT_MODEL_ORDER=\[([^\]]+)\]", text)
    if not m:
        return []
    items = [p.strip() for p in m.group(1).split(",") if p.strip()]
    return items or []


def find_codex_max_versions(text: str) -> List[str]:
    """Find all gpt-X(.Y)-codex-max strings and sort desc."""
    versions = set(re.findall(r"gpt-[0-9](?:\.[0-9]+)?-codex-max", text))

    def key(v: str) -> Tuple[int, ...]:
        nums = v.split("-")[1]  # e.g. "5.2"
        return tuple(int(x) for x in nums.split("."))

    return sorted(versions, key=key, reverse=True)


def find_gpt5_models(text: str) -> List[str]:
    """Return all gpt-5* model strings found in bundle."""
    models = set(re.findall(r"gpt-5[\w\.-]*", text))
    return sorted(models)


def _strip_quotes(name: str) -> str:
    return name.strip().strip('"').strip("'")


def _normalize_name(name: str) -> str:
    raw = _strip_quotes(name)
    m = re.match(r"^(gpt-5)-([0-9]+)([\w\.-]*)$", raw)
    if not m:
        return raw
    return f"{m.group(1)}.{m.group(2)}{m.group(3)}"


def _quote(name: str) -> str:
    return f'"{_normalize_name(name)}"'


def _version_tuple(name: str) -> Tuple[int, ...]:
    m = re.search(r"gpt-5[.-]([0-9]+(?:\.[0-9]+)?)", name)
    if not m:
        return (0,)
    parts = m.group(1).replace("-", ".").split(".")
    return tuple(int(p) for p in parts)


def _model_sort_key(name: str) -> Tuple[Tuple[int, ...], int, str]:
    v = tuple(-n for n in _version_tuple(name))  # desc
    cat = 2
    if "codex-max" in name:
        cat = 0
    elif "codex" in name and "mini" not in name:
        cat = 1
    elif "codex-mini" in name:
        cat = 3
    return v, cat, name


def order_models(models: Iterable[str]) -> List[str]:
    """Normalize, sort (newest codex-max first), and quote."""
    normalized = {_normalize_name(m) for m in models if _strip_quotes(m)}
    ordered = sorted(normalized, key=_model_sort_key)
    return [_quote(m) for m in ordered]


def build_apikey_list(text: str, include_mini: bool = False) -> List[str]:
    """Compose apikey list: include所有可搜索到的 gpt-5* 变种."""
    default_order = [_strip_quotes(x) for x in parse_default_order(text)]
    codex_versions = find_codex_max_versions(text)
    gpt5_models = find_gpt5_models(text)

    candidates = set(gpt5_models) | set(default_order) | set(codex_versions)
    if not candidates:
        candidates = {"gpt-5.1-codex-max"}

    # 默认过滤掉 -mini 模型，除非显式指定 include_mini
    if not include_mini:
        candidates = {m for m in candidates if "mini" not in m.lower()}

    return order_models(candidates)


def ensure_apikey(text: str, include_mini: bool = False) -> Tuple[str, bool]:
    """Rewrite apikey:... to include desired model list."""
    new_list = build_apikey_list(text, include_mini=include_mini)
    new_list_str = "apikey:[" + ",".join(new_list) + "]"
    pattern = re.compile(
        r"apikey:\s*(?:\[\s*[^]]*?\s*\]|DEFAULT_MODEL_ORDER|\[\s*\.\.\.DEFAULT_MODEL_ORDER\s*\])"
    )
    if pattern.search(text):
        return pattern.sub(new_list_str, text, count=1), True

    m = re.search(r"apikey:\[([^\]]*?)\]", text)
    if m:
        new_text = text[: m.start()] + new_list_str + text[m.end() :]
        return new_text, True
    return text, False


def remove_auth_only(text: str) -> Tuple[str, bool]:
    """Remove codex-only entries from CHAT_GPT_AUTH_ONLY_MODELS."""
    m = re.search(r"CHAT_GPT_AUTH_ONLY_MODELS=new Set\(\[([^\]]*?)\]\)", text)
    if not m:
        return text, False
    items = [p.strip() for p in m.group(1).split(",") if p.strip()]
    filtered = [p for p in items if "codex" not in p]
    if filtered == items:
        return text, False
    replacement = "CHAT_GPT_AUTH_ONLY_MODELS=new Set([" + ",".join(filtered) + "])"
    new_text = text[: m.start()] + replacement + text[m.end() :]
    return new_text, True


def patch_file(path: Path, include_mini: bool = False) -> None:
    bak = path.with_suffix(path.suffix + ".bak")
    if not bak.exists():
        shutil.copy2(path, bak)
        print(f"[backup]  {bak}")

    original = path.read_text()
    text, changed_apikey = ensure_apikey(original, include_mini=include_mini)
    text, changed_auth = remove_auth_only(text)

    if changed_apikey or changed_auth:
        path.write_text(text)
        print(f"[patched] {path}")
    else:
        print(f"[skip]    {path} (already compliant)")


def auto_discover() -> List[Path]:
    home = Path.home()
    roots = [home / ".vscode" / "extensions"]
    if os.name == "nt":
        roots.append(Path(os.environ.get("USERPROFILE", home)) / ".vscode" / "extensions")

    found: List[Path] = []
    for root in roots:
        base = root / "openai.chatgpt"
        parent = base.parent if base else root
        if not parent.exists():
            continue
        for ext in parent.glob("openai.chatgpt*"):
            webview = ext / "webview" / "assets"
            if webview.is_dir():
                found.extend(webview.glob("index-*.js"))
    return found


def auto_discover_baks() -> List[Path]:
    home = Path.home()
    roots = [home / ".vscode" / "extensions"]
    if os.name == "nt":
        roots.append(Path(os.environ.get("USERPROFILE", home)) / ".vscode" / "extensions")

    found: List[Path] = []
    for root in roots:
        base = root / "openai.chatgpt"
        parent = base.parent if base else root
        if not parent.exists():
            continue
        for ext in parent.glob("openai.chatgpt*"):
            webview = ext / "webview" / "assets"
            if webview.is_dir():
                found.extend(webview.glob("index-*.js.bak"))
    return found


def restore(bak_files: List[str]) -> int:
    targets = [Path(bak) for bak in bak_files] if bak_files else auto_discover_baks()
    if not targets:
        print("没有找到可恢复的 .bak 文件。")
        return 1
    for bak_path in targets:
        if not bak_path.exists():
            print(f"[error]   {bak_path} does not exist")
            continue
        orig = bak_path.with_suffix("")
        shutil.copy2(bak_path, orig)
        print(f"[restored] {orig} <- {bak_path}")
    print("提示：如仍异常，建议重新安装插件或手动替换原文件。")
    return 0


def main(argv: List[str]) -> int:
    parser = argparse.ArgumentParser(
        description="Patch VS Code Codex webview bundles to expose latest+previous codex-max for apikey users."
    )
    parser.add_argument("files", nargs="*", help="index-*.js files to patch")
    parser.add_argument("--auto", action="store_true", help="auto-discover index-*.js under default extension dirs")
    parser.add_argument(
        "--restore",
        action="store_true",
        help="restore from .bak files (auto-discover when no file args)",
    )
    parser.add_argument(
        "--include-mini",
        action="store_true",
        help="include -mini models in apikey list (excluded by default)",
    )
    args = parser.parse_args(argv[1:])

    if args.restore:
        return restore(args.files)

    targets: List[Path] = []
    if args.files:
        targets.extend(Path(p) for p in args.files)
    if args.auto:
        targets.extend(auto_discover())

    if not targets:
        print("没有找到需要 patch 的文件。请指定文件或使用 --auto。")
        return 1

    for path in targets:
        if not path.exists():
            print(f"[error]   {path} does not exist")
            continue
        patch_file(path, include_mini=args.include_mini)

    print("操作完成。请重启 VS Code 插件以加载新资源。")
    return 0


if __name__ == "__main__":
    raise SystemExit(main(sys.argv))
