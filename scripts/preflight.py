#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Repository preflight checks for release hygiene.

The checks are intentionally lightweight and deterministic:
  - every active Go module is listed in go.work
  - every active module uses the pinned Go toolchain
  - plugin modules do not regress the core Lynx dependency version
  - example configuration files do not contain common committed-secret placeholders
"""

import os
import re
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent.parent
EXPECTED_TOOLCHAIN = "go1.26.2"
EXPECTED_GO_VERSION = "1.26"
EXPECTED_LYNX_VERSION = "v1.6.0-beta"
STALE_LYNX_MODULE_RE = re.compile(r"github\.com/go-lynx/(lynx-[\w-]+)\s+v1\.5\.")

SECRET_PATTERNS = [
    re.compile(r"changeme", re.IGNORECASE),
    re.compile(r"your[_-]?password", re.IGNORECASE),
    re.compile(r"your[_-]?secret", re.IGNORECASE),
    re.compile(r"your[_-]?token", re.IGNORECASE),
    re.compile(r"your-[a-z0-9_-]*(?:secret|token|password)", re.IGNORECASE),
    re.compile(r"polaris-token", re.IGNORECASE),
    re.compile(r"admin:123456", re.IGNORECASE),
    re.compile(r"lynx123456", re.IGNORECASE),
    re.compile(r"guest:guest", re.IGNORECASE),
    re.compile(r"user:password@", re.IGNORECASE),
    re.compile(r"password:\s*\"(?:password|pass|guest)\"", re.IGNORECASE),
]


def parse_skip_modules() -> set[str]:
    skipped: set[str] = set()
    for item in os.environ.get("LYNX_SKIP_MODULES", "").split(","):
        item = item.strip().strip("/")
        if item:
            skipped.add(item)
    return skipped


def find_modules(skipped: set[str]) -> list[str]:
    modules: list[str] = []
    for gomod in ROOT.rglob("go.mod"):
        rel = gomod.parent.relative_to(ROOT).as_posix()
        if rel.startswith(".github/.git/") or rel in skipped:
            continue
        modules.append(rel)
    return sorted(modules)


def read(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def parse_go_work_modules() -> set[str]:
    work = ROOT / "go.work"
    if not work.exists():
        return set()
    modules: set[str] = set()
    for match in re.finditer(r"\./[A-Za-z0-9_./-]+", read(work)):
        modules.add(match.group(0)[2:].strip("/"))
    return modules


def check_modules(modules: list[str]) -> list[str]:
    errors: list[str] = []
    go_work_modules = parse_go_work_modules()
    for module in modules:
        gomod = ROOT / module / "go.mod"
        text = read(gomod)
        if module not in go_work_modules:
            errors.append(f"{module}: missing from go.work")
        if f"toolchain {EXPECTED_TOOLCHAIN}" not in text and f"go {EXPECTED_GO_VERSION}" not in text:
            errors.append(f"{module}: must use go {EXPECTED_GO_VERSION} or toolchain {EXPECTED_TOOLCHAIN}")
        if module != "lynx" and "github.com/go-lynx/lynx " in text and f"github.com/go-lynx/lynx {EXPECTED_LYNX_VERSION}" not in text:
            errors.append(f"{module}: github.com/go-lynx/lynx must be {EXPECTED_LYNX_VERSION}")
        for match in STALE_LYNX_MODULE_RE.finditer(text):
            errors.append(f"{module}: {match.group(1)} must not depend on stale v1.5.x internal module versions")
    return errors


def check_example_configs(skipped: set[str]) -> list[str]:
    errors: list[str] = []
    candidates = []
    for pattern in ("conf/*.yml", "conf/*.yaml", "configs/*.yml", "configs/*.yaml"):
        candidates.extend(ROOT.glob(f"lynx*/{pattern}"))
    candidates.extend((ROOT / "lynx" / "conf").glob("*.yml"))
    candidates.extend((ROOT / "lynx" / "conf").glob("*.yaml"))

    for path in sorted(set(candidates)):
        rel = path.relative_to(ROOT).as_posix()
        if any(rel == skip or rel.startswith(skip + "/") for skip in skipped):
            continue
        text = read(path)
        for line_no, line in enumerate(text.splitlines(), 1):
            stripped = line.strip()
            if not stripped or stripped.startswith("#"):
                continue
            for pattern in SECRET_PATTERNS:
                if pattern.search(stripped):
                    errors.append(f"{rel}:{line_no}: suspicious committed credential placeholder: {stripped}")
                    break
    return errors


def main() -> int:
    skipped = parse_skip_modules()
    modules = find_modules(skipped)
    errors = []
    errors.extend(check_modules(modules))
    errors.extend(check_example_configs(skipped))

    if errors:
        print("Preflight failed:")
        for err in errors:
            print(f"  - {err}")
        return 1

    print(f"Preflight passed: {len(modules)} modules checked")
    if skipped:
        print("Skipped modules:", ", ".join(sorted(skipped)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
