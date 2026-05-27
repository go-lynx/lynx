#!/usr/bin/env python3
"""
Automatically tag and release all lynx plugins on GitHub

Before creating tags and releases, this script will:
1. Update pluginVersion in each plugin's Go files to match the release version
2. Commit and push the version bump (like release_main.py syncs banner version)
3. Create tag, push tag, and create GitHub release

Usage:
    python3 release_plugins.py <version> [--token <github_token>] [--dry-run]

Examples:
    python3 release_plugins.py v1.0.0
    python3 release_plugins.py v1.0.0 --token ghp_xxxxx
    python3 release_plugins.py v1.0.0 --lynx-version v1.6.1 --dry-run
"""

import os
import sys
import subprocess
import argparse
import re
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any, List, Optional, Tuple
import json


class Colors:
    """Terminal colors"""
    HEADER = '\033[95m'
    OKBLUE = '\033[94m'
    OKCYAN = '\033[96m'
    OKGREEN = '\033[92m'
    WARNING = '\033[93m'
    FAIL = '\033[91m'
    ENDC = '\033[0m'
    BOLD = '\033[1m'


def print_info(msg: str):
    print(f"{Colors.OKCYAN}ℹ️  {msg}{Colors.ENDC}")


def print_success(msg: str):
    print(f"{Colors.OKGREEN}✅ {msg}{Colors.ENDC}")


def print_warning(msg: str):
    print(f"{Colors.WARNING}⚠️  {msg}{Colors.ENDC}")


def print_error(msg: str):
    print(f"{Colors.FAIL}❌ {msg}{Colors.ENDC}")


try:
    import requests
except ImportError:
    requests = None


class HttpResponse:
    """Minimal response wrapper used by the urllib fallback."""

    def __init__(self, status_code: int, text: str):
        self.status_code = status_code
        self.text = text

    def json(self) -> Any:
        return json.loads(self.text or "{}")

    def raise_for_status(self) -> None:
        if self.status_code >= 400:
            raise RuntimeError(f"HTTP {self.status_code}: {self.text}")


def run_cmd(cmd: List[str], check: bool = True, capture_output: bool = False, cwd: Optional[str] = None) -> Tuple[int, str, str]:
    """Execute shell command"""
    try:
        result = subprocess.run(
            cmd,
            check=check,
            capture_output=capture_output,
            text=True,
            cwd=cwd
        )
        stdout = result.stdout.strip() if capture_output else ""
        stderr = result.stderr.strip() if capture_output else ""
        return result.returncode, stdout, stderr
    except subprocess.CalledProcessError as e:
        if capture_output:
            return e.returncode, e.stdout.strip() if e.stdout else "", e.stderr.strip() if e.stderr else ""
        return e.returncode, "", ""


def get_git_remote_url() -> Optional[str]:
    """Get git remote origin URL"""
    code, output, _ = run_cmd(["git", "remote", "get-url", "origin"], check=False, capture_output=True)
    if code != 0:
        return None
    return output.strip()


def parse_github_repo(url: str) -> Optional[Tuple[str, str]]:
    """Parse GitHub owner/repo from git URL"""
    # Support multiple formats:
    # https://github.com/owner/repo.git
    # git@github.com:owner/repo.git
    # https://github.com/owner/repo
    patterns = [
        r'github\.com[:/]([^/]+)/([^/]+?)(?:\.git)?/?$',
    ]
    
    for pattern in patterns:
        match = re.search(pattern, url)
        if match:
            owner = match.group(1)
            repo = match.group(2)
            return owner, repo
    
    return None


def load_plugins_config(config_path: Path) -> List[dict]:
    """Load plugins configuration from JSON file"""
    if not config_path.exists():
        print_error(f"Configuration file not found: {config_path}")
        print_info("Please create plugins.json with plugin information")
        sys.exit(1)
    
    try:
        with open(config_path, 'r', encoding='utf-8') as f:
            config = json.load(f)
        
        plugins = config.get('plugins', [])
        # Filter only enabled plugins
        enabled_plugins = [p for p in plugins if p.get('enabled', True)]
        
        if not enabled_plugins:
            print_error("No enabled plugins found in configuration")
            sys.exit(1)
        
        return enabled_plugins
    except json.JSONDecodeError as e:
        print_error(f"Failed to parse JSON configuration: {e}")
        sys.exit(1)
    except Exception as e:
        print_error(f"Failed to load configuration: {e}")
        sys.exit(1)


