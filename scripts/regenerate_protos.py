#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Lynx 全量 Proto 重新生成脚本

在「有 .proto 文件」的各个模块下执行 Makefile 的 config 目标，统一重新生成 Go 代码。
- 若模块尚无 Makefile，则从 lynx/script/Makefile.plugin 复制一份再执行 make config。
- 若模块已有 Makefile（如 lynx、lynx-layout），直接执行 make config，不覆盖。

依赖：Python 3、Go、protoc、make；建议先在本仓库根目录执行一次 lynx 的 make init。
用法：在仓库根目录执行
  python3 lynx/script/regenerate_protos.py
  python3 lynx/script/regenerate_protos.py --dry-run
  python3 lynx/script/regenerate_protos.py --modules lynx-etcd lynx-redis
退出码：全部成功为 0，有失败为 1。
"""

import argparse
import os
import shutil
import subprocess
import sys
from pathlib import Path
from typing import List, Optional

# 仓库根 = 脚本所在目录的上两级（lynx/script/regenerate_protos.py -> 根）
ROOT = Path(__file__).resolve().parent.parent.parent

# 脚本目录（lynx/script/），内含 Makefile.plugin
LYNX_SCRIPT_DIR = Path(__file__).resolve().parent
MAKEFILE_PLUGIN = LYNX_SCRIPT_DIR / "Makefile.plugin"

# 不单独处理的模块：其 proto 由父目录的 Makefile 生成（如 lynx-layout/api 由 lynx-layout 的 make api 生成）
SKIP_MODULES = {"lynx-layout/api"}


def find_modules_with_protos(root: Path) -> list[Path]:
    """找出所有包含至少一个 .proto 文件的「模块根」目录（含 go.mod 的目录）。"""
    proto_dirs: set[Path] = set()
    for proto_path in root.rglob("*.proto"):
        try:
            rel = proto_path.relative_to(root)
        except ValueError:
            continue
        # 忽略 third_party 等依赖里的 proto，只关心我们要生成的
        parts = rel.parts
        if "third_party" in parts or "vendor" in parts:
            continue
        proto_dirs.add(proto_path.parent)

    # 对每个含 proto 的目录，找到其「模块根」（包含 go.mod 的最近上级）
    module_roots: set[Path] = set()
    for dir_with_proto in proto_dirs:
        cur = root / dir_with_proto
        while cur != root and cur != root.parent:
            if (cur / "go.mod").exists():
                module_roots.add(cur)
                break
            cur = cur.parent

    return sorted(module_roots, key=lambda p: (p.name.lower(), str(p)))


def should_skip_module(module_root: Path) -> bool:
    """是否跳过该模块（不复制 Makefile、不执行 make）。"""
    try:
        rel = str(module_root.relative_to(ROOT)).replace("\\", "/")
        return rel in SKIP_MODULES
    except ValueError:
        return False


def ensure_plugin_makefile(module_root: Path, makefile_plugin: Path, dry_run: bool) -> bool:
    """若模块根下没有 Makefile，则复制 Makefile.plugin 过去。返回是否已存在或已复制。"""
    makefile_path = module_root / "Makefile"
    if makefile_path.exists():
        return True
    if dry_run:
        print(f"  [dry-run] would copy Makefile.plugin -> {makefile_path.relative_to(ROOT)}")
        return True
    try:
        shutil.copy2(makefile_plugin, makefile_path)
        print(f"  copied Makefile -> {makefile_path.relative_to(ROOT)}")
        return True
    except OSError as e:
        print(f"  failed to copy Makefile: {e}", file=sys.stderr)
        return False


def run_make_config(module_root: Path, dry_run: bool, make_targets: Optional[List[str]] = None) -> tuple[bool, str]:
    """在 module_root 下执行 make 目标（默认 config）。返回 (成功?, 摘要信息)。"""
    rel = module_root.relative_to(ROOT)
    targets = make_targets if make_targets is not None else ["config"]
    targets_str = " ".join(targets)
    if dry_run:
        print(f"  [dry-run] would run: make {targets_str} (in {rel})")
        return True, f"{rel} (dry-run)"

    try:
        result = subprocess.run(
            ["make"] + targets,
            cwd=str(module_root),
            capture_output=True,
            text=True,
            timeout=120,
        )
        ok = result.returncode == 0
        err = (result.stderr or "").strip()
        out = (result.stdout or "").strip()
        if not ok and (err or out):
            return False, f"{rel}\nstdout:\n{out}\nstderr:\n{err}"
        return ok, str(rel)
    except subprocess.TimeoutExpired:
        return False, f"{rel} (timeout)"
    except FileNotFoundError:
        return False, f"{rel} (make not found)"


def main() -> int:
    parser = argparse.ArgumentParser(
        description="在各含 .proto 的模块下执行 make config，统一重新生成 Go 代码。"
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="只打印将要执行的步骤，不复制 Makefile、不执行 make",
    )
    parser.add_argument(
        "--modules",
        nargs="*",
        metavar="DIR",
        help="仅处理这些模块（相对仓库根的目录名，如 lynx-etcd lynx）；默认处理所有含 proto 的模块",
    )
    parser.add_argument(
        "--no-sync-makefile",
        action="store_true",
        help="不向无 Makefile 的模块复制 Makefile.plugin，仅对已有 Makefile 的目录执行 make config",
    )
    args = parser.parse_args()

    if not MAKEFILE_PLUGIN.exists():
        print(f"Makefile 模板不存在: {MAKEFILE_PLUGIN}", file=sys.stderr)
        return 1

    modules = find_modules_with_protos(ROOT)
    # 排除由父目录 Makefile 负责生成的子模块（如 lynx-layout/api）
    modules = [m for m in modules if not should_skip_module(m)]
    if args.modules:
        allowed = {s.rstrip("/") for s in args.modules}
        modules = [m for m in modules if m.name in allowed or str(m.relative_to(ROOT)).replace("\\", "/") in allowed]
        if not modules:
            print("未匹配到任何模块。", file=sys.stderr)
            return 1

    print("=" * 60)
    print("  Lynx 全量 Proto 重新生成")
    print("  仓库根:", ROOT)
    print("  待处理模块数:", len(modules))
    if args.dry_run:
        print("  [dry-run] 不执行写文件与 make")
    print("=" * 60)

    failed: list[str] = []
    for mod in modules:
        rel = mod.relative_to(ROOT)
        rel_str = str(rel).replace("\\", "/")
        print(f"\n[{rel}]")
        if not args.no_sync_makefile:
            if not ensure_plugin_makefile(mod, MAKEFILE_PLUGIN, args.dry_run):
                failed.append(f"{rel} (copy Makefile failed)")
                continue
        # lynx-layout 需同时执行 config 与 api 才能生成 internal + api 的 proto
        make_targets = ["config", "api"] if rel_str == "lynx-layout" else None
        ok, msg = run_make_config(mod, args.dry_run, make_targets)
        if not ok:
            failed.append(msg)
            print(f"  failed: {msg}")
        else:
            print(f"  ok: {msg}")

    print("\n" + "=" * 60)
    if failed:
        print("  失败:", len(failed))
        for f in failed:
            print(f"    - {f}")
        return 1
    print("  全部成功")
    return 0


if __name__ == "__main__":
    sys.exit(main())
