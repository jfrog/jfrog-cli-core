package vscode

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// VscodeCommand represents the VSCode configuration command
type VscodeCommand struct {
	repoKey        string
	artifactoryURL string
	productPath    string
	backupPath     string
}

// VscodeInstallCommand represents the VSCode extension installation command
type VscodeInstallCommand struct {
	repoKey        string
	artifactoryURL string
	publisher      string
	extensionName  string
	version        string
}

// Note: We use sed for direct modification, so we don't need JSON struct definitions

// NewVscodeCommand creates a new VSCode configuration command
func NewVscodeCommand(repoKey, artifactoryURL, productPath string) *VscodeCommand {
	return &VscodeCommand{
		repoKey:        repoKey,
		artifactoryURL: artifactoryURL,
		productPath:    productPath,
	}
}

// NewVscodeInstallCommand creates a new VSCode extension installation command
func NewVscodeInstallCommand(repoKey, artifactoryURL, publisher, extensionName, version string) *VscodeInstallCommand {
	return &VscodeInstallCommand{
		repoKey:        repoKey,
		artifactoryURL: artifactoryURL,
		publisher:      publisher,
		extensionName:  extensionName,
		version:        version,
	}
}

// Run executes the VSCode configuration command
func (vc *VscodeCommand) Run() error {
	log.Info("Configuring VSCode extensions repository...")

	var repoURL string
	if vc.repoKey == "" {
		repoURL = vc.artifactoryURL
	} else {
		if vc.artifactoryURL == "" {
			serverDetails, err := config.GetDefaultServerConf()
			if err != nil {
				return errorutils.CheckError(fmt.Errorf("failed to get default server configuration: %w", err))
			}
			if serverDetails == nil {
				return errorutils.CheckError(fmt.Errorf("no default server configuration found. Please configure JFrog CLI or provide --artifactory-url"))
			}
			vc.artifactoryURL = serverDetails.GetUrl()
		}
		repoURL = vc.buildRepositoryURL()
	}

	if err := vc.validateRepository(repoURL); err != nil {
		return errorutils.CheckError(fmt.Errorf("repository validation failed: %w", err))
	}

	if vc.productPath == "" {
		detectedPath, err := vc.detectVSCodeInstallation()
		if err != nil {
			return errorutils.CheckError(fmt.Errorf("failed to auto-detect VSCode installation: %w\n\nManual setup instructions:\n%s", err, vc.getManualSetupInstructions(repoURL)))
		}
		vc.productPath = detectedPath
		log.Info("Detected VSCode at:", vc.productPath)
	}

	if err := vc.modifyProductJson(repoURL); err != nil {
		if restoreErr := vc.restoreBackup(); restoreErr != nil {
			log.Error("Failed to restore backup:", restoreErr)
		}
		return errorutils.CheckError(fmt.Errorf("failed to modify product.json: %w\n\nManual setup instructions:\n%s", err, vc.getManualSetupInstructions(repoURL)))
	}

	log.Info("VSCode configuration updated successfully")
	log.Info("Repository URL:", repoURL)
	log.Info("Please restart VSCode to apply changes")

	return nil
}

// buildRepositoryURL constructs the complete repository URL
func (vc *VscodeCommand) buildRepositoryURL() string {
	baseURL := strings.TrimSuffix(vc.artifactoryURL, "/")
	return fmt.Sprintf("%s/artifactory/api/vscodeextensions/%s/_apis/public/gallery", baseURL, vc.repoKey)
}

// checkWritePermissions checks if we have write permissions to the product.json file
func (vc *VscodeCommand) checkWritePermissions() error {
	// Check if file exists and we can read it
	info, err := os.Stat(vc.productPath)
	if err != nil {
		return fmt.Errorf("failed to access product.json: %w", err)
	}

	if runtime.GOOS != "windows" {
		if os.Getuid() == 0 {
			return nil
		}
	}

	file, err := os.OpenFile(vc.productPath, os.O_WRONLY|os.O_APPEND, info.Mode())
	if err != nil {
		if os.IsPermission(err) {
			return vc.handlePermissionError()
		}
		return fmt.Errorf("failed to check write permissions: %w", err)
	}
	file.Close()
	return nil
}