def sync_plugin_version(plugin_dir: Path, version: str, dry_run: bool = False) -> Tuple[bool, List[Path]]:
    """
    Update pluginVersion/PluginVersion in all Go files of the plugin to match the release version.
    Handles:
      1. pluginVersion = "v1.0.0" (unexported const)
      2. PluginVersion = "1.0.0" (exported const, e.g. lynx-eon-id)
      3. assert.Equal(t, "v1.0.0", pluginVersion) (test assertions)
      4. assert.Equal(t, "v1.0.0", PluginVersion) (test assertions, e.g. lynx-sentinel)
    Note: \\s* matches any amount of whitespace, so alignment like
      pluginVersion         =   "v1.0.0"  (spaces for const block alignment) is supported.
    Returns (updated: bool, updated_files: List[Path]).
    """
    # Ensure version has 'v' prefix for consistency
    if not version.startswith("v"):
        version = f"v{version}"

    # Patterns to replace (order matters for combined application)
    patterns = [
        # 1. pluginVersion = "v1.0.0" or pluginVersion     = "1.0.0"
        (re.compile(r'(pluginVersion\s*=\s*)"[^"]*"'), rf'\1"{version}"'),
        # 2. PluginVersion = "1.0.0" (exported, excludes PluginVersion = pluginVersion)
        (re.compile(r'(PluginVersion\s*=\s*)"[^"]*"'), rf'\1"{version}"'),
        # 3. assert.Equal(t, "v1.0.0", pluginVersion)
        (re.compile(r'(assert\.Equal\(t,\s*)"[^"]*"(\s*,\s*pluginVersion\))'), rf'\1"{version}"\2'),
        # 4. assert.Equal(t, "v1.0.0", PluginVersion)
        (re.compile(r'(assert\.Equal\(t,\s*)"[^"]*"(\s*,\s*PluginVersion\))'), rf'\1"{version}"\2'),
    ]

    updated_files: List[Path] = []
    for go_file in plugin_dir.rglob("*.go"):
        try:
            content = go_file.read_text(encoding="utf-8")
            new_content = content
            for pattern, replacement in patterns:
                new_content = pattern.sub(replacement, new_content)
            if new_content != content:
                updated_files.append(go_file)
                if not dry_run:
                    go_file.write_text(new_content, encoding="utf-8")
        except Exception as e:
            print_warning(f"Could not process {go_file}: {e}")

    if not updated_files:
        return False, []

    if dry_run:
        for f in updated_files:
            print_info(f"[DRY-RUN] Would update pluginVersion to {version} in {f.relative_to(plugin_dir)}")
        return True, updated_files

    for f in updated_files:
        print_success(f"Updated pluginVersion to {version} in {f.relative_to(plugin_dir)}")
    return True, updated_files


def normalize_version(version: str) -> str:
    """Normalize release versions to the tag format used by Go modules."""
    if version.startswith("v"):
        return version
    return f"v{version}"


def parse_required_module_version(go_mod_text: str, module_path: str) -> Optional[str]:
    """Return the required version for a module in go.mod, if present."""
    pattern = re.compile(rf"^\s*{re.escape(module_path)}\s+([^\s]+)", re.MULTILINE)
    match = pattern.search(go_mod_text)
    if match:
        return match.group(1)
    return None


def has_module_replace(go_mod_text: str, module_path: str) -> bool:
    """Detect committed replace directives for the provided module path."""
    in_replace_block = False
    for line in go_mod_text.splitlines():
        stripped = line.split("//", 1)[0].strip()
        if not stripped:
            continue
        if in_replace_block:
            if stripped == ")":
                in_replace_block = False
                continue
            if stripped == module_path or stripped.startswith(f"{module_path} "):
                return True
            continue
        if stripped == "replace (":
            in_replace_block = True
            continue
        if stripped.startswith(f"replace {module_path} ") or stripped == f"replace {module_path}":
            return True
    return False


