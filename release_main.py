#!/usr/bin/env python3
"""
Release script for main lynx project
Usage: python3 release_main.py <version> [--token <github_token>] [--dry-run]
Example: python3 release_main.py v1.5.1
"""

import os
import sys
import subprocess
import argparse
import re

try:
    import requests
except ImportError:
    print("❌ requests library is required")
    print("Install it with: pip3 install requests")
    sys.exit(1)


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


def run_cmd(cmd: list, check: bool = True, capture_output: bool = False, cwd: str = None) -> tuple:
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


def get_git_remote_url() -> str:
    """Get git remote origin URL"""
    code, output, _ = run_cmd(["git", "remote", "get-url", "origin"], check=False, capture_output=True)
    if code != 0:
        return ""
    return output.strip()


def parse_github_repo(url: str) -> tuple:
    """Parse GitHub owner/repo from git URL"""
    patterns = [
        r'github\.com[:/]([^/]+)/([^/]+?)(?:\.git)?/?$',
    ]
    
    for pattern in patterns:
        match = re.search(pattern, url)
        if match:
            owner = match.group(1)
            repo = match.group(2)
            return owner, repo
    
    return None, None


def sync_banner_version(version: str, dry_run: bool = False) -> bool:
    """
    Update the version in internal/banner/banner.txt to match the release version.
    The last line of banner.txt contains the version (e.g. "... v1.5.0").
    Returns True if file was updated (or would be in dry-run), False if no change needed.
    """
    script_dir = os.path.dirname(os.path.abspath(__file__))
    banner_path = os.path.join(script_dir, "internal", "banner", "banner.txt")
    if not os.path.isfile(banner_path):
        print_warning(f"Banner file not found: {banner_path}, skipping version sync")
        return False

    with open(banner_path, "r", encoding="utf-8") as f:
        content = f.read()

    # Replace version on the last line (e.g. "... v1.5.0" -> "... v1.5.1")
    # Match trailing version pattern: optional space + v + semver + optional whitespace
    new_content = re.sub(r"\s+v[\d.]+\s*$", f" {version}\n", content, count=1)
    if new_content == content:
        print_info("Banner version already matches, no change needed")
        return False

    if dry_run:
        print_info(f"[DRY-RUN] Would update banner version to {version} in internal/banner/banner.txt")
        return True

    with open(banner_path, "w", encoding="utf-8") as f:
        f.write(new_content)
    print_success(f"Updated banner version to {version} in internal/banner/banner.txt")
    return True


class GitHubAPI:
    """GitHub API client"""
    
    def __init__(self, token: str, owner: str, repo: str):
        self.token = token
        self.owner = owner
        self.repo = repo
        self.base_url = "https://api.github.com"
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
    
    def get_release_by_tag(self, tag: str):
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
                print_warning("  Solution: Create a new token with 'repo' scope at:")
                print_warning("    https://github.com/settings/tokens")
            else:
                print_error(f"Failed to create GitHub release: {response.status_code} - {error_msg}")
            return False