// handlePermissionError provides appropriate guidance based on the operating system
func (vc *VscodeCommand) handlePermissionError() error {
	if runtime.GOOS == "darwin" && strings.HasPrefix(vc.productPath, "/Applications/") {
		// Get current user info for better error message
		userInfo := "the current user"
		if user := os.Getenv("USER"); user != "" {
			userInfo = user
		}

		return fmt.Errorf(`insufficient permissions to modify VSCode configuration.

VSCode is installed in /Applications/ which requires elevated privileges to modify.

To fix this, run the command with sudo:

    sudo jf vscode config '%s'

This is the same approach that works with manual editing:
    sudo nano "%s"

Note: This does NOT require disabling System Integrity Protection (SIP).
The file is owned by admin and %s needs elevated privileges to write to it.

Alternative: Install VSCode in a user-writable location like ~/Applications/`, vc.artifactoryURL, vc.productPath, userInfo)
	}

	return fmt.Errorf(`insufficient permissions to modify VSCode configuration.

To fix this, try running the command with elevated privileges:
    sudo jf vscode config '%s'

Or use the manual setup instructions provided in the error output.`, vc.artifactoryURL)
}

// validateRepository checks if the repository is accessible
func (vc *VscodeCommand) validateRepository(repoURL string) error {
	log.Info("Validating repository...")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(repoURL)
	if err != nil {
		log.Warn("Could not validate repository connection:", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		log.Warn("Repository not found (404). Please verify the repository exists")
		return nil
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		log.Warn("Repository requires authentication")
		return nil
	}
	if resp.StatusCode >= 400 {
		log.Warn("Repository returned status", resp.StatusCode)
		return nil
	}

	log.Info("Repository validation successful")
	return nil
}

// detectVSCodeInstallation attempts to auto-detect VSCode installation
func (vc *VscodeCommand) detectVSCodeInstallation() (string, error) {
	var possiblePaths []string

	switch runtime.GOOS {
	case "darwin":
		possiblePaths = []string{
			"/Applications/Visual Studio Code.app/Contents/Resources/app/product.json",
			"/Applications/Visual Studio Code - Insiders.app/Contents/Resources/app/product.json",
			// Add user-installed locations
			filepath.Join(os.Getenv("HOME"), "Applications", "Visual Studio Code.app", "Contents", "Resources", "app", "product.json"),
		}
	case "windows":
		possiblePaths = []string{
			filepath.Join(os.Getenv("LOCALAPPDATA"), "Programs", "Microsoft VS Code", "resources", "app", "product.json"),
			filepath.Join(os.Getenv("PROGRAMFILES"), "Microsoft VS Code", "resources", "app", "product.json"),
			filepath.Join(os.Getenv("PROGRAMFILES(X86)"), "Microsoft VS Code", "resources", "app", "product.json"),
		}
	case "linux":
		possiblePaths = []string{
			"/usr/share/code/resources/app/product.json",
			"/opt/visual-studio-code/resources/app/product.json",
			"/snap/code/current/usr/share/code/resources/app/product.json",
			filepath.Join(os.Getenv("HOME"), ".vscode-server", "bin", "*", "product.json"),
		}
	default:
		return "", fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
		// Handle glob patterns for Linux
		if strings.Contains(path, "*") {
			matches, _ := filepath.Glob(path)
			for _, match := range matches {
				if _, err := os.Stat(match); err == nil {
					return match, nil
				}
			}
		}
	}

	return "", fmt.Errorf("VSCode installation not found in standard locations")
}

// createBackup creates a backup of the original product.json
func (vc *VscodeCommand) createBackup() error {
	backupDir, err := coreutils.GetJfrogBackupDir()
	if err != nil {
		return fmt.Errorf("failed to get JFrog backup directory: %w", err)
	}

	ideBackupDir := filepath.Join(backupDir, "ide", "vscode")
	err = fileutils.CreateDirIfNotExist(ideBackupDir)
	if err != nil {
		return fmt.Errorf("failed to create IDE backup directory: %w", err)
	}

	timestamp := time.Now().Format("20060102-150405")
	backupFileName := "product.json.backup." + timestamp
	vc.backupPath = filepath.Join(ideBackupDir, backupFileName)

	data, err := os.ReadFile(vc.productPath)
	if err != nil {
		return fmt.Errorf("failed to read original product.json: %w", err)
	}

	if err := os.WriteFile(vc.backupPath, data, 0644); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	log.Info("Backup created at:", vc.backupPath)
	return nil
}

// restoreBackup restores the backup in case of failure
func (vc *VscodeCommand) restoreBackup() error {
	if vc.backupPath == "" {
		return fmt.Errorf("no backup path available")
	}

	data, err := os.ReadFile(vc.backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	if err := os.WriteFile(vc.productPath, data, 0644); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}
	return nil
}

// modifyProductJson modifies the VSCode product.json file
func (vc *VscodeCommand) modifyProductJson(repoURL string) error {
	// Check write permissions first
	if err := vc.checkWritePermissions(); err != nil {
		return err
	}

	// Create backup first
	if err := vc.createBackup(); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	var err error
	if runtime.GOOS == "windows" {
		err = vc.modifyWithPowerShell(repoURL)
	} else {
		err = vc.modifyWithSed(repoURL)
	}

	if err != nil {
		if restoreErr := vc.restoreBackup(); restoreErr != nil {
			log.Error("Failed to restore backup:", restoreErr)
		}
		return err
	}

	return nil
}

// modifyWithSed uses sed for direct in-place modification - simpler and more efficient
func (vc *VscodeCommand) modifyWithSed(repoURL string) error {
	// Escape special characters in the URL for sed
	escapedURL := strings.ReplaceAll(repoURL, "/", "\\/")
	escapedURL = strings.ReplaceAll(escapedURL, "&", "\\&")

	// Create sed command to find and replace serviceUrl
	// This regex looks for "serviceUrl": "any-content" and replaces with new URL
	sedPattern := fmt.Sprintf(`s#"serviceUrl": "[^"]*"#"serviceUrl": "%s"#`, escapedURL)

	var cmd *exec.Cmd
	if runtime.GOOS == "darwin" {
		// macOS sed needs empty string for -i flag
		cmd = exec.Command("sudo", "sed", "-i", "", sedPattern, vc.productPath)
	} else {
		// Linux sed
		cmd = exec.Command("sudo", "sed", "-i", sedPattern, vc.productPath)
	}

	cmd.Stdin = os.Stdin // Allow interactive sudo prompt if needed

	if output, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(output), "operation not permitted") {
			return fmt.Errorf("SIP protection prevents modification")
		}
		return fmt.Errorf("failed to modify product.json with sed: %w\nOutput: %s", err, string(output))
	}

	if err := vc.verifyModification(repoURL); err != nil {
		return fmt.Errorf("modification verification failed: %w", err)
	}
	return nil
}

