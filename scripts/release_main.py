#!/usr/bin/env python3
"""
Release script for main lynx project
Usage: python3 release_main.py <version> [--token <github_token>] [--repo owner/repo] [--dry-run]
Example: python3 release_main.py v1.5.1
"""

import os
import sys
import subprocess
import argparse
import re
import json
import urllib.error
import urllib.request

try:
    import requests
except ImportError:
    requests = None


class HTTPResponse:
    """Small response adapter shared by requests and urllib."""

    def __init__(self, status_code: int, text: str, headers: dict):
        self.status_code = status_code
        self.text = text
        self.headers = headers

    def json(self):
        return json.loads(self.text) if self.text else {}

    def raise_for_status(self):
        if self.status_code >= 400:
            raise RuntimeError(f"HTTP {self.status_code}: {self.text}")


def http_request(method: str, url: str, headers: dict, **kwargs) -> HTTPResponse:
    """Send HTTP request using requests when available, otherwise urllib."""
    if requests is not None:
        return requests.request(method, url, headers=headers, **kwargs)

    body = None
    request_headers = dict(headers)
    if "json" in kwargs:
        body = json.dumps(kwargs["json"]).encode("utf-8")
        request_headers["Content-Type"] = "application/json"
    elif "data" in kwargs:
        data = kwargs["data"]
        body = data if isinstance(data, bytes) else str(data).encode("utf-8")

    req = urllib.request.Request(url, data=body, headers=request_headers, method=method)
    try:
        with urllib.request.urlopen(req, timeout=kwargs.get("timeout", 30)) as response:
            text = response.read().decode("utf-8")
            return HTTPResponse(response.status, text, dict(response.headers))
    except urllib.error.HTTPError as exc:
        text = exc.read().decode("utf-8")
        return HTTPResponse(exc.code, text, dict(exc.headers))
    except urllib.error.URLError as exc:
        return HTTPResponse(0, str(exc), {})


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


def parse_repo_slug(value: str) -> tuple:
    """Parse owner/repo from a repository slug or GitHub URL."""
    if not value:
        return None, None
    value = value.strip()
    if re.match(r"^[^/\s]+/[^/\s]+$", value):
        owner, repo = value.split("/", 1)
        return owner, repo.removesuffix(".git")
    return parse_github_repo(value)