def check_lynx_dependency(plugin_dir: Path, expected_version: str) -> bool:
    """Ensure plugin releases depend on the published Lynx SDK, not local replace directives."""
    go_mod = plugin_dir / "go.mod"
    module_path = "github.com/go-lynx/lynx"
    if not go_mod.exists():
        print_error(f"go.mod not found: {go_mod}")
        return False

    text = go_mod.read_text(encoding="utf-8")
    if has_module_replace(text, module_path):
        print_error(f"{plugin_dir.name}: go.mod must not commit replace {module_path}")
        print_info("Use go.work for local development instead of committing replace directives.")
        return False

    actual_version = parse_required_module_version(text, module_path)
    if not actual_version:
        print_error(f"{plugin_dir.name}: go.mod must require {module_path} {expected_version}")
        return False

    if actual_version != expected_version:
        print_error(f"{plugin_dir.name}: {module_path} must be {expected_version}, found {actual_version}")
        print_info("Use --lynx-version if the plugin release version intentionally differs from the SDK version.")
        return False

    print_success(f"Verified Lynx SDK dependency: {module_path} {expected_version}")
    return True


def get_worktree_status(plugin_dir: Path) -> str:
    """Return porcelain status for a plugin repository."""
    code, output, stderr = run_cmd(["git", "status", "--short"], check=False, capture_output=True, cwd=str(plugin_dir))
    if code != 0:
        return stderr or output or "git status failed"
    return output


def ensure_clean_worktree(plugin_dir: Path, dry_run: bool = False) -> bool:
    """Prevent releases from tagging a commit that differs from local files."""
    status = get_worktree_status(plugin_dir)
    if not status:
        return True
    if dry_run:
        print_warning("Plugin working tree has uncommitted changes; real release would stop here.")
        for line in status.splitlines():
            print(f"  {line}")
        return True

    print_error(f"{plugin_dir.name}: working tree has uncommitted changes")
    for line in status.splitlines():
        print(f"  {line}")
    print_info("Commit or discard these changes before creating a release tag.")
    return False


def ensure_upstream_synced(plugin_dir: Path, dry_run: bool = False) -> bool:
    """Require release tags to be created from a branch synchronized with its upstream."""
    code, upstream, _ = run_cmd(
        ["git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{u}"],
        check=False,
        capture_output=True,
        cwd=str(plugin_dir),
    )
    if code != 0 or not upstream:
        message = "No upstream branch configured; cannot verify release commit is pushed."
        if dry_run:
            print_warning(message)
            return True
        print_error(f"{plugin_dir.name}: {message}")
        return False

    code, counts, stderr = run_cmd(
        ["git", "rev-list", "--left-right", "--count", f"HEAD...{upstream}"],
        check=False,
        capture_output=True,
        cwd=str(plugin_dir),
    )
    if code != 0:
        message = stderr or counts or "failed to compare with upstream"
        if dry_run:
            print_warning(message)
            return True
        print_error(f"{plugin_dir.name}: {message}")
        return False

    ahead, behind = (int(part) for part in counts.split())
    if ahead == 0 and behind == 0:
        return True

    message = f"branch differs from {upstream}: ahead {ahead}, behind {behind}"
    if dry_run:
        print_warning(f"{message}; real release would stop here.")
        return True

    print_error(f"{plugin_dir.name}: {message}")
    print_info("Push or pull the branch before creating a release tag.")
    return False


def check_tag_exists(tag: str) -> bool:
    """Check if tag exists locally"""
    code, _, _ = run_cmd(["git", "tag", "-l", tag], check=False, capture_output=True)
    return code == 0


def delete_local_tag(tag: str, plugin_dir: Path, dry_run: bool = False) -> bool:
    """Delete local tag in plugin directory"""
    if dry_run:
        print_info(f"[DRY-RUN] Would delete local tag: {tag}")
        return True
    
    code, _, _ = run_cmd(["git", "tag", "-d", tag], check=False, cwd=str(plugin_dir))
    if code == 0:
        print_success(f"Deleted local tag: {tag}")
        return True
    else:
        print_warning(f"Local tag does not exist or deletion failed: {tag}")
        return False


def create_local_tag(tag: str, plugin_dir: Path, dry_run: bool = False) -> bool:
    """Create local tag in plugin directory"""
    if dry_run:
        print_info(f"[DRY-RUN] Would create local tag: {tag}")
        return True
    
    code, _, _ = run_cmd(["git", "tag", tag], check=False, cwd=str(plugin_dir))
    if code == 0:
        print_success(f"Created local tag: {tag}")
        return True
    else:
        print_error(f"Failed to create local tag: {tag}")
        return False