// modifyWithPowerShell uses PowerShell for Windows file modification
func (vc *VscodeCommand) modifyWithPowerShell(repoURL string) error {
	// Escape quotes for PowerShell
	escapedURL := strings.ReplaceAll(repoURL, `"`, `\"`)

	// PowerShell command to replace serviceUrl in the JSON file
	// Uses PowerShell's -replace operator which works similar to sed
	psCommand := fmt.Sprintf(`(Get-Content "%s") -replace '"serviceUrl": "[^"]*"', '"serviceUrl": "%s"' | Set-Content "%s"`,
		vc.productPath, escapedURL, vc.productPath)

	// Run PowerShell command
	// Note: This requires the JF CLI to be run as Administrator on Windows
	cmd := exec.Command("powershell", "-Command", psCommand)
	cmd.Stdin = os.Stdin

	if output, err := cmd.CombinedOutput(); err != nil {
		if strings.Contains(string(output), "Access") && strings.Contains(string(output), "denied") {
			return fmt.Errorf("access denied - please run JF CLI as Administrator on Windows")
		}
		return fmt.Errorf("failed to modify product.json with PowerShell: %w\nOutput: %s", err, string(output))
	}

	if err := vc.verifyModification(repoURL); err != nil {
		return fmt.Errorf("modification verification failed: %w", err)
	}
	return nil
}

// verifyModification checks that the serviceUrl was actually changed
func (vc *VscodeCommand) verifyModification(expectedURL string) error {
	data, err := os.ReadFile(vc.productPath)
	if err != nil {
		return fmt.Errorf("failed to read file for verification: %w", err)
	}

	if !strings.Contains(string(data), expectedURL) {
		return fmt.Errorf("expected URL %s not found in modified file", expectedURL)
	}

	return nil
}

// getManualSetupInstructions returns manual setup instructions
func (vc *VscodeCommand) getManualSetupInstructions(repoURL string) string {
	instructions := fmt.Sprintf(`
Manual VSCode Setup Instructions:
=================================

1. Close VSCode completely

2. Locate your VSCode installation directory:
   • macOS: /Applications/Visual Studio Code.app/Contents/Resources/app/
   • Windows: %%LOCALAPPDATA%%\Programs\Microsoft VS Code\resources\app\
   • Linux: /usr/share/code/resources/app/

3. Open the product.json file in a text editor with appropriate permissions:
   • macOS: sudo nano "/Applications/Visual Studio Code.app/Contents/Resources/app/product.json"
   • Windows: Run editor as Administrator
   • Linux: sudo nano /usr/share/code/resources/app/product.json

4. Find the "extensionsGallery" section and modify the "serviceUrl":
   {
     "extensionsGallery": {
       "serviceUrl": "%s",
       ...
     }
   }

5. Save the file and restart VSCode

Repository URL: %s
`, repoURL, repoURL)

	return instructions
}

