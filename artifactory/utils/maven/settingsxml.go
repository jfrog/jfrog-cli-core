package maven

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	mavenv1 "github.com/apache/camel-k/v2/pkg/apis/camel/v1"
	"github.com/apache/camel-k/v2/pkg/util/maven"
)

const (
	// ArtifactoryMirrorID is the ID used for the Artifactory mirror.
	ArtifactoryMirrorID = "artifactory-mirror"

	// ArtifactoryDeployProfileID is the ID used for the Artifactory deployment profile.
	ArtifactoryDeployProfileID = "artifactory-deploy"

	// AltDeploymentRepositoryProperty is the Maven property for overriding deployment repository.
	AltDeploymentRepositoryProperty = "altDeploymentRepository"
)

// SettingsXmlManager manages the maven settings file (`settings.xml`).
type SettingsXmlManager struct {
	path     string
	settings maven.Settings
}

// NewSettingsXmlManager creates a new SettingsXmlManager instance.
// It automatically loads the existing settings from the `settings.xml` file if it exists.
func NewSettingsXmlManager() (*SettingsXmlManager, error) {
	// Check HOME environment variable first (for cross-platform compatibility, especially in tests)
	homeDir := os.Getenv("HOME")
	if homeDir == "" {
		// Fall back to OS-specific user home directory
		var err error
		homeDir, err = os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get user home directory: %w", err)
		}
	}
	manager := &SettingsXmlManager{
		path: filepath.Join(homeDir, ".m2", "settings.xml"),
	}

	// Load existing settings from file
	err := manager.loadSettings()
	if err != nil {
		return nil, fmt.Errorf("failed to load settings from %s: %w", manager.path, err)
	}

	return manager, nil
}

// loadSettings reads the settings.xml file and unmarshals it into the Settings struct.
func (sxm *SettingsXmlManager) loadSettings() error {
	file, err := os.ReadFile(sxm.path)
	if err != nil {
		if os.IsNotExist(err) {
			// If file does not exist, initialize with empty settings
			sxm.settings = maven.Settings{
				XMLName: xml.Name{Local: "settings"},
			}
			return nil
		}
		return fmt.Errorf("failed to read settings file %s: %w", sxm.path, err)
	}

	// Unmarshal the file contents into the settings
	err = xml.Unmarshal(file, &sxm.settings)
	if err != nil {
		return fmt.Errorf("failed to unmarshal settings from file %s: %w", sxm.path, err)
	}
	return nil
}

// configureArtifactoryMirror updates or adds the Artifactory mirror in the settings.
func (sxm *SettingsXmlManager) configureArtifactoryMirror(repoUrl, repoName string) error {
	// Find or create the mirror and update it with the provided details
	return sxm.updateMirror(repoUrl, repoName)
}

// buildAltDeploymentRepoString creates the altDeploymentRepository string for Maven Deploy Plugin
func buildAltDeploymentRepoString(repoUrl string) string {
	return fmt.Sprintf("%s::default::%s", ArtifactoryMirrorID, repoUrl)
}

// findDeploymentProfileIndex finds the index of the deployment profile in settings.Profiles
// Returns -1 if not found
func (sxm *SettingsXmlManager) findDeploymentProfileIndex() int {
	for i, profile := range sxm.settings.Profiles {
		if profile.ID == ArtifactoryDeployProfileID {
			return i
		}
	}
	return -1
}

// updateExistingDeploymentProfile updates an existing deployment profile at the given index
func (sxm *SettingsXmlManager) updateExistingDeploymentProfile(profileIndex int, altDeploymentRepo string) {
	profile := sxm.settings.Profiles[profileIndex]

	// Initialize properties if nil
	if profile.Properties == nil {
		profile.Properties = &mavenv1.Properties{}
	}

	// Set/update deployment property (preserve others)
	(*profile.Properties)[AltDeploymentRepositoryProperty] = altDeploymentRepo

	// Set activation
	if profile.Activation == nil {
		profile.Activation = &maven.Activation{
			ActiveByDefault: true,
		}
	} else {
		profile.Activation.ActiveByDefault = true
	}

	// Update the profile in settings
	sxm.settings.Profiles[profileIndex] = profile
}

// createNewDeploymentProfile creates a new deployment profile
func createNewDeploymentProfile(altDeploymentRepo string) maven.Profile {
	return maven.Profile{
		ID: ArtifactoryDeployProfileID,
		Properties: &mavenv1.Properties{
			AltDeploymentRepositoryProperty: altDeploymentRepo,
		},
		Activation: &maven.Activation{
			ActiveByDefault: true,
		},
	}
}

// configureArtifactoryDeployment configures Maven to deploy/push artifacts to Artifactory by default
// This adds a profile with altDeploymentRepository properties that override any pom.xml distributionManagement
// Uses the same server credentials as the mirror configuration (artifactory-mirror)
func (sxm *SettingsXmlManager) configureArtifactoryDeployment(repoUrl string) {
	// Build the altDeploymentRepository string for Maven Deploy Plugin
	// Source: apache/maven-deploy-plugin/src/main/java/org/apache/maven/plugins/deploy/DeployMojo.java
	altDeploymentRepo := buildAltDeploymentRepoString(repoUrl)

	// Find existing profile or create new one
	profileIndex := sxm.findDeploymentProfileIndex()
	if profileIndex >= 0 {
		// Update existing profile
		sxm.updateExistingDeploymentProfile(profileIndex, altDeploymentRepo)
	} else {
		// Create and add new profile
		deployProfile := createNewDeploymentProfile(altDeploymentRepo)
		sxm.settings.Profiles = append(sxm.settings.Profiles, deployProfile)
	}
}

// ConfigureArtifactoryRepository configures both downloading and deployment to Artifactory
// This is the main public API that sets up complete Artifactory integration using the same repository
// for both download (via mirrors) and deployment (via altDeploymentRepository)
func (sxm *SettingsXmlManager) ConfigureArtifactoryRepository(artifactoryUrl, repoName, username, password string) error {
	// Load settings once at the beginning
	err := sxm.loadSettings()
	if err != nil {
		return fmt.Errorf("failed to load settings: %w", err)
	}

	// Build repository URL once for both mirror and deployment
	repoUrl := strings.TrimRight(artifactoryUrl, "/") + "/" + repoName

	// Set server credentials once (used by both mirror and deployment)
	if username != "" && password != "" {
		sxm.updateServerCredentials(username, password)
	}

	// Configure download mirror (without credentials)
	err = sxm.configureArtifactoryMirror(repoUrl, repoName)
	if err != nil {
		return fmt.Errorf("failed to configure Artifactory download mirror: %w", err)
	}

	// Configure deployment to the same repository (without credentials)
	sxm.configureArtifactoryDeployment(repoUrl)

	// Write settings once at the end
	return sxm.writeSettingsToFile()
}

// updateMirror finds the existing mirror or creates a new one and updates it with the provided details.
func (sxm *SettingsXmlManager) updateMirror(repoUrl, repoName string) error {
	// Create the new mirror with the provided details
	updatedMirror := maven.Mirror{
		ID:       ArtifactoryMirrorID,
		Name:     repoName,
		MirrorOf: "*",
		URL:      repoUrl,
	}

	// Find if the mirror already exists
	var foundMirror bool
	for i, mirror := range sxm.settings.Mirrors {
		if mirror.ID == ArtifactoryMirrorID {
			// Override the existing mirror with the updated one
			sxm.settings.Mirrors[i] = updatedMirror
			foundMirror = true
			break
		}
	}

	// If the mirror doesn't exist, add it
	if !foundMirror {
		sxm.settings.Mirrors = append(sxm.settings.Mirrors, updatedMirror)
	}

	return nil
}

// updateServerCredentials updates or adds server credentials in the settings.
func (sxm *SettingsXmlManager) updateServerCredentials(username, password string) {
	// Create the new server with the provided credentials
	updatedServer := mavenv1.Server{
		ID:       ArtifactoryMirrorID,
		Username: username,
		Password: password,
	}

	// Find if the server already exists
	var foundServer bool
	for i, s := range sxm.settings.Servers {
		if s.ID == ArtifactoryMirrorID {
			// Override the existing server with the updated one
			sxm.settings.Servers[i] = updatedServer
			foundServer = true
			break
		}
	}

	// If the server doesn't exist, add it
	if !foundServer {
		sxm.settings.Servers = append(sxm.settings.Servers, updatedServer)
	}
}

// writeSettingsToFile writes the updated settings to the settings.xml file.
func (sxm *SettingsXmlManager) writeSettingsToFile() error {
	// Marshal the updated settings back to XML
	data, err := xml.MarshalIndent(&sxm.settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal settings to XML: %w", err)
	}

	// Add XML header and write to file
	data = append([]byte(xml.Header), data...)
	err = os.MkdirAll(filepath.Dir(sxm.path), 0755)
	if err != nil {
		return fmt.Errorf("failed to create directory for settings file: %w", err)
	}

	err = os.WriteFile(sxm.path, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write settings to file %s: %w", sxm.path, err)
	}

	return nil
}
