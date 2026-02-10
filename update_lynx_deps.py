#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
统一更新各插件（及 layout 等）的 go.mod 中对 lynx 主库的依赖版本。

当 lynx 主库发布新版本（如 1.5.4）后，运行本脚本可一次性把所有依赖
github.com/go-lynx/lynx 的模块的 go.mod 中的版本号改为指定版本，无需逐个手工修改。

依赖：Python 3，本机 Go 可选（若使用 --tidy）。
用法：
  在仓库根或 lynx 目录下执行均可，例如：
  python lynx/update_lynx_deps.py 1.5.4
  python lynx/update_lynx_deps.py v1.5.4 --dry-run
  python lynx/update_lynx_deps.py v1.5.4 --tidy   # 更新后在各模块执行 go mod tidy
"""

import os
import re
import subprocess
import sys
from pathlib import Path

# 项目根 = 脚本所在目录的上一级（与 build_all.py 一致）
ROOT = Path(__file__).resolve().parent.parent

# 匹配 go.mod 中的 github.com/go-lynx/lynx 依赖行（保留缩进与 // indirect 等注释）
LYNX_REQUIRE_RE = re.compile(
    r"^(\s*github\.com/go-lynx/lynx\s+)v[\d.]+(\s*(?://.*)?)$",
    re.MULTILINE,
)


def find_go_modules(root: Path) -> list[str]:
    """查找所有包含 go.mod 的模块根目录（相对 root 的路径字符串）。与 build_all 逻辑一致。"""
    modules = []
    for path in root.rglob("go.mod"):
        parent = path.parent
        try:
            rel = parent.relative_to(root)
        except ValueError:
            continue
        rel_str = str(rel).replace("\\", "/")
        if rel_str not in [str(m).replace("\\", "/") for m in modules]:
            modules.append(rel_str)
    return sorted(modules)


def is_main_lynx_module(root: Path, module_rel: str) -> bool:
    """判断是否为 lynx 主库模块（主库的 go.mod 不需要改自己的 require）。"""
    go_mod_path = root / module_rel.replace("/", os.sep) / "go.mod"
    if not go_mod_path.is_file():
        return False
    with open(go_mod_path, "r", encoding="utf-8") as f:
        first_line = f.readline()
    return first_line.strip() == "module github.com/go-lynx/lynx"


def update_lynx_version_in_gomod(content: str, new_version: str) -> tuple[str, int]:
    """
    在 go.mod 文件内容中，把所有 github.com/go-lynx/lynx vX.Y.Z 替换为 new_version。
    new_version 应为带 v 的版本号，如 v1.5.4。
    返回 (新内容, 替换次数)。
    """
    count = 0

    def repl(m):
        nonlocal count
        count += 1
        return f"{m.group(1)}{new_version}{m.group(2)}"

    new_content = LYNX_REQUIRE_RE.sub(repl, content)
    return new_content, count


def run_go_mod_tidy(root: Path, module_rel: str, dry_run: bool) -> bool:
    """在指定模块目录执行 go mod tidy。"""
    work_dir = root / module_rel.replace("/", os.sep)
    if dry_run:
        return True
    try:
        subprocess.run(
            ["go", "mod", "tidy"],
            cwd=str(work_dir),
            check=True,
            capture_output=True,
            text=True,
            timeout=60,
        )
        return True
    except (subprocess.CalledProcessError, FileNotFoundError, subprocess.TimeoutExpired):
        return False


def main():
    import argparse

    # 避免 Windows 控制台 cp932 无法输出中文
    if hasattr(sys.stdout, "reconfigure"):
        try:
            sys.stdout.reconfigure(encoding="utf-8", errors="replace")
        except Exception:
            pass

    parser = argparse.ArgumentParser(
        description="统一更新各插件 go.mod 中对 github.com/go-lynx/lynx 的依赖版本",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )
    parser.add_argument(
        "version",
        help="目标版本号，如 1.5.4 或 v1.5.4",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="仅打印将要修改的内容，不写入文件",
    )
    parser.add_argument(
        "--tidy",
        action="store_true",
        help="更新后在每个修改过的模块下执行 go mod tidy",
    )
    args = parser.parse_args()

    version = args.version.strip()
    if not re.match(r"^v?[\d.]+$", version):
        print(f"错误：版本号格式不正确，应为纯数字或 v 开头，例如 1.5.4 或 v1.5.4", file=sys.stderr)
        sys.exit(1)
    if not version.startswith("v"):
        version = f"v{version}"

    modules = find_go_modules(ROOT)
    if not modules:
        print("未找到任何 go.mod，请确认在 lynx 仓库根目录下运行。", file=sys.stderr)
        sys.exit(1)

    updated = []
    skipped_main = []
    no_lynx = []

    for module_rel in modules:
        go_mod_path = ROOT / module_rel.replace("/", os.sep) / "go.mod"
        if not go_mod_path.is_file():
            continue
        if is_main_lynx_module(ROOT, module_rel):
            skipped_main.append(module_rel)
            continue

        with open(go_mod_path, "r", encoding="utf-8") as f:
            content = f.read()

        new_content, count = update_lynx_version_in_gomod(content, version)
        if count == 0:
            no_lynx.append(module_rel)
            continue

        updated.append((module_rel, count, content, new_content))

    # 输出结果
    print("=" * 60)
    print("  统一更新 lynx 依赖版本")
    print("  项目根:", ROOT)
    print("  目标版本:", version)
    print("  dry-run:", args.dry_run)
    print("=" * 60)

    if skipped_main:
        print("\n跳过主库模块（无需修改）:", ", ".join(skipped_main))

    if no_lynx:
        print("\n未依赖 lynx 的模块（未改动）:", ", ".join(no_lynx))

    if not updated:
        print("\n没有需要更新的 go.mod。")
        sys.exit(0)

    print(f"\n将更新以下 {len(updated)} 个模块中的 lynx 版本为 {version}:")
    for module_rel, count, old_content, new_content in updated:
        print(f"  - {module_rel} （{count} 处）")
        if args.dry_run:
            # 只显示被替换的 base lynx 依赖行
            for line in new_content.splitlines():
                if re.match(r"^\s*github\.com/go-lynx/lynx\s+" + re.escape(version), line):
                    print(f"      {line.strip()}")

    if args.dry_run:
        print("\n[--dry-run] 未写入任何文件。")
        sys.exit(0)

    for module_rel, count, old_content, new_content in updated:
        go_mod_path = ROOT / module_rel.replace("/", os.sep) / "go.mod"
        with open(go_mod_path, "w", encoding="utf-8") as f:
            f.write(new_content)
        print(f"  已写入: {go_mod_path.relative_to(ROOT)}")

    if args.tidy:
        print("\n正在对已修改的模块执行 go mod tidy ...")
        for module_rel, _, _, _ in updated:
            ok = run_go_mod_tidy(ROOT, module_rel, dry_run=False)
            if ok:
                print(f"  ok: {module_rel}")
            else:
                print(f"  失败: {module_rel}", file=sys.stderr)

    print("\n完成。")


if __name__ == "__main__":
    main()
