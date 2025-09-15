package maven

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Settings represents the Maven settings.xml structure
type Settings struct {
	XMLName           xml.Name
	XMLNs             string   `xml:"xmlns,attr"`
	XMLNsXsi          string   `xml:"xmlns:xsi,attr"`
	XsiSchemaLocation string   `xml:"xsi:schemaLocation,attr"`
	LocalRepository   string   `xml:"localRepository"`
	Servers           []Server `xml:"servers>server,omitempty"`
	Mirrors           []Mirror `xml:"mirrors>mirror,omitempty"`
}

// Mirror represents a Maven mirror configuration
type Mirror struct {
	ID       string `xml:"id"`
	Name     string `xml:"name,omitempty"`
	URL      string `xml:"url"`
	MirrorOf string `xml:"mirrorOf"`
}

// Server represents a Maven server configuration with credentials
type Server struct {
	XMLName  xml.Name `xml:"server"`
	ID       string   `xml:"id,omitempty"`
	Username string   `xml:"username,omitempty"`
	Password string   `xml:"password,omitempty"`
}

// ArtifactoryMirrorID is the ID used for the Artifactory mirror.
const ArtifactoryMirrorID = "artifactory-mirror"

// SettingsXmlManager manages the maven settings file (`settings.xml`).
type SettingsXmlManager struct {
	path     string
	settings Settings
}

// NewSettingsXmlManager creates a new SettingsXmlManager instance.
// It automatically loads the existing settings from the `settings.xml` file if it exists.
func NewSettingsXmlManager() (*SettingsXmlManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}
	manager := &SettingsXmlManager{
		path: filepath.Join(homeDir, ".m2", "settings.xml"),
	}

	// Load existing settings from file
	err = manager.loadSettings()
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
			sxm.settings = Settings{
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

// ConfigureArtifactoryMirror updates or adds the Artifactory mirror and its credentials in the settings.
func (sxm *SettingsXmlManager) ConfigureArtifactoryMirror(artifactoryUrl, repoName, username, password string) error {
	// Find or create the mirror and update it with the provided details
	if err := sxm.updateMirror(artifactoryUrl, repoName); err != nil {
		return err
	}

	// Update server credentials if needed
	if username != "" && password != "" {
		if err := sxm.updateServerCredentials(username, password); err != nil {
			return err
		}
	}

	// Write the updated settings back to the settings.xml file
	return sxm.writeSettingsToFile()
}

// updateMirror finds the existing mirror or creates a new one and updates it with the provided details.
func (sxm *SettingsXmlManager) updateMirror(artifactoryUrl, repoName string) error {
	// Create the new mirror with the provided details
	updatedMirror := Mirror{
		ID:       ArtifactoryMirrorID,
		Name:     repoName,
		MirrorOf: "*",
		URL:      strings.TrimRight(artifactoryUrl, "/") + "/" + repoName,
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
func (sxm *SettingsXmlManager) updateServerCredentials(username, password string) error {
	// Create the new server with the provided credentials
	updatedServer := Server{
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

	return nil
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
