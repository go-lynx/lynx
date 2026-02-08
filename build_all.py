#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Lynx 一键构建脚本
构建 lynx 主库、CLI、layout 及所有插件，并输出详细结果。

依赖：本机已安装 Python 3、Go，且 go 在 PATH 中。
用法：脚本位于 lynx/ 目录下，以仓库根（lynx 的上一级）为项目根扫描并构建所有模块。
  在任意目录执行均可，例如：
  python build_all.py
  python3 lynx/build_all.py
退出码：全部通过为 0，有失败为 1。
"""

import os
import subprocess
import sys
import time
from pathlib import Path

# 项目根 = 脚本所在目录的上一级（脚本在 lynx/build_all.py 时，根为仓库根，可发现 lynx 与所有 lynx-* 插件）
ROOT = Path(__file__).resolve().parent.parent

# 期望的模块顺序：先主库和 CLI，再 layout，其余按字母序
PREFERRED_ORDER = [
    "lynx",           # 主框架
    "lynx/cmd/lynx",  # CLI 工具
    "lynx-layout",    # 布局/示例
    "lynx-layout/api",
]


def find_go_modules(root: Path) -> list[str]:
    """查找所有包含 go.mod 的模块根目录（相对 root 的路径字符串）。"""
    modules = []
    for path in root.rglob("go.mod"):
        parent = path.parent
        try:
            rel = parent.relative_to(root)
        except ValueError:
            continue
        # 只保留在 root 下的目录，且每个目录只算一个模块
        rel_str = str(rel).replace("\\", "/")
        if rel_str not in [str(m).replace("\\", "/") for m in modules]:
            modules.append(rel_str)
    return sorted(modules, key=_module_sort_key)


def _module_sort_key(name: str) -> tuple:
    """排序：优先顺序内的靠前，其余按字母序。"""
    name_norm = name.replace("\\", "/")
    try:
        idx = PREFERRED_ORDER.index(name_norm)
        return (0, idx, name_norm)
    except ValueError:
        return (1, 0, name_norm)


def build_module(root: Path, module_rel: str, timeout_sec: int = 300) -> dict:
    """
    在指定模块目录执行 go build ./...
    返回 {"module": str, "ok": bool, "returncode": int, "duration_sec": float, "stdout": str, "stderr": str}
    """
    work_dir = root / module_rel.replace("/", os.sep)
    if not work_dir.is_dir():
        return {
            "module": module_rel,
            "ok": False,
            "returncode": -1,
            "duration_sec": 0.0,
            "stdout": "",
            "stderr": f"目录不存在: {work_dir}",
        }
    start = time.perf_counter()
    go_cmd = "go"
    try:
        result = subprocess.run(
            [go_cmd, "build", "./..."],
            cwd=str(work_dir),
            capture_output=True,
            text=True,
            timeout=timeout_sec,
            shell=False,
        )
    except FileNotFoundError:
        return {
            "module": module_rel,
            "ok": False,
            "returncode": -1,
            "duration_sec": time.perf_counter() - start,
            "stdout": "",
            "stderr": "未找到 go 命令，请确保 Go 已安装并加入 PATH",
        }
    except subprocess.TimeoutExpired as e:
        return {
            "module": module_rel,
            "ok": False,
            "returncode": -1,
            "duration_sec": timeout_sec,
            "stdout": (e.stdout or b"").decode("utf-8", errors="replace") if e.stdout else "",
            "stderr": (e.stderr or b"").decode("utf-8", errors="replace") if e.stderr else f"构建超时（{timeout_sec}s）",
        }
    duration = time.perf_counter() - start
    return {
        "module": module_rel,
        "ok": result.returncode == 0,
        "returncode": result.returncode,
        "duration_sec": duration,
        "stdout": result.stdout or "",
        "stderr": result.stderr or "",
    }


def main():
    print("=" * 60)
    print("  Lynx 全量构建")
    print("  项目根目录:", ROOT)
    print("=" * 60)

    modules = find_go_modules(ROOT)
    if not modules:
        print("未找到任何 go.mod，请确认在 lynx 项目根目录下运行本脚本。")
        sys.exit(1)

    print(f"\n共发现 {len(modules)} 个模块，开始构建...\n")

    results = []
    for i, rel in enumerate(modules, 1):
        print(f"[{i}/{len(modules)}] 构建: {rel} ... ", end="", flush=True)
        r = build_module(ROOT, rel)
        results.append(r)
        if r["ok"]:
            print(f"通过 ({r['duration_sec']:.2f}s)")
        else:
            print("失败")

    # 详细结果
    print("\n" + "=" * 60)
    print("  详细结果")
    print("=" * 60)

    passed = [r for r in results if r["ok"]]
    failed = [r for r in results if not r["ok"]]

    for r in results:
        status = "通过" if r["ok"] else "失败"
        duration = f"{r['duration_sec']:.2f}s"
        print(f"\n【{r['module']}】 {status} (耗时: {duration})")
        if not r["ok"]:
            if r["stdout"]:
                print("  --- stdout ---")
                for line in r["stdout"].strip().splitlines():
                    print("   ", line)
            if r["stderr"]:
                print("  --- stderr ---")
                for line in r["stderr"].strip().splitlines():
                    print("   ", line)
            print("   returncode:", r["returncode"])

    # 汇总
    print("\n" + "=" * 60)
    print("  汇总")
    print("=" * 60)
    print(f"  总模块数: {len(results)}")
    print(f"  通过:     {len(passed)}")
    print(f"  失败:     {len(failed)}")
    if failed:
        print("\n  失败模块:")
        for r in failed:
            print(f"    - {r['module']}")
    print("=" * 60)

    sys.exit(0 if not failed else 1)


if __name__ == "__main__":
    main()
