#!/usr/bin/env python3
"""
Automatically tag and release all lynx plugins on GitHub

Usage:
    python3 release_plugins.py <version> [--token <github_token>] [--repo <repo>] [--dry-run]

Examples:
    python3 release_plugins.py v1.0.0
    python3 release_plugins.py v1.0.0 --token ghp_xxxxx
    python3 release_plugins.py v1.0.0 --repo go-lynx/lynx --dry-run
"""

import os
import sys
import subprocess
import argparse
import re
from pathlib import Path
from typing import List, Optional, Tuple
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


# Try to import requests after defining print functions
try:
    import requests
except ImportError:
    print_error("requests library is required")
    print_info("Installation options:")
    print_info("1. Using virtual environment (recommended):")
    print_info("   python3 -m venv venv")
    print_info("   source venv/bin/activate  # On Windows: venv\\Scripts\\activate")
    print_info("   pip3 install -r requirements.txt")
    print_info("")
    print_info("2. Using --user flag:")
    print_info("   pip3 install --user requests")
    print_info("")
    print_info("3. Using --break-system-packages (not recommended):")
    print_info("   pip3 install --break-system-packages requests")
    sys.exit(1)


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
    
    def _request(self, method: str, endpoint: str, **kwargs) -> requests.Response:
        """Send HTTP request"""
        url = f"{self.base_url}/repos/{self.owner}/{self.repo}/{endpoint}"
        return requests.request(method, url, headers=self.headers, **kwargs)
    
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
                   plugin_dir: Path, github_api: Optional[GitHubAPI], dry_run: bool = False) -> bool:
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
        print_warning("⚠️  GitHub API not available - will skip release operations")
        print_warning("   (Only tag operations will be performed)")
    
    # 1. Check and delete existing release
    if github_api:
        print_info("Step 1: Checking for existing GitHub release...")
        release = github_api.get_release_by_tag(tag)
        if release:
            print_warning(f"Found existing GitHub release: {tag}")
            if not github_api.delete_release(release["id"], dry_run=dry_run):
                return False
        else:
            print_info("No existing release found")
    else:
        print_info("Step 1: Skipped (no GitHub API)")
    
    # 2. Delete remote tag (if exists)
    print_info("Step 2: Deleting remote tag (if exists)...")
    delete_remote_tag(tag, plugin_dir, dry_run=dry_run)
    
    # 3. Delete local tag (if exists)
    print_info("Step 3: Deleting local tag (if exists)...")
    delete_local_tag(tag, plugin_dir, dry_run=dry_run)
    
    # 4. Create local tag
    print_info("Step 4: Creating local tag...")
    if not create_local_tag(tag, plugin_dir, dry_run=dry_run):
        return False
    
    # 5. Push tag to remote
    print_info("Step 5: Pushing tag to remote...")
    if not push_tag(tag, plugin_dir, dry_run=dry_run):
        return False
    
    # 6. Create GitHub release
    if github_api:
        print_info("Step 6: Creating GitHub release...")
        release_name = version  # Only version number, no plugin name prefix
        release_body = f"Release {version} for {plugin_name}"
        if not github_api.create_release(tag, release_name, release_body, dry_run=dry_run):
            return False
    else:
        print_warning("Step 6: Skipped (no GitHub API - provide token to create releases)")
    
    print_success(f"✅ Plugin {plugin_name} processed successfully")
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
  
  # Dry-run mode (preview only)
  python3 release_plugins.py v1.0.0 --dry-run
        """
    )
    
    parser.add_argument("version", help="Version number, e.g.: v1.0.0")
    parser.add_argument("--token", help="GitHub token (or set GITHUB_TOKEN environment variable)")
    parser.add_argument("--config", default="plugins.json", help="Path to plugins configuration file (default: plugins.json)")
    parser.add_argument("--plugin", help="Process only a specific plugin by name (e.g., lynx-redis)")
    parser.add_argument("--dry-run", action="store_true", help="Dry-run mode, do not execute actual operations")
    
    args = parser.parse_args()
    
    # Validate version format
    version = args.version
    if not version.startswith("v"):
        print_warning(f"Version should start with 'v', e.g.: v{version}")
        version = f"v{version}"
    
    # Get GitHub token
    token = args.token or os.getenv("GITHUB_TOKEN")
    if not token and not args.dry_run:
        print_warning("GitHub token not provided, will skip GitHub release creation")
        print_info("You can provide it via --token argument or GITHUB_TOKEN environment variable")
    
    if args.dry_run:
        print_warning("⚠️  DRY-RUN mode: No actual operations will be performed")
    
    # Load plugins configuration
    root_dir = Path(__file__).parent
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
    
    # Process each plugin
    success_count = 0
    failed_plugins = []
    
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
            if process_plugin(plugin_name, plugin_repo, version, plugin_dir, github_api, args.dry_run):
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
