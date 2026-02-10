#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Lynx 一键 go mod tidy
在仓库内所有包含 go.mod 的模块（主库、CLI、layout、各插件）中执行 go mod tidy，
无需逐个进入目录执行。

依赖：本机已安装 Python 3、Go，且 go 在 PATH 中。
用法（在仓库根目录执行）：
  python3 lynx/script/tidy_all.py
退出码：全部成功为 0，有失败为 1。
"""

import os
import subprocess
import sys
import time
from pathlib import Path

# 项目根 = 脚本所在目录的上两级（lynx/script/tidy_all.py -> 仓库根）
ROOT = Path(__file__).resolve().parent.parent.parent

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


def tidy_module(root: Path, module_rel: str, timeout_sec: int = 120) -> dict:
    """
    在指定模块目录执行 go mod tidy。
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
    try:
        result = subprocess.run(
            ["go", "mod", "tidy"],
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
            "stderr": (e.stderr or b"").decode("utf-8", errors="replace") if e.stderr else f"执行超时（{timeout_sec}s）",
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
    print("  Lynx 全量 go mod tidy")
    print("  项目根目录:", ROOT)
    print("=" * 60)

    modules = find_go_modules(ROOT)
    if not modules:
        print("未找到任何 go.mod，请确认在 lynx 项目根目录下运行本脚本。")
        sys.exit(1)

    print(f"\n共发现 {len(modules)} 个模块，开始执行 go mod tidy...\n")

    results = []
    for i, rel in enumerate(modules, 1):
        print(f"[{i}/{len(modules)}] {rel} ... ", end="", flush=True)
        r = tidy_module(ROOT, rel)
        results.append(r)
        if r["ok"]:
            print(f"完成 ({r['duration_sec']:.2f}s)")
        else:
            print("失败")

    # 详细结果（仅展示失败或有输出的）
    failed = [r for r in results if not r["ok"]]
    has_output = [r for r in results if r["stdout"].strip() or r["stderr"].strip()]

    if failed or has_output:
        print("\n" + "=" * 60)
        print("  详细结果")
        print("=" * 60)
        for r in results:
            if not r["ok"] or r["stdout"].strip() or r["stderr"].strip():
                status = "完成" if r["ok"] else "失败"
                print(f"\n【{r['module']}】 {status} ({r['duration_sec']:.2f}s)")
                if not r["ok"] or r["stdout"] or r["stderr"]:
                    if r["stdout"]:
                        for line in r["stdout"].strip().splitlines():
                            print("   ", line)
                    if r["stderr"]:
                        for line in r["stderr"].strip().splitlines():
                            print("   ", line)
                    if not r["ok"]:
                        print("   returncode:", r["returncode"])

    # 汇总
    print("\n" + "=" * 60)
    print("  汇总")
    print("=" * 60)
    passed = [r for r in results if r["ok"]]
    print(f"  总模块数: {len(results)}")
    print(f"  成功:     {len(passed)}")
    print(f"  失败:     {len(failed)}")
    if failed:
        print("\n  失败模块:")
        for r in failed:
            print(f"    - {r['module']}")
    print("=" * 60)

    sys.exit(0 if not failed else 1)


if __name__ == "__main__":
    main()