def push_commit(plugin_dir: Path, dry_run: bool = False) -> bool:
    """Push commits to remote from plugin directory"""
    if dry_run:
        print_info("[DRY-RUN] Would push commits to remote")
        return True

    code, _, _ = run_cmd(["git", "push", "origin", "HEAD"], check=False, cwd=str(plugin_dir))
    if code == 0:
        print_success("Pushed commits to remote")
        return True
    else:
        print_error("Failed to push commits")
        return False


def push_tag(tag: str, plugin_dir: Path, dry_run: bool = False) -> bool:
    """Push tag to remote from plugin directory"""
    if dry_run:
        print_info(f"[DRY-RUN] Would push tag to remote: {tag}")
        return True
    
    code, _, _ = run_cmd(["git", "push", "origin", tag], check=False, cwd=str(plugin_dir))
    if code == 0:
        print_success(f"Pushed tag to remote: {tag}")
        return True
    else:
        print_error(f"Failed to push tag: {tag}")
        return False


def delete_remote_tag(tag: str, plugin_dir: Path, dry_run: bool = False) -> bool:
    """Delete remote tag from plugin directory"""
    if dry_run:
        print_info(f"[DRY-RUN] Would delete remote tag: {tag}")
        return True
    
    code, _, _ = run_cmd(["git", "push", "origin", "--delete", tag], check=False, cwd=str(plugin_dir))
    if code == 0:
        print_success(f"Deleted remote tag: {tag}")
        return True
    else:
        print_warning(f"Remote tag does not exist or deletion failed: {tag}")
        return False


class GitHubAPI:
    """GitHub API client"""
    
    def __init__(self, token: str, owner: str, repo: str):
        self.token = token
        self.owner = owner
        self.repo = repo
        self.base_url = "https://api.github.com"
        # Support both classic tokens (ghp_...) and fine-grained tokens (github_pat_...)
        # Classic tokens use "token", fine-grained tokens use "Bearer"
        if token.startswith("github_pat_"):
            auth_header = f"Bearer {token}"
        else:
            auth_header = f"token {token}"
        
        self.headers = {
            "Authorization": auth_header,
            "Accept": "application/vnd.github.v3+json",
        }
    
    def _request(self, method: str, endpoint: str, **kwargs):
        """Send HTTP request"""
        url = f"{self.base_url}/repos/{self.owner}/{self.repo}/{endpoint}"
        if requests is not None:
            return requests.request(method, url, headers=self.headers, **kwargs)

        data = None
        headers = dict(self.headers)
        if "json" in kwargs:
            data = json.dumps(kwargs["json"]).encode("utf-8")
            headers["Content-Type"] = "application/json"

        request = urllib.request.Request(url, data=data, headers=headers, method=method)
        try:
            with urllib.request.urlopen(request, timeout=kwargs.get("timeout", 30)) as response:
                body = response.read().decode("utf-8")
                return HttpResponse(response.status, body)
        except urllib.error.HTTPError as e:
            body = e.read().decode("utf-8", errors="replace")
            return HttpResponse(e.code, body)
    
    def get_release_by_tag(self, tag: str) -> Optional[dict]:
        """Get release by tag"""
        response = self._request("GET", f"releases/tags/{tag}")
        if response.status_code == 200:
            return response.json()
        elif response.status_code == 404:
            return None
        else:
            response.raise_for_status()
            return None
    
    def delete_release(self, release_id: int, dry_run: bool = False) -> bool:
        """Delete release"""
        if dry_run:
            print_info(f"[DRY-RUN] Would delete GitHub release (ID: {release_id})")
            return True
        
        response = self._request("DELETE", f"releases/{release_id}")
        if response.status_code == 204:
            print_success(f"Deleted GitHub release (ID: {release_id})")
            return True
        else:
            print_error(f"Failed to delete GitHub release: {response.status_code} - {response.text}")
            return False
    
    def create_release(self, tag: str, name: str, body: str = "", dry_run: bool = False) -> bool:
        """Create release"""
        if dry_run:
            print_info(f"[DRY-RUN] Would create GitHub release: {tag}")
            print_info(f"  Name: {name}")
            print_info(f"  Body: {body[:100]}..." if len(body) > 100 else f"  Body: {body}")
            return True
        
        data = {
            "tag_name": tag,
            "name": name,
            "body": body or f"Release {tag}",
            "draft": False,
            "prerelease": False,
        }
        
        response = self._request("POST", "releases", json=data)
        if response.status_code == 201:
            print_success(f"Created GitHub release: {tag}")
            return True
        else:
            error_msg = response.text
            if response.status_code == 403:
                print_error(f"Failed to create GitHub release: 403 Forbidden")
                print_error(f"  Repository: {self.owner}/{self.repo}")
                print_error(f"  Error: {error_msg}")
                print_warning("  Possible causes:")
                print_warning("    1. Token does not have 'repo' permission")
                print_warning("    2. Token does not have access to this repository")
                print_warning("    3. Repository is private and token lacks private repo access")
                print_warning("  Solution: Create a new token with 'repo' scope at:")
                print_warning("    https://github.com/settings/tokens")
            else:
                print_error(f"Failed to create GitHub release: {response.status_code} - {error_msg}")
            return False


