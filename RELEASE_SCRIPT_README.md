# Plugin Release Script

This script automatically tags and releases all `lynx-*` plugins on GitHub using a configuration file.

## Features

- ✅ Reads plugin list from configuration file (`plugins.json`)
- ✅ Each plugin is an independent GitHub repository
- ✅ Creates tags in each plugin's own repository (format: `version`, e.g., `v1.0.0`)
- ✅ Automatically deletes existing tags and releases (if they exist)
- ✅ Creates releases on GitHub for each plugin's repository
- ✅ Supports dry-run mode for previewing operations
- ✅ Batch processing: one command to release all plugins

## Installation

### Option 1: Using Virtual Environment (Recommended)

```bash
# Create virtual environment
python3 -m venv venv

# Activate virtual environment
# On macOS/Linux:
source venv/bin/activate
# On Windows:
# venv\Scripts\activate

# Install dependencies
pip3 install -r requirements.txt
```

### Option 2: Using --user Flag

```bash
pip3 install --user requests
```

This installs the package to your user directory, which doesn't require system-wide permissions.

### Option 3: Using --break-system-packages (Not Recommended)

```bash
pip3 install --break-system-packages requests
```

**Warning**: This may break your system Python installation. Only use if you understand the risks.

**Note**: On macOS, the system Python environment is externally managed. Using a virtual environment (Option 1) is the recommended approach.

## Usage

### Basic Usage

```bash
# Requires GITHUB_TOKEN environment variable
export GITHUB_TOKEN=ghp_xxxxx
python3 release_plugins.py v1.0.0
```

### Specify GitHub Token

```bash
python3 release_plugins.py v1.0.0 --token ghp_xxxxx
```

### Specify GitHub Repository

```bash
python3 release_plugins.py v1.0.0 --repo owner/repo --token ghp_xxxx
```

### Dry-Run Mode (Preview)

```bash
python3 release_plugins.py v1.0.0 --dry-run
```

## Workflow

The script performs the following operations for **each plugin** in the configuration:

1. **Load configuration**: Read plugin list from `plugins.json`
2. **For each enabled plugin**:
   - **Navigate to plugin directory**: Each plugin must be in its own git repository
   - **Check for existing GitHub release**: If exists, delete it first (in the plugin's repository)
   - **Delete remote tag**: If exists, delete it first (from plugin's repository)
   - **Delete local tag**: If exists, delete it first (in plugin's directory)
   - **Create local tag**: Format is `version` (e.g., `v1.0.0`) in the plugin's repository
   - **Push tag to remote**: Push tag to the plugin's GitHub repository
   - **Create GitHub release**: Create corresponding release in the plugin's GitHub repository

All plugins are processed in sequence. The script will show progress for each plugin and provide a summary at the end.

**Important**: Each plugin directory must be a separate git repository with its own remote configured.

## Notes

1. **GitHub Token Permissions**: Requires the following permissions:
   - `repo` permission (for creating and deleting releases in all plugin repositories)
   - Or at least `public_repo` permission (if repositories are public)

2. **Git Repository**: Each plugin directory must be a separate git repository with `origin` remote configured. The script will navigate to each plugin directory and perform git operations there.

3. **Version Format**: Recommended to use semantic versioning, e.g.:
   - `v1.0.0`
   - `v2.1.3`
   - `v1.0.0-beta.1`

4. **Plugin Repositories**: Each plugin can have its own GitHub repository. The script will create releases in each plugin's repository as specified in `plugins.json`.

5. **Batch Processing**: All enabled plugins are processed in one command. You can disable plugins by setting `"enabled": false` in the configuration file.

## Example Output

```
ℹ️  Loaded 26 plugins from configuration:
  ✅ lynx-apollo -> go-lynx/lynx-apollo
  ✅ lynx-dtm -> go-lynx/lynx-dtm
  ✅ lynx-elasticsearch -> go-lynx/lynx-elasticsearch
  ...

Processing plugin: lynx-redis
Repository: go-lynx/lynx-redis
Plugin directory: /path/to/lynx-redis
Tag: v1.0.0
✅ Deleted local tag: v1.0.0
✅ Created local tag: v1.0.0
✅ Pushed tag to remote: v1.0.0
✅ Created GitHub release: v1.0.0
✅ Plugin lynx-redis processed successfully

============================================================
Processing complete
Success: 26/26
============================================================
```

## Troubleshooting

### Issue: Configuration file not found

**Solution**: Make sure `plugins.json` exists in the same directory as the script, or use `--config` to specify a custom path

### Issue: Invalid repository format

**Solution**: Check that each plugin's `repo` field in `plugins.json` is in the format `owner/repo` (e.g., `go-lynx/lynx-redis`)

### Issue: GitHub API authentication failed

**Solution**:
1. Check if the token is valid
2. Confirm the token has sufficient permissions
3. Check if the token has expired

### Issue: Failed to push tag

**Solution**:
1. Check if you have push permissions
2. Confirm network connection is normal
3. Check if git remote is configured

## GitHub Token Setup

### Step 1: Create a GitHub Personal Access Token

1. Go to GitHub: https://github.com/settings/tokens
2. Click **"Generate new token"** → **"Generate new token (classic)"**
3. Give it a name (e.g., "Lynx Plugin Release")
4. Select expiration (recommend: 90 days or custom)

5. **Repository access** (IMPORTANT):
   - Select **"All repositories"** (recommended)
   - OR **"Only select repositories"** and select all your plugin repositories
   
6. **Repository permissions** (scroll down to find this section):
   - Expand **"Repository permissions"** section
   - Under **"Contents"**: Select **"Read and write"**
   - Under **"Metadata"**: Select **"Read-only"** (usually selected by default)
   - Under **"Pull requests"**: Not required, but can be "Read-only" if needed
   - **Most importantly**: Look for **"Releases"** permission and select **"Write"**
     - This is required to create and delete releases!

7. Click **"Generate token"** at the bottom
8. **Copy the token immediately** (you won't be able to see it again!)
   - Token format: `ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx` or `github_pat_...`

**Note**: If you don't see "Releases" permission, make sure you:
- Selected "All repositories" or specific repositories in "Repository access"
- Scrolled down to the "Repository permissions" section (not "Account permissions")

### Step 2: Set the Token

**Option 1: Environment Variable (Recommended)**

```bash
# Set for current session
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx

# Verify it's set
echo $GITHUB_TOKEN

# Run the script
python3 release_plugins.py v1.0.0 --dry-run
```

**Option 2: Add to Shell Profile (Persistent)**

Add to `~/.zshrc` (for zsh) or `~/.bash_profile` (for bash):

```bash
# Add this line to your ~/.zshrc file
export GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

Then reload:
```bash
source ~/.zshrc
```

**Option 3: Command Line Argument**

```bash
python3 release_plugins.py v1.0.0 --token ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

**Option 4: Using GitHub CLI (if installed)**

```bash
# If you have GitHub CLI installed
export GITHUB_TOKEN=$(gh auth token)
python3 release_plugins.py v1.0.0
```

### Security Notes

⚠️ **Important Security Tips:**

- **Never commit the token to code repository**
- **Don't share the token publicly**
- Use environment variables instead of command line arguments when possible
- Set token expiration for security
- Revoke old tokens if compromised

### Verify Token Works

Test if your token is set correctly:

```bash
# Check if token is set
echo $GITHUB_TOKEN

# Test with dry-run
python3 release_plugins.py v1.0.0 --dry-run
```

If the token is valid, you should see GitHub API operations in the output.
