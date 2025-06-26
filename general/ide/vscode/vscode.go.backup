package vscode

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// VscodeCommand represents the VSCode configuration command
type VscodeCommand struct {
	repoKey        string
	artifactoryURL string
	productPath    string
	backupPath     string
}

// ProductJSON represents the structure of VSCode's product.json file
type ProductJSON struct {
	ExtensionsGallery *ExtensionsGallery `json:"extensionsGallery,omitempty"`
	// Other fields can be added as needed
}

// ExtensionsGallery represents the extensions gallery configuration
type ExtensionsGallery struct {
	NlsBaseURL          string `json:"nlsBaseUrl,omitempty"`
	ServiceURL          string `json:"serviceUrl,omitempty"`
	CacheURL            string `json:"cacheUrl,omitempty"`
	ItemURL             string `json:"itemUrl,omitempty"`
	PublisherURL        string `json:"publisherUrl,omitempty"`
	ResourceURLTemplate string `json:"resourceUrlTemplate,omitempty"`
	ControlURL          string `json:"controlUrl,omitempty"`
}

// NewVscodeCommand creates a new VSCode configuration command
func NewVscodeCommand(repoKey, artifactoryURL, productPath string) *VscodeCommand {
	return &VscodeCommand{
		repoKey:        repoKey,
		artifactoryURL: artifactoryURL,
		productPath:    productPath,
	}
}

// Run executes the VSCode configuration command
func (vc *VscodeCommand) Run() error {
	log.Info("Configuring VSCode to use JFrog Artifactory extensions repository...")

	// Get the Artifactory URL if not provided
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

	// Build the complete repository URL
	repoURL := vc.buildRepositoryURL()

	// Validate repository connection
	if err := vc.validateRepository(repoURL); err != nil {
		return errorutils.CheckError(fmt.Errorf("repository validation failed: %w", err))
	}

	// Auto-detect VSCode installation if path not provided
	if vc.productPath == "" {
		detectedPath, err := vc.detectVSCodeInstallation()
		if err != nil {
			return errorutils.CheckError(fmt.Errorf("failed to auto-detect VSCode installation: %w\n\nManual setup instructions:\n%s", err, vc.getManualSetupInstructions(repoURL)))
		}
		vc.productPath = detectedPath
		log.Info("Auto-detected VSCode installation at:", vc.productPath)
	}

	// Check for System Integrity Protection on macOS
	if runtime.GOOS == "darwin" {
		if err := vc.checkSIPStatus(); err != nil {
			log.Warn("System Integrity Protection (SIP) may prevent modification of VSCode files.")
			log.Warn("If the setup fails, please see the manual setup instructions.")
		}
	}

	// Create backup
	if err := vc.createBackup(); err != nil {
		return errorutils.CheckError(fmt.Errorf("failed to create backup: %w", err))
	}

	// Modify product.json
	if err := vc.modifyProductJSON(repoURL); err != nil {
		// Attempt to restore backup on failure
		if restoreErr := vc.restoreBackup(); restoreErr != nil {
			log.Error("Failed to restore backup after error:", restoreErr)
		}
		return errorutils.CheckError(fmt.Errorf("failed to modify product.json: %w\n\nBackup restored. Manual setup instructions:\n%s", err, vc.getManualSetupInstructions(repoURL)))
	}

	log.Info("✅ VSCode successfully configured to use JFrog Artifactory extensions repository!")
	log.Info("Repository URL:", repoURL)
	log.Info("Backup created at:", vc.backupPath)
	log.Info("\nPlease restart VSCode for the changes to take effect.")

	return nil
}

// buildRepositoryURL constructs the complete repository URL
func (vc *VscodeCommand) buildRepositoryURL() string {
	baseURL := strings.TrimSuffix(vc.artifactoryURL, "/")
	return fmt.Sprintf("%s/artifactory/api/vscode/%s", baseURL, vc.repoKey)
}

// validateRepository checks if the repository is accessible
func (vc *VscodeCommand) validateRepository(repoURL string) error {
	log.Info("Validating repository connection...")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(repoURL)
	if err != nil {
		return fmt.Errorf("failed to connect to repository: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return fmt.Errorf("repository not found (404). Please verify the repository key '%s' exists and has anonymous read access", vc.repoKey)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("repository returned status %d. Please verify the repository is accessible", resp.StatusCode)
	}

	log.Info("✅ Repository validation successful")
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

// checkSIPStatus checks if System Integrity Protection might interfere
func (vc *VscodeCommand) checkSIPStatus() error {
	if !strings.HasPrefix(vc.productPath, "/Applications/") {
		return nil // Not in a protected location
	}

	log.Warn("VSCode is installed in /Applications/ which may be protected by System Integrity Protection (SIP)")
	log.Warn("If modification fails, you may need to:")
	log.Warn("1. Disable SIP temporarily")
	log.Warn("2. Use the manual setup instructions")
	log.Warn("3. Install VSCode in a user-writable location")

	return nil
}

// createBackup creates a backup of the original product.json
func (vc *VscodeCommand) createBackup() error {
	log.Info("Creating backup of original product.json...")

	vc.backupPath = vc.productPath + ".backup." + time.Now().Format("20060102-150405")

	data, err := ioutil.ReadFile(vc.productPath)
	if err != nil {
		return fmt.Errorf("failed to read original product.json: %w", err)
	}

	if err := ioutil.WriteFile(vc.backupPath, data, 0644); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	log.Info("✅ Backup created at:", vc.backupPath)
	return nil
}

// restoreBackup restores the backup in case of failure
func (vc *VscodeCommand) restoreBackup() error {
	if vc.backupPath == "" {
		return fmt.Errorf("no backup path available")
	}

	log.Info("Restoring backup...")
	data, err := ioutil.ReadFile(vc.backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	if err := ioutil.WriteFile(vc.productPath, data, 0644); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	log.Info("✅ Backup restored")
	return nil
}

// modifyProductJSON modifies the product.json file to use Artifactory
func (vc *VscodeCommand) modifyProductJSON(repoURL string) error {
	log.Info("Modifying product.json...")

	// Read current product.json
	data, err := ioutil.ReadFile(vc.productPath)
	if err != nil {
		return fmt.Errorf("failed to read product.json: %w", err)
	}

	// Parse JSON
	var product ProductJSON
	if err := json.Unmarshal(data, &product); err != nil {
		return fmt.Errorf("failed to parse product.json: %w", err)
	}

	// Ensure extensionsGallery exists
	if product.ExtensionsGallery == nil {
		product.ExtensionsGallery = &ExtensionsGallery{}
	}

	// Store original serviceUrl for logging
	originalURL := product.ExtensionsGallery.ServiceURL

	// Update serviceUrl to point to Artifactory
	product.ExtensionsGallery.ServiceURL = repoURL

	// Marshal back to JSON with indentation
	modifiedData, err := json.MarshalIndent(product, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal modified JSON: %w", err)
	}

	// Write modified product.json
	if err := ioutil.WriteFile(vc.productPath, modifiedData, 0644); err != nil {
		return fmt.Errorf("failed to write modified product.json: %w", err)
	}

	log.Info("✅ product.json modified successfully")
	if originalURL != "" {
		log.Info("Original serviceUrl:", originalURL)
	}
	log.Info("New serviceUrl:", repoURL)

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

3. Open the product.json file in a text editor

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