def process_plugin(plugin_name: str, plugin_repo: str, version: str,
                   plugin_dir: Path, github_api: Optional[GitHubAPI], dry_run: bool = False,
                   confirm_all: Optional[List[bool]] = None, lynx_version: Optional[str] = None,
                   skip_lynx_check: bool = False) -> bool:
    """Process a single plugin: tag and create release in its own repository"""
    # Tag is just the version (e.g., v1.5.0), not plugin-name/version
    tag = version
    
    print_info(f"\nProcessing plugin: {plugin_name}")
    print_info(f"Repository: {plugin_repo}")
    print_info(f"Plugin directory: {plugin_dir}")
    print_info(f"Tag: {tag}")
    
    # Check if plugin directory exists and is a git repository
    if not plugin_dir.exists():
        print_error(f"Plugin directory does not exist: {plugin_dir}")
        return False
    
    if not (plugin_dir / ".git").exists():
        print_error(f"Plugin directory is not a git repository: {plugin_dir}")
        return False
    
    if not github_api:
        print_warning("GitHub API not available - will skip release operations")
        print_warning("   (Only tag operations will be performed)")

    if not ensure_clean_worktree(plugin_dir, dry_run=dry_run):
        return False
    if not ensure_upstream_synced(plugin_dir, dry_run=dry_run):
        return False

    if skip_lynx_check:
        print_warning("Skipping Lynx SDK dependency check")
    else:
        expected_lynx_version = lynx_version or version
        print_info(f"Step 0: Checking Lynx SDK dependency ({expected_lynx_version})...")
        if not check_lynx_dependency(plugin_dir, expected_lynx_version):
            return False
    
    # Sync pluginVersion in Go files (like release_main syncs banner version)
    print_info("Step 1: Syncing pluginVersion in Go files...")
    version_updated, updated_files = sync_plugin_version(plugin_dir, version, dry_run=dry_run)
    if version_updated and updated_files and not dry_run:
        # Show changed files and diff, then ask for confirmation before commit/push
        print_info(f"Updated {len(updated_files)} file(s):")
        for f in updated_files:
            print(f"  - {f.relative_to(plugin_dir)}")
        # Show git diff for review
        code, diff_out, _ = run_cmd(
            ["git", "diff", "--no-color"] + [str(f.relative_to(plugin_dir)) for f in updated_files],
            check=False,
            capture_output=True,
            cwd=str(plugin_dir),
        )
        if code == 0 and diff_out:
            print(f"\n{Colors.OKCYAN}--- git diff ---{Colors.ENDC}")
            print(diff_out)
            print(f"{Colors.OKCYAN}--- end diff ---{Colors.ENDC}\n")
        # Skip prompt if user already confirmed "yall" for all plugins
        if confirm_all and confirm_all[0]:
            response = "y"
            print_info("Auto-confirmed (yall)")
        else:
            print(f"{Colors.WARNING}Confirm to commit and push these changes? (y/N/yall): {Colors.ENDC}", end="")
            try:
                response = input().strip().lower()
            except KeyboardInterrupt:
                print("\nCancelled")
                # Revert changes
                for f in updated_files:
                    run_cmd(["git", "checkout", "--", str(f.relative_to(plugin_dir))], check=False, cwd=str(plugin_dir))
                return False
            if response == "yall":
                if confirm_all is not None:
                    confirm_all[0] = True
                response = "y"
                print_info("Will auto-confirm remaining plugins")
        if response != "y":
            print_warning("Skipped commit/push. Reverting file changes...")
            for f in updated_files:
                run_cmd(["git", "checkout", "--", str(f.relative_to(plugin_dir))], check=False, cwd=str(plugin_dir))
            return False
        # Git add only the modified files (like release_main adds only banner.txt)
        for f in updated_files:
            rel_path = f.relative_to(plugin_dir)
            run_cmd(["git", "add", str(rel_path)], check=True, cwd=str(plugin_dir))
        run_cmd(
            ["git", "commit", "-m", f"chore: bump pluginVersion to {version}"],
            check=True,
            cwd=str(plugin_dir),
        )
        print_success("Committed pluginVersion update")
        if not push_commit(plugin_dir, dry_run=dry_run):
            return False
    
    # Check and delete existing release
    if github_api:
        print_info("Step 2: Checking for existing GitHub release...")
        release = github_api.get_release_by_tag(tag)
        if release:
            print_warning(f"Found existing GitHub release: {tag}")
            if not github_api.delete_release(release["id"], dry_run=dry_run):
                return False
        else:
            print_info("No existing release found")
    else:
        print_info("Step 2: Skipped (no GitHub API)")
    
    # Delete remote tag (if exists)
    print_info("Step 3: Deleting remote tag (if exists)...")
    delete_remote_tag(tag, plugin_dir, dry_run=dry_run)
    
    # Delete local tag (if exists)
    print_info("Step 4: Deleting local tag (if exists)...")
    delete_local_tag(tag, plugin_dir, dry_run=dry_run)
    
    # Create local tag
    print_info("Step 5: Creating local tag...")
    if not create_local_tag(tag, plugin_dir, dry_run=dry_run):
        return False
    
    # Push tag to remote
    print_info("Step 6: Pushing tag to remote...")
    if not push_tag(tag, plugin_dir, dry_run=dry_run):
        return False
    
    # Create GitHub release
    if github_api:
        print_info("Step 7: Creating GitHub release...")
        release_name = version  # Only version number, no plugin name prefix
        release_body = f"Release {version} for {plugin_name}"
        if not github_api.create_release(tag, release_name, release_body, dry_run=dry_run):
            return False
    else:
        print_warning("Step 7: Skipped (no GitHub API - provide token to create releases)")
    
    print_success(f"Plugin {plugin_name} processed successfully")
    return True