// Run executes the VSCode extension installation command
func (vic *VscodeInstallCommand) Run() error {
	log.Info("Installing VSCode extension from JFrog Artifactory...")

	var repoURL string
	if vic.artifactoryURL == "" {
		serverDetails, err := config.GetDefaultServerConf()
		if err != nil {
			return errorutils.CheckError(fmt.Errorf("failed to get default server configuration: %w", err))
		}
		if serverDetails == nil {
			return errorutils.CheckError(fmt.Errorf("no default server configuration found. Please configure JFrog CLI or provide --artifactory-url"))
		}
		vic.artifactoryURL = serverDetails.GetUrl()
	}
	repoURL = vic.buildExtensionURL()

	if err := vic.validateExtensionRepository(repoURL); err != nil {
		return errorutils.CheckError(fmt.Errorf("repository validation failed: %w", err))
	}

	if err := vic.downloadAndInstallExtension(repoURL); err != nil {
		return errorutils.CheckError(fmt.Errorf("failed to install extension: %w", err))
	}

	log.Info("Extension installed successfully")
	log.Info("Publisher:", vic.publisher)
	log.Info("Extension:", vic.extensionName)
	if vic.version != "" {
		log.Info("Version:", vic.version)
	}
	log.Info("Please restart VSCode to use the extension")

	return nil
}

// buildExtensionURL constructs the extension download URL
func (vic *VscodeInstallCommand) buildExtensionURL() string {
	baseURL := strings.TrimSuffix(vic.artifactoryURL, "/")
	if vic.version != "" {
		return fmt.Sprintf("%s/artifactory/api/vscodeextensions/%s/_apis/public/gallery/publishers/%s/extensions/%s/%s/vspackage",
			baseURL, vic.repoKey, vic.publisher, vic.extensionName, vic.version)
	}
	return fmt.Sprintf("%s/artifactory/api/vscodeextensions/%s/_apis/public/gallery/publishers/%s/extensions/%s/latest/vspackage",
		baseURL, vic.repoKey, vic.publisher, vic.extensionName)
}

// validateExtensionRepository checks if the extension repository is accessible
func (vic *VscodeInstallCommand) validateExtensionRepository(repoURL string) error {
	log.Info("Validating extension repository...")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Head(repoURL)
	if err != nil {
		log.Warn("Could not validate extension repository:", err)
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("extension not found. Please verify publisher '%s' and extension '%s' exist in repository '%s'", vic.publisher, vic.extensionName, vic.repoKey)
	}
	if resp.StatusCode >= 400 {
		log.Warn("Extension repository returned status", resp.StatusCode)
		return nil
	}

	log.Info("Extension repository validation successful")
	return nil
}

// downloadAndInstallExtension downloads and installs the VSCode extension
func (vic *VscodeInstallCommand) downloadAndInstallExtension(repoURL string) error {
	log.Info("Downloading extension...")

	// Download the extension package
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(repoURL)
	if err != nil {
		return fmt.Errorf("failed to download extension: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download extension: HTTP %d", resp.StatusCode)
	}

	// Create temporary file for the extension package
	tempDir := os.TempDir()
	extensionFileName := vic.publisher + "." + vic.extensionName + ".vsix"
	if vic.version != "" {
		extensionFileName = vic.publisher + "." + vic.extensionName + "-" + vic.version + ".vsix"
	}
	tempFile := filepath.Join(tempDir, extensionFileName)

	// Write the downloaded content to temp file
	out, err := os.Create(tempFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tempFile) // Clean up temp file

	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		return fmt.Errorf("failed to save extension package: %w", err)
	}

	log.Info("Installing extension using VSCode CLI...")

	// Install extension using VSCode's CLI
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("code.cmd", "--install-extension", tempFile, "--force")
	} else {
		cmd = exec.Command("code", "--install-extension", tempFile, "--force")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install extension via VSCode CLI: %w\nOutput: %s", err, string(output))
	}

	log.Info("Extension package installed via VSCode CLI")
	return nil
}