def main():
    parser = argparse.ArgumentParser(
        description="Release main lynx project",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Basic usage (requires GITHUB_TOKEN environment variable)
  python3 release_main.py v1.5.1
  
  # Specify GitHub token
  python3 release_main.py v1.5.1 --token ghp_xxxxx
  
  # Dry-run mode (preview only)
  python3 release_main.py v1.5.1 --dry-run
        """
    )
    
    parser.add_argument("version", help="Version number, e.g.: v1.5.1")
    parser.add_argument("--token", help="GitHub token (or set GITHUB_TOKEN environment variable)")
    parser.add_argument("--dry-run", action="store_true", help="Dry-run mode, do not execute actual operations")
    
    args = parser.parse_args()
    
    # Validate version format
    version = args.version
    if not version.startswith("v"):
        print_warning(f"Version should start with 'v', e.g.: v{version}")
        version = f"v{version}"
    
    if args.dry_run:
        print_warning("⚠️  DRY-RUN mode: No actual operations will be performed")

    # Ensure we run from repository root (so git add/commit and paths are correct)
    code, git_root, _ = run_cmd(
        ["git", "rev-parse", "--show-toplevel"],
        check=False,
        capture_output=True,
    )
    if code == 0 and git_root:
        os.chdir(git_root)
        print_info(f"Working directory: {git_root}")

    # Get git repository info
    print_info("Checking git repository...")
    remote_url = get_git_remote_url()
    if not remote_url:
        print_error("Failed to get git remote URL. Are you in a git repository?")
        sys.exit(1)
    
    owner, repo = parse_github_repo(remote_url)
    if not owner or not repo:
        print_error(f"Failed to parse GitHub repository from: {remote_url}")
        sys.exit(1)
    
    print_info(f"Repository: {owner}/{repo}")
    
    # Get GitHub token
    token = args.token or os.getenv("GITHUB_TOKEN")
    github_api = None
    if token:
        github_api = GitHubAPI(token, owner, repo)
    elif not args.dry_run:
        print_warning("⚠️  GitHub token not provided!")
        print_warning("   Without token, only tag will be created, GitHub release will be skipped")
        print_info("   To create GitHub release, provide token via:")
        print_info("     - --token argument: python3 release_main.py v1.5.1 --token ghp_xxxxx")
        print_info("     - GITHUB_TOKEN environment variable: export GITHUB_TOKEN=ghp_xxxxx")
        print_info("")
        response = input("Continue without creating GitHub release? (y/N): ")
        if response.lower() != 'y':
            print("Release cancelled")
            sys.exit(0)
    
    # Check if we're in a git repository
    code, _, _ = run_cmd(["git", "rev-parse", "--git-dir"], check=False)
    if code != 0:
        print_error("Not in a git repository")
        sys.exit(1)
    
    # Check for uncommitted changes
    code, _, _ = run_cmd(["git", "diff-index", "--quiet", "HEAD", "--"], check=False)
    if code != 0:
        print_warning("You have uncommitted changes")
        if not args.dry_run:
            response = input("Continue anyway? (y/N): ")
            if response.lower() != 'y':
                print("Release cancelled")
                sys.exit(0)

    # Sync banner version so that startup banner matches the release version
    print_info("Syncing banner version...")
    banner_updated = sync_banner_version(version, dry_run=args.dry_run)
    if banner_updated and not args.dry_run:
        run_cmd(["git", "add", "internal/banner/banner.txt"], check=True)
        run_cmd(
            ["git", "commit", "-m", f"chore: bump banner version to {version}"],
            check=True,
        )
        print_success("Committed banner version update (tag will point to this commit)")

    # Check if tag exists locally
    code, _, _ = run_cmd(["git", "rev-parse", version], check=False, capture_output=True)
    if code == 0:
        print_warning(f"Tag {version} already exists locally")
        if not args.dry_run:
            response = input("Delete and recreate it? (y/N): ")
            if response.lower() == 'y':
                print_info(f"Deleting local tag {version}...")
                run_cmd(["git", "tag", "-d", version], check=False)
            else:
                print("Release cancelled")
                sys.exit(0)
        else:
            print_info(f"[DRY-RUN] Would delete local tag {version}")
    
    # Check if tag exists on remote
    code, output, _ = run_cmd(["git", "ls-remote", "--tags", "origin", version], check=False, capture_output=True)
    if code == 0 and output:
        print_warning(f"Tag {version} already exists on remote")
        if not args.dry_run:
            response = input("Delete and recreate it? (y/N): ")
            if response.lower() == 'y':
                print_info(f"Deleting remote tag {version}...")
                run_cmd(["git", "push", "origin", "--delete", version], check=False)
            else:
                print("Release cancelled")
                sys.exit(0)
        else:
            print_info(f"[DRY-RUN] Would delete remote tag {version}")
    
    # Check and delete existing GitHub release
    if github_api:
        print_info("Checking for existing GitHub release...")
        release = github_api.get_release_by_tag(version)
        if release:
            print_warning(f"Found existing GitHub release: {version}")
            if not github_api.delete_release(release["id"], dry_run=args.dry_run):
                sys.exit(1)
        else:
            print_info("No existing release found")
    
    # Create local tag
    print_info(f"Creating local tag {version}...")
    if args.dry_run:
        print_info(f"[DRY-RUN] Would create local tag: {version}")
    else:
        code, _, _ = run_cmd(["git", "tag", "-a", version, "-m", f"Release {version}"], check=False)
        if code != 0:
            print_error(f"Failed to create local tag: {version}")
            sys.exit(1)
        print_success(f"Created local tag: {version}")
    
    # Push tag to remote
    print_info(f"Pushing tag {version} to remote...")
    if args.dry_run:
        print_info(f"[DRY-RUN] Would push tag to remote: {version}")
    else:
        code, _, _ = run_cmd(["git", "push", "origin", version], check=False)
        if code != 0:
            print_error(f"Failed to push tag: {version}")
            sys.exit(1)
        print_success(f"Pushed tag to remote: {version}")
    
    # Create GitHub release
    if github_api:
        print_info("Creating GitHub release...")
        release_name = version
        release_body = f"Release {version} of lynx framework"
        if not github_api.create_release(version, release_name, release_body, dry_run=args.dry_run):
            sys.exit(1)
        print_success("GitHub release created successfully!")
    else:
        print_warning("⚠️  Skipped GitHub release creation (no token provided)")
        print_info("📋 To create GitHub release manually:")
        print_info(f"   1. Visit: https://github.com/{owner}/{repo}/releases/new")
        print_info(f"   2. Select tag: {version}")
        print_info(f"   3. Fill in release notes and publish")
        print_info("")
        print_info("   Or run this script again with --token to create release automatically")
        print_info("   GitHub Actions may also automatically create a release when it detects the tag")
    
    print_success(f"✅ Release {version} completed successfully!")
    print_info("📋 Next steps:")
    print_info("   - If you want to release plugins, run: python3 release_plugins.py " + version)


if __name__ == "__main__":
    main()