def main():
    parser = argparse.ArgumentParser(
        description="Tag and release lynx plugins on GitHub (all plugins or a single plugin)",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Basic usage (requires GITHUB_TOKEN environment variable)
  python3 release_plugins.py v1.0.0
  
  # Specify GitHub token
  python3 release_plugins.py v1.0.0 --token ghp_xxxxx
  
  # Release a single plugin
  python3 release_plugins.py v1.0.0 --plugin lynx-redis
  
  # Use custom configuration file
  python3 release_plugins.py v1.0.0 --config my-plugins.json
  
  # Release plugins against a specific Lynx SDK version
  python3 release_plugins.py v1.0.0 --lynx-version v1.6.1

  # Dry-run mode (preview only)
  python3 release_plugins.py v1.0.0 --dry-run
        """
    )
    
    parser.add_argument("version", help="Version number, e.g.: v1.0.0")
    parser.add_argument("--token", help="GitHub token (or set GITHUB_TOKEN environment variable)")
    parser.add_argument("--config", default="plugins.json", help="Path to plugins configuration file (default: plugins.json)")
    parser.add_argument("--plugin", help="Process only a specific plugin by name (e.g., lynx-redis)")
    parser.add_argument("--lynx-version", help="Expected github.com/go-lynx/lynx SDK version in each plugin go.mod (default: release version)")
    parser.add_argument("--skip-lynx-check", action="store_true", help="Skip Lynx SDK dependency validation")
    parser.add_argument("--dry-run", action="store_true", help="Dry-run mode, do not execute actual operations")
    
    args = parser.parse_args()
    
    # Validate version format
    version = normalize_version(args.version)
    if args.version != version:
        print_warning(f"Version should start with 'v', using: {version}")

    lynx_version = normalize_version(args.lynx_version) if args.lynx_version else version
    
    # Get GitHub token
    token = args.token or os.getenv("GITHUB_TOKEN")
    if not token and not args.dry_run:
        print_warning("GitHub token not provided, will skip GitHub release creation")
        print_info("You can provide it via --token argument or GITHUB_TOKEN environment variable")
    
    if args.dry_run:
        print_warning("DRY-RUN mode: No actual operations will be performed")
    if not args.skip_lynx_check:
        print_info(f"Expected Lynx SDK dependency: github.com/go-lynx/lynx {lynx_version}")
    
    # 脚本在 lynx/script/ 下，plugins.json 在 lynx/ 下
    lynx_root = Path(__file__).resolve().parent.parent
    root_dir = lynx_root
    config_path = root_dir / args.config
    plugins_config = load_plugins_config(config_path)
    
    # Filter to single plugin if specified
    if args.plugin:
        plugin_name = args.plugin
        matching_plugins = [p for p in plugins_config if p['name'] == plugin_name]
        if not matching_plugins:
            print_error(f"Plugin '{plugin_name}' not found in configuration")
            print_info("Available plugins:")
            for plugin in plugins_config:
                print(f"  - {plugin['name']}")
            sys.exit(1)
        plugins_config = matching_plugins
        print_info(f"\nFiltered to single plugin: {plugin_name}")
    
    print_info(f"\nLoaded {len(plugins_config)} plugin(s) from configuration:")
    for plugin in plugins_config:
        status = "✅" if plugin.get('enabled', True) else "❌"
        print(f"  {status} {plugin['name']} -> {plugin['repo']}")
    
    # Confirmation
    if not args.dry_run:
        plugin_text = "plugin" if len(plugins_config) == 1 else "plugins"
        print(f"\n{Colors.WARNING}About to create tag {version} for {len(plugins_config)} {plugin_text} and release on GitHub")
        print(f"{Colors.WARNING}Press Enter to continue, or Ctrl+C to cancel...{Colors.ENDC}")
        try:
            input()
        except KeyboardInterrupt:
            print("\nCancelled")
            sys.exit(0)
    
    # Process each plugin (confirm_all: when user types "yall", skip confirmation for rest)
    success_count = 0
    failed_plugins = []
    confirm_all = [False]
    
    for plugin_config in plugins_config:
        plugin_name = plugin_config['name']
        plugin_repo = plugin_config['repo']
        
        # Get plugin directory (parent directory of lynx folder)
        plugin_dir = root_dir.parent / plugin_name
        if not plugin_dir.exists():
            print_error(f"Plugin directory not found: {plugin_dir}")
            failed_plugins.append(plugin_name)
            continue
        
        # Create GitHub API for this plugin's repository
        github_api = None
        if token:
            try:
                owner, repo_name = plugin_repo.split("/", 1)
                github_api = GitHubAPI(token, owner, repo_name)
            except ValueError:
                print_error(f"Invalid repository format for {plugin_name}: {plugin_repo}")
                failed_plugins.append(plugin_name)
                continue
        
        try:
            if process_plugin(
                plugin_name,
                plugin_repo,
                version,
                plugin_dir,
                github_api,
                args.dry_run,
                confirm_all,
                lynx_version,
                args.skip_lynx_check,
            ):
                success_count += 1
            else:
                failed_plugins.append(plugin_name)
        except Exception as e:
            print_error(f"Error processing plugin {plugin_name}: {e}")
            failed_plugins.append(plugin_name)
    
    # Summary
    print(f"\n{Colors.BOLD}{'='*60}{Colors.ENDC}")
    print(f"{Colors.BOLD}Processing complete{Colors.ENDC}")
    print(f"Success: {Colors.OKGREEN}{success_count}/{len(plugins_config)}{Colors.ENDC}")
    if failed_plugins:
        print(f"Failed: {Colors.FAIL}{len(failed_plugins)}{Colors.ENDC}")
        print(f"Failed plugins: {', '.join(failed_plugins)}")
    print(f"{Colors.BOLD}{'='*60}{Colors.ENDC}")


if __name__ == "__main__":
    main()
