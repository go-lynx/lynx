#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Run tests for every Go module in the repository.

Usage from the repository root:
  python3 lynx/scripts/test_all.py
  python3 lynx/scripts/test_all.py --race --cover

Exit code is 0 only when every module passes.
"""

import argparse
import os
import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent.parent

PREFERRED_ORDER = [
    "lynx",
    "lynx/cmd/lynx",
    "lynx-layout",
    "lynx-layout/api",
]


def module_sort_key(name: str) -> tuple:
    name_norm = name.replace("\\", "/")
    try:
        return (0, PREFERRED_ORDER.index(name_norm), name_norm)
    except ValueError:
        return (1, 0, name_norm)


def find_go_modules(root: Path) -> list[str]:
    modules: list[str] = []
    for path in root.rglob("go.mod"):
        rel = path.parent.relative_to(root).as_posix()
        if rel.startswith((".git/", ".github/.git/")):
            continue
        modules.append(rel)
    return sorted(set(modules), key=module_sort_key)


def parse_skip_modules(values: list[str]) -> set[str]:
    skipped: set[str] = set()
    for value in values:
        for item in value.split(","):
            item = item.strip().strip("/")
            if item:
                skipped.add(item)
    return skipped


def test_module(root: Path, module_rel: str, race: bool, cover: bool, timeout_sec: int) -> dict:
    work_dir = root / module_rel.replace("/", os.sep)
    cmd = ["go", "test", "./..."]
    if race:
        cmd.insert(2, "-race")
    if cover:
        cmd.insert(2, "-cover")

    start = time.perf_counter()
    try:
        result = subprocess.run(
            cmd,
            cwd=str(work_dir),
            capture_output=True,
            text=True,
            timeout=timeout_sec,
            shell=False,
        )
    except FileNotFoundError:
        return result_for(module_rel, False, -1, time.perf_counter() - start, "", "go command not found")
    except subprocess.TimeoutExpired as exc:
        stdout = decode_timeout_output(exc.stdout)
        stderr = decode_timeout_output(exc.stderr) or f"test timeout after {timeout_sec}s"
        return result_for(module_rel, False, -1, timeout_sec, stdout, stderr)

    return result_for(
        module_rel,
        result.returncode == 0,
        result.returncode,
        time.perf_counter() - start,
        result.stdout or "",
        result.stderr or "",
    )


def decode_timeout_output(value) -> str:
    if not value:
        return ""
    if isinstance(value, bytes):
        return value.decode("utf-8", errors="replace")
    return str(value)


def result_for(module: str, ok: bool, returncode: int, duration_sec: float, stdout: str, stderr: str) -> dict:
    return {
        "module": module,
        "ok": ok,
        "returncode": returncode,
        "duration_sec": duration_sec,
        "stdout": stdout,
        "stderr": stderr,
    }


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Run go test ./... in every repository module.")
    parser.add_argument("--race", action="store_true", help="enable the Go race detector")
    parser.add_argument("--cover", action="store_true", help="enable package coverage summaries")
    parser.add_argument("--timeout", type=int, default=300, help="timeout per module in seconds")
    parser.add_argument(
        "--skip-module",
        action="append",
        default=[],
        help="module path to skip; may be repeated or comma-separated",
    )
    return parser.parse_args()


def main() -> int:
    args = parse_args()
    modules = find_go_modules(ROOT)
    skip_values = list(args.skip_module)
    if os.environ.get("LYNX_SKIP_MODULES"):
        skip_values.append(os.environ["LYNX_SKIP_MODULES"])
    skipped = parse_skip_modules(skip_values)
    modules = [module for module in modules if module not in skipped]
    if not modules:
        print(f"No go.mod files found under {ROOT}")
        return 1

    print("=" * 60)
    print("  Lynx all-module test")
    print("  repository root:", ROOT)
    print("  modules:", len(modules))
    if skipped:
        print("  skipped:", ", ".join(sorted(skipped)))
    print("=" * 60)

    results = []
    for index, module in enumerate(modules, 1):
        print(f"[{index}/{len(modules)}] testing {module} ... ", end="", flush=True)
        result = test_module(ROOT, module, args.race, args.cover, args.timeout)
        results.append(result)
        print(f"{'pass' if result['ok'] else 'fail'} ({result['duration_sec']:.2f}s)")

    failed = [result for result in results if not result["ok"]]
    if failed:
        print("\n" + "=" * 60)
        print("  failures")
        print("=" * 60)
        for result in failed:
            print(f"\n[{result['module']}] returncode={result['returncode']}")
            if result["stdout"].strip():
                print("--- stdout ---")
                print(result["stdout"].rstrip())
            if result["stderr"].strip():
                print("--- stderr ---")
                print(result["stderr"].rstrip())

    print("\n" + "=" * 60)
    print("  summary")
    print("=" * 60)
    print(f"  total:  {len(results)}")
    print(f"  passed: {len(results) - len(failed)}")
    print(f"  failed: {len(failed)}")
    return 0 if not failed else 1


if __name__ == "__main__":
    sys.exit(main())
