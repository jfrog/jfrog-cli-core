# JFrog CLI IDE Configuration

Automates JFrog Artifactory repository configuration for IDEs.

## Features

- **VSCode**: Configures extension gallery to use JFrog Artifactory
- **JetBrains**: Configures plugin repositories for all JetBrains IDEs  
- **Cross-platform**: Windows, macOS, Linux support
- **Automatic backup**: Creates backups before modifications
- **Auto-detection**: Finds IDE installations automatically

## Commands

### VSCode
```bash
# Configure VSCode extensions repository
jf vscode config --repo=<REPO_KEY> --artifactory-url=<URL>

# Install VSCode extension from repository
jf vscode install --publisher=<PUBLISHER> --extension-name=<EXTENSION_NAME> --repo=<REPO_KEY> [--version=<VERSION>] [--artifactory-url=<URL>]

# Examples
jf vscode config --repo=vscode-extensions --artifactory-url=https://myartifactory.com/
sudo jf vscode config --repo=vscode-extensions --artifactory-url=https://myartifactory.com/  # macOS/Linux

# Install specific extension
jf vscode install --publisher=microsoft --extension-name=vscode-python --repo=vscode-extensions
jf vscode install --publisher=ms-python --extension-name=python --version=2023.12.0 --repo=vscode-extensions
```

### JetBrains
```bash
# Configure JetBrains plugin repository  
jf jetbrains config --repo=<REPO_KEY> --artifactory-url=<URL>

# Examples
jf jetbrains config --repo=jetbrains-plugins --artifactory-url=https://myartifactory.com/
```

## How It Works

### VSCode
**Configuration:**
1. Detects VSCode installation (`product.json` location)
2. Creates backup in `~/.jfrog/backup/ide/vscode/`
3. Modifies `extensionsGallery.serviceUrl` using:
   - **macOS/Linux**: `sed` command
   - **Windows**: PowerShell regex replacement
4. Preserves all other VSCode configuration

**Extension Installation:**
1. Validates extension exists in Artifactory repository
2. Downloads extension package (.vsix) from JFrog Artifactory 
3. Installs extension using VSCode CLI (`code --install-extension`)
4. Supports specific version installation or latest version
5. Cross-platform support (Windows/macOS/Linux)

### JetBrains  
1. Scans for JetBrains IDE configurations
2. Creates backups in `~/.jfrog/backup/ide/jetbrains/`
3. Updates `idea.properties` files with repository URLs
4. Supports all JetBrains IDEs (IntelliJ, PyCharm, WebStorm, etc.)

## Platform Requirements

- **macOS**: Requires `sudo` for system-installed IDEs
- **Windows**: Requires "Run as Administrator" 
- **Linux**: Requires `sudo` for system-installed IDEs

## File Locations

### VSCode
- **macOS**: `/Applications/Visual Studio Code.app/Contents/Resources/app/product.json`
- **Windows**: `%LOCALAPPDATA%\Programs\Microsoft VS Code\resources\app\product.json`
- **Linux**: `/usr/share/code/resources/app/product.json`

### JetBrains
- **macOS**: `~/Library/Application Support/JetBrains/`
- **Windows**: `%APPDATA%\JetBrains\`  
- **Linux**: `~/.config/JetBrains/`

## Backup & Recovery

- Automatic backups created before modifications
- Located in `~/.jfrog/backup/ide/`
- Automatic restore on failure
- Manual restore: copy backup file back to original location 