package jetbrains

import (
	"bufio"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

// JetbrainsCommand represents the JetBrains configuration command
type JetbrainsCommand struct {
	repoKey        string
	artifactoryURL string
	detectedIDEs   []IDEInstallation
	backupPaths    map[string]string
}

// IDEInstallation represents a detected JetBrains IDE installation
type IDEInstallation struct {
	Name           string
	Version        string
	PropertiesPath string
	ConfigDir      string
}

// JetBrains IDE product codes and names
var jetbrainsIDEs = map[string]string{
	"IntelliJIdea":  "IntelliJ IDEA",
	"PyCharm":       "PyCharm",
	"WebStorm":      "WebStorm",
	"PhpStorm":      "PhpStorm",
	"RubyMine":      "RubyMine",
	"CLion":         "CLion",
	"DataGrip":      "DataGrip",
	"GoLand":        "GoLand",
	"Rider":         "Rider",
	"AndroidStudio": "Android Studio",
	"AppCode":       "AppCode",
	"RustRover":     "RustRover",
	"Aqua":          "Aqua",
}

// NewJetbrainsCommand creates a new JetBrains configuration command
func NewJetbrainsCommand(repoKey, artifactoryURL string) *JetbrainsCommand {
	return &JetbrainsCommand{
		repoKey:        repoKey,
		artifactoryURL: artifactoryURL,
		backupPaths:    make(map[string]string),
	}
}

// Run executes the JetBrains configuration command
func (jc *JetbrainsCommand) Run() error {
	log.Info("Configuring JetBrains IDEs plugin repository...")

	var repoURL string
	if jc.repoKey == "" {
		repoURL = jc.artifactoryURL
	} else {
		if jc.artifactoryURL == "" {
			serverDetails, err := config.GetDefaultServerConf()
			if err != nil {
				return errorutils.CheckError(fmt.Errorf("failed to get default server configuration: %w", err))
			}
			if serverDetails == nil {
				return errorutils.CheckError(fmt.Errorf("no default server configuration found. Please configure JFrog CLI or provide --artifactory-url"))
			}
			jc.artifactoryURL = serverDetails.GetUrl()
		}
		repoURL = jc.buildRepositoryURL()
	}

	if err := jc.validateRepository(repoURL); err != nil {
		return errorutils.CheckError(fmt.Errorf("repository validation failed: %w", err))
	}

	if err := jc.detectJetBrainsIDEs(); err != nil {
		return errorutils.CheckError(fmt.Errorf("failed to detect JetBrains IDEs: %w\n\nManual setup instructions:\n%s", err, jc.getManualSetupInstructions(repoURL)))
	}

	if len(jc.detectedIDEs) == 0 {
		return errorutils.CheckError(fmt.Errorf("no JetBrains IDEs found\n\nManual setup instructions:\n%s", jc.getManualSetupInstructions(repoURL)))
	}

	log.Info(fmt.Sprintf("Found %d JetBrains IDE installation(s):", len(jc.detectedIDEs)))
	for _, ide := range jc.detectedIDEs {
		log.Info(fmt.Sprintf("  %s %s", ide.Name, ide.Version))
	}

	modifiedCount := 0
	for _, ide := range jc.detectedIDEs {
		log.Info(fmt.Sprintf("Configuring %s %s...", ide.Name, ide.Version))

		if err := jc.createBackup(ide); err != nil {
			log.Warn(fmt.Sprintf("Failed to create backup for %s: %v", ide.Name, err))
			continue
		}

		if err := jc.modifyPropertiesFile(ide, repoURL); err != nil {
			log.Error(fmt.Sprintf("Failed to configure %s: %v", ide.Name, err))
			if restoreErr := jc.restoreBackup(ide); restoreErr != nil {
				log.Error(fmt.Sprintf("Failed to restore backup for %s: %v", ide.Name, restoreErr))
			}
			continue
		}

		modifiedCount++
		log.Info(fmt.Sprintf("%s %s configured successfully", ide.Name, ide.Version))
	}

	if modifiedCount == 0 {
		return errorutils.CheckError(fmt.Errorf("failed to configure any JetBrains IDEs\n\nManual setup instructions:\n%s", jc.getManualSetupInstructions(repoURL)))
	}

	log.Info(fmt.Sprintf("Successfully configured %d out of %d JetBrains IDE(s)", modifiedCount, len(jc.detectedIDEs)))
	log.Info("Repository URL:", repoURL)
	log.Info("Please restart your JetBrains IDEs to apply changes")

	return nil
}

// buildRepositoryURL constructs the complete repository URL
func (jc *JetbrainsCommand) buildRepositoryURL() string {
	baseURL := strings.TrimSuffix(jc.artifactoryURL, "/")
	return fmt.Sprintf("%s/artifactory/%s", baseURL, jc.repoKey)
}

// validateRepository checks if the repository is accessible
func (jc *JetbrainsCommand) validateRepository(repoURL string) error {
	log.Info("Validating repository...")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(repoURL)
	if err != nil {
		return fmt.Errorf("failed to connect to repository: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("repository not found (404). Please verify the repository key '%s' exists", jc.repoKey)
	}
	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusUnauthorized {
		return fmt.Errorf("repository returned status %d. Please verify the repository is accessible", resp.StatusCode)
	}

	log.Info("Repository validation successful")
	return nil
}

// detectJetBrainsIDEs attempts to auto-detect JetBrains IDE installations
func (jc *JetbrainsCommand) detectJetBrainsIDEs() error {
	var configBasePath string

	switch runtime.GOOS {
	case "darwin":
		configBasePath = filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "JetBrains")
	case "windows":
		configBasePath = filepath.Join(os.Getenv("APPDATA"), "JetBrains")
	case "linux":
		configBasePath = filepath.Join(os.Getenv("HOME"), ".config", "JetBrains")
		// Also check legacy location
		if _, err := os.Stat(configBasePath); os.IsNotExist(err) {
			legacyPath := filepath.Join(os.Getenv("HOME"), ".JetBrains")
			if _, err := os.Stat(legacyPath); err == nil {
				configBasePath = legacyPath
			}
		}
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}

	if _, err := os.Stat(configBasePath); os.IsNotExist(err) {
		return fmt.Errorf("JetBrains configuration directory not found at: %s", configBasePath)
	}

	// Scan for IDE configurations
	entries, err := os.ReadDir(configBasePath)
	if err != nil {
		return fmt.Errorf("failed to read JetBrains configuration directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Parse IDE name and version from directory name
		dirName := entry.Name()
		ide := jc.parseIDEFromDirName(dirName)
		if ide == nil {
			continue
		}

		// Set the full config directory path
		ide.ConfigDir = filepath.Join(configBasePath, dirName)

		// Check for idea.properties file
		propertiesPath := filepath.Join(ide.ConfigDir, "idea.properties")
		if _, err := os.Stat(propertiesPath); err == nil {
			ide.PropertiesPath = propertiesPath
		} else {
			// Create idea.properties if it doesn't exist
			ide.PropertiesPath = propertiesPath
		}

		jc.detectedIDEs = append(jc.detectedIDEs, *ide)
	}

	// Sort IDEs by name for consistent output
	sort.Slice(jc.detectedIDEs, func(i, j int) bool {
		return jc.detectedIDEs[i].Name < jc.detectedIDEs[j].Name
	})

	return nil
}

// parseIDEFromDirName extracts IDE name and version from configuration directory name
func (jc *JetbrainsCommand) parseIDEFromDirName(dirName string) *IDEInstallation {
	for productCode, displayName := range jetbrainsIDEs {
		if strings.HasPrefix(dirName, productCode) {
			// Extract version from directory name (e.g., "IntelliJIdea2023.3" -> "2023.3")
			version := strings.TrimPrefix(dirName, productCode)
			if version == "" {
				version = "Unknown"
			}

			return &IDEInstallation{
				Name:    displayName,
				Version: version,
			}
		}
	}
	return nil
}

// createBackup creates a backup of the original idea.properties file
func (jc *JetbrainsCommand) createBackup(ide IDEInstallation) error {
	backupPath := ide.PropertiesPath + ".backup." + time.Now().Format("20060102-150405")

	// If properties file doesn't exist, create an empty backup
	if _, err := os.Stat(ide.PropertiesPath); os.IsNotExist(err) {
		// Create empty file for backup record
		if err := os.WriteFile(backupPath, []byte("# Empty properties file backup\n"), 0644); err != nil {
			return fmt.Errorf("failed to create backup marker: %w", err)
		}
		jc.backupPaths[ide.PropertiesPath] = backupPath
		return nil
	}

	// Read existing properties file
	data, err := os.ReadFile(ide.PropertiesPath)
	if err != nil {
		return fmt.Errorf("failed to read properties file: %w", err)
	}

	// Write backup
	if err := os.WriteFile(backupPath, data, 0644); err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	jc.backupPaths[ide.PropertiesPath] = backupPath
	log.Info(fmt.Sprintf("	Backup created at: %s", backupPath))
	return nil
}

// restoreBackup restores the backup in case of failure
func (jc *JetbrainsCommand) restoreBackup(ide IDEInstallation) error {
	backupPath, exists := jc.backupPaths[ide.PropertiesPath]
	if !exists {
		return fmt.Errorf("no backup path available for %s", ide.PropertiesPath)
	}

	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("failed to read backup: %w", err)
	}

	// Check if this was an empty file backup
	if strings.Contains(string(data), "# Empty properties file backup") {
		// Remove the properties file if it was created
		if err := os.Remove(ide.PropertiesPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove created properties file: %w", err)
		}
		return nil
	}

	if err := os.WriteFile(ide.PropertiesPath, data, 0644); err != nil {
		return fmt.Errorf("failed to restore backup: %w", err)
	}

	log.Info(fmt.Sprintf("	Backup restored for %s", ide.Name))
	return nil
}

// modifyPropertiesFile modifies or creates the idea.properties file
func (jc *JetbrainsCommand) modifyPropertiesFile(ide IDEInstallation, repoURL string) error {
	var lines []string
	var pluginsHostSet bool

	// Read existing properties if file exists
	if _, err := os.Stat(ide.PropertiesPath); err == nil {
		data, err := os.ReadFile(ide.PropertiesPath)
		if err != nil {
			return fmt.Errorf("failed to read properties file: %w", err)
		}

		scanner := bufio.NewScanner(strings.NewReader(string(data)))
		for scanner.Scan() {
			line := scanner.Text()
			trimmedLine := strings.TrimSpace(line)

			// Check if this line sets idea.plugins.host
			if strings.HasPrefix(trimmedLine, "idea.plugins.host=") {
				// Replace with our repository URL
				lines = append(lines, fmt.Sprintf("idea.plugins.host=%s", repoURL))
				pluginsHostSet = true
				log.Info(fmt.Sprintf("  Updated existing idea.plugins.host property"))
			} else {
				lines = append(lines, line)
			}
		}

		if err := scanner.Err(); err != nil {
			return fmt.Errorf("failed to scan properties file: %w", err)
		}
	}

	// Add idea.plugins.host if not found
	if !pluginsHostSet {
		if len(lines) > 0 {
			lines = append(lines, "") // Add empty line for readability
		}
		lines = append(lines, "# JFrog Artifactory plugins repository")
		lines = append(lines, fmt.Sprintf("idea.plugins.host=%s", repoURL))
		log.Info(fmt.Sprintf("  Added idea.plugins.host property"))
	}

	// Ensure config directory exists
	if err := os.MkdirAll(filepath.Dir(ide.PropertiesPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write modified properties file
	content := strings.Join(lines, "\n") + "\n"
	if err := os.WriteFile(ide.PropertiesPath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write properties file: %w", err)
	}

	return nil
}

// getManualSetupInstructions returns manual setup instructions
func (jc *JetbrainsCommand) getManualSetupInstructions(repoURL string) string {
	var configPath string
	switch runtime.GOOS {
	case "darwin":
		configPath = "~/Library/Application Support/JetBrains/[IDE][VERSION]/idea.properties"
	case "windows":
		configPath = "%APPDATA%\\JetBrains\\[IDE][VERSION]\\idea.properties"
	case "linux":
		configPath = "~/.config/JetBrains/[IDE][VERSION]/idea.properties"
	default:
		configPath = "[JetBrains config directory]/[IDE][VERSION]/idea.properties"
	}

	instructions := fmt.Sprintf(`
Manual JetBrains IDE Setup Instructions:
=======================================

1. Close all JetBrains IDEs

2. Locate your IDE configuration directory:
   %s

   Examples:
   • IntelliJ IDEA: IntelliJIdea2023.3/idea.properties
   • PyCharm: PyCharm2023.3/idea.properties
   • WebStorm: WebStorm2023.3/idea.properties

3. Open or create the idea.properties file in a text editor

4. Add or modify the following line:
   idea.plugins.host=%s

5. Save the file and restart your IDE

Repository URL: %s

Supported IDEs: IntelliJ IDEA, PyCharm, WebStorm, PhpStorm, RubyMine, CLion, DataGrip, GoLand, Rider, Android Studio, AppCode, RustRover, Aqua
`, configPath, repoURL, repoURL)

	return instructions
}