def sync_banner_version(version: str, dry_run: bool = False) -> bool:
    """
    Update the version in internal/banner/banner.txt to match the release version.
    The last line of banner.txt contains the version (e.g. "... v1.5.0").
    Returns True if file was updated (or would be in dry-run), False if no change needed.
    """
    # 脚本在 lynx/script/ 下，banner 在 lynx/internal/banner/
    _lynx_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    banner_path = os.path.join(_lynx_root, "internal", "banner", "banner.txt")
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
    
    def __init__(self, token: str, owner: str, repo: str, base_url: str = "https://api.github.com"):
        self.token = token.strip()
        self.owner = owner
        self.repo = repo
        self.base_url = base_url.rstrip("/")
        if self.token.startswith("github_pat_"):
            auth_header = f"Bearer {self.token}"
        else:
            auth_header = f"token {self.token}"
        
        self.headers = {
            "Authorization": auth_header,
            "Accept": "application/vnd.github.v3+json",
            "X-GitHub-Api-Version": "2022-11-28",
        }
    
    def _request(self, method: str, endpoint: str, **kwargs) -> requests.Response:
        """Send HTTP request"""
        endpoint = endpoint.strip("/")
        repo_url = f"{self.base_url}/repos/{self.owner}/{self.repo}"
        url = f"{repo_url}/{endpoint}" if endpoint else repo_url
        return http_request(method, url, headers=self.headers, **kwargs)

    def _raw_request(self, method: str, endpoint: str, **kwargs) -> requests.Response:
        """Send HTTP request to an API endpoint outside /repos/{owner}/{repo}."""
        endpoint = endpoint.lstrip("/")
        url = f"{self.base_url}/{endpoint}"
        return http_request(method, url, headers=self.headers, **kwargs)

    def _public_request(self, method: str, endpoint: str, **kwargs) -> requests.Response:
        """Send an unauthenticated request for public visibility checks."""
        endpoint = endpoint.lstrip("/")
        url = f"{self.base_url}/{endpoint}"
        headers = {
            "Accept": "application/vnd.github.v3+json",
            "X-GitHub-Api-Version": "2022-11-28",
        }
        return http_request(method, url, headers=headers, **kwargs)

    def describe_target(self) -> str:
        return f"{self.owner}/{self.repo} via {self.base_url}"

    def check_token_and_repo_access(self) -> bool:
        """Validate that the token can see the target repository before release creation."""
        user_response = self._raw_request("GET", "user")
        if user_response.status_code == 401:
            print_error("GitHub token is invalid or expired")
            print_error(f"  API: {self.base_url}")
            return False
        if user_response.status_code == 200:
            user_info = user_response.json()
            login = user_info.get("login") or "<unknown>"
            scopes = user_response.headers.get("X-OAuth-Scopes")
            print_info(f"GitHub token authenticated as: {login}")
            if scopes:
                print_info(f"GitHub token scopes: {scopes}")
        if user_response.status_code not in (200, 403):
            print_warning(f"Could not verify GitHub token identity: {user_response.status_code} - {user_response.text}")

        repo_response = self._request("GET", "")
        if repo_response.status_code == 200:
            repo_info = repo_response.json()
            permissions = repo_info.get("permissions", {})
            if permissions and not permissions.get("push") and not permissions.get("admin"):
                print_warning("Token can see the repository but may not be able to create releases")
                print_warning(f"  Repository permissions reported by GitHub: {permissions}")
            return True

        print_error(f"GitHub repository access check failed: {repo_response.status_code} - {repo_response.text}")
        print_error(f"  Repository: {self.owner}/{self.repo}")
        print_error(f"  API: {self.base_url}")
        if repo_response.status_code == 404:
            print_warning("  GitHub returns 404 when the repository does not exist OR the token cannot see it.")
            public_response = self._public_request("GET", f"repos/{self.owner}/{self.repo}")
            if public_response.status_code == 200:
                print_warning("  Unauthenticated check can see this repository, so the token is the problem.")
            elif public_response.status_code == 404:
                print_warning("  Unauthenticated check also returns 404; confirm the repository slug or private-repo access.")
            print_warning("  Check:")
            print_warning("    1. --repo / GITHUB_REPOSITORY / git origin resolves to the correct owner/repo")
            print_warning("    2. Fine-grained token is granted access to this exact repository")
            print_warning("    3. Token has Contents: read/write permission for releases")
            print_warning("    4. For org repositories, SSO is authorized for the token if required")
        elif repo_response.status_code == 403:
            print_warning("  Token can reach GitHub but lacks permission or is blocked by org policy/rate limit.")
        return False
    
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
                print_error("Failed to create GitHub release: 403 Forbidden")
                print_error(f"  Target: {self.describe_target()}")
                print_error(f"  Error: {error_msg}")
                print_warning("  Possible causes:")
                print_warning("    1. Token does not have 'repo' permission")
                print_warning("    2. Token does not have access to this repository")
                print_warning("  Solution: Create a new token with 'repo' scope at:")
                print_warning("    https://github.com/settings/tokens")
            elif response.status_code == 404:
                print_error("Failed to create GitHub release: 404 Not Found")
                print_error(f"  Target: {self.describe_target()}")
                print_error(f"  Error: {error_msg}")
                print_warning("  GitHub commonly returns 404 when token access is missing or the repo slug/API host is wrong.")
                print_warning("  Try:")
                print_warning(f"    python3 scripts/release_main.py {tag} --repo {self.owner}/{self.repo} --token <token>")
                print_warning("  For fine-grained tokens, grant this repository and Contents: read/write.")
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

  # Override repository/API target
  python3 release_main.py v1.5.1 --repo go-lynx/lynx --api-url https://api.github.com
  
  # Dry-run mode (preview only)
  python3 release_main.py v1.5.1 --dry-run
        """
    )
    
    parser.add_argument("version", help="Version number, e.g.: v1.5.1")
    parser.add_argument("--token", help="GitHub token (or set GITHUB_TOKEN environment variable)")
    parser.add_argument("--repo", help="GitHub repository as owner/repo (or set GITHUB_REPOSITORY)")
    parser.add_argument("--api-url", default=os.getenv("GITHUB_API_URL", "https://api.github.com"), help="GitHub API URL")
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
    
    repo_override = args.repo or os.getenv("GITHUB_REPOSITORY")
    if repo_override:
        owner, repo = parse_repo_slug(repo_override)
        if not owner or not repo:
            print_error(f"Failed to parse GitHub repository from --repo/GITHUB_REPOSITORY: {repo_override}")
            sys.exit(1)
    else:
        owner, repo = parse_github_repo(remote_url)
    if not owner or not repo:
        print_error(f"Failed to parse GitHub repository from: {remote_url}")
        sys.exit(1)
    
    print_info(f"Repository: {owner}/{repo}")
    print_info(f"GitHub API: {args.api_url}")
    
    # Get GitHub token
    token = (args.token or os.getenv("GITHUB_TOKEN") or os.getenv("GH_TOKEN") or "").strip()
    github_api = None
    if token:
        github_api = GitHubAPI(token, owner, repo, base_url=args.api_url)
        if not args.dry_run and not github_api.check_token_and_repo_access():
            sys.exit(1)
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
    code, _, _ = run_cmd(["git", "rev-parse", "--git-dir"], check=False, capture_output=True)
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
