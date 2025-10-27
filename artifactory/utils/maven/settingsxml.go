package maven

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/beevik/etree"
)

const (
	// ArtifactoryMirrorID is the ID used for the Artifactory mirror.
	ArtifactoryMirrorID = "artifactory-mirror"

	// ArtifactoryDeployProfileID is the ID used for the Artifactory deployment profile.
	ArtifactoryDeployProfileID = "artifactory-deploy"

	// AltDeploymentRepositoryProperty is the Maven property for overriding deployment repository.
	AltDeploymentRepositoryProperty = "altDeploymentRepository"

	// mirrorOfAllRepositories configures the mirror to proxy all repositories.
	mirrorOfAllRepositories = "*"

	// XML element names
	xmlElementSettings        = "settings"
	xmlElementServers         = "servers"
	xmlElementServer          = "server"
	xmlElementMirrors         = "mirrors"
	xmlElementMirror          = "mirror"
	xmlElementProfiles        = "profiles"
	xmlElementProfile         = "profile"
	xmlElementID              = "id"
	xmlElementUsername        = "username"
	xmlElementPassword        = "password"
	xmlElementName            = "name"
	xmlElementURL             = "url"
	xmlElementMirrorOf        = "mirrorOf"
	xmlElementActivation      = "activation"
	xmlElementActiveByDefault = "activeByDefault"
	xmlElementProperties      = "properties"

	// XML namespace constants
	// jfrog-ignore - Maven XML namespace URL, required by specification
	xmlnsURL = "http://maven.apache.org/SETTINGS/1.2.0"
	// jfrog-ignore - W3C XML Schema namespace URL, standard specification
	xmlnsXsi = "http://www.w3.org/2001/XMLSchema-instance"
	// jfrog-ignore - Maven XSD schema URLs, required by specification
	xsiSchemaLocationURL = "http://maven.apache.org/SETTINGS/1.2.0 http://maven.apache.org/xsd/settings-1.2.0.xsd"
)

// SettingsXmlManager manages the Maven settings file (settings.xml).
// It provides methods to read, modify, and write Maven configuration while
// preserving all existing user settings.
type SettingsXmlManager struct {
	path string          // Absolute path to the settings.xml file
	doc  *etree.Document // XML document tree representation
}

// NewSettingsXmlManager creates a new SettingsXmlManager instance.
// It automatically loads the existing settings from the settings.xml file if it exists,
// or initializes a new document with proper Maven XML structure if the file is not found.
// The settings.xml location is determined by the user's home directory (~/.m2/settings.xml).
func NewSettingsXmlManager() (*SettingsXmlManager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}
	return NewSettingsXmlManagerWithPath(filepath.Join(homeDir, ".m2", "settings.xml"))
}

// NewSettingsXmlManagerWithPath creates a new SettingsXmlManager with a custom settings.xml path.
// This is useful for testing or when using a non-standard Maven settings location.
func NewSettingsXmlManagerWithPath(settingsPath string) (*SettingsXmlManager, error) {
	manager := &SettingsXmlManager{
		path: settingsPath,
		doc:  etree.NewDocument(),
	}

	// Load existing settings from file
	manager.loadSettings()

	return manager, nil
}

// loadSettings reads the settings.xml file or creates a new one if it doesn't exist.
func (sxm *SettingsXmlManager) loadSettings() {
	if err := sxm.doc.ReadFromFile(sxm.path); err != nil {
		// If file doesn't exist, create a new settings document with proper structure
		sxm.doc = etree.NewDocument()
		sxm.doc.CreateProcInst("xml", `version="1.0" encoding="UTF-8"`)
		root := sxm.doc.CreateElement(xmlElementSettings)
		root.CreateAttr("xmlns", xmlnsURL)
		root.CreateAttr("xmlns:xsi", xmlnsXsi)
		root.CreateAttr("xsi:schemaLocation", xsiSchemaLocationURL)
	}
	sxm.doc.Indent(2)
}

// buildRepositoryURL constructs the full repository URL from base URL and repository name.
func buildRepositoryURL(artifactoryUrl, repoName string) string {
	return strings.TrimRight(artifactoryUrl, "/") + "/" + repoName
}

// ConfigureArtifactoryRepository configures Maven to use Artifactory for both downloading and deployment.
// It updates or creates the following in settings.xml:
//   - Mirror configuration for downloading artifacts from Artifactory
//   - Server credentials for authentication (if username and password are provided)
//   - Deployment profile with altDeploymentRepository property for mvn deploy
//
// All existing configuration in settings.xml is preserved.
//
// Parameters:
//   - artifactoryUrl: Base URL of the Artifactory instance (e.g., "https://mycompany.jfrog.io/artifactory")
//   - repoName: Name of the Artifactory repository (e.g., "maven-virtual")
//   - username: Username for authentication (optional, can be empty for anonymous access)
//   - password: Password or access token for authentication (optional, can be empty for anonymous access)
func (sxm *SettingsXmlManager) ConfigureArtifactoryRepository(artifactoryUrl, repoName, username, password string) error {
	// Validate required parameters
	if artifactoryUrl == "" {
		return fmt.Errorf("artifactoryUrl cannot be empty")
	}
	if repoName == "" {
		return fmt.Errorf("repoName cannot be empty")
	}

	// Build repository URL
	repoUrl := buildRepositoryURL(artifactoryUrl, repoName)

	// Ensure we have a root <settings> element
	root := sxm.doc.SelectElement(xmlElementSettings)
	if root == nil {
		return fmt.Errorf("invalid settings.xml: missing <%s> root element", xmlElementSettings)
	}

	// Configure server credentials
	if username != "" && password != "" {
		sxm.configureServer(root, username, password)
	}

	// Configure mirror
	sxm.configureMirror(root, repoUrl, repoName)

	// Configure deployment profile
	sxm.configureDeploymentProfile(root, repoUrl)

	// Write settings to file
	return sxm.writeSettingsToFile()
}

// configureServer updates or creates the server entry for authentication.
func (sxm *SettingsXmlManager) configureServer(root *etree.Element, username, password string) {
	servers := getOrCreateElement(root, xmlElementServers)
	server := findOrCreateElementByID(servers, xmlElementServer, ArtifactoryMirrorID)

	setOrCreateChildElement(server, xmlElementID, ArtifactoryMirrorID)
	setOrCreateChildElement(server, xmlElementUsername, username)
	setOrCreateChildElement(server, xmlElementPassword, password)
}

// configureMirror updates or creates the mirror entry.
func (sxm *SettingsXmlManager) configureMirror(root *etree.Element, repoUrl, repoName string) {
	mirrors := getOrCreateElement(root, xmlElementMirrors)
	mirror := findOrCreateElementByID(mirrors, xmlElementMirror, ArtifactoryMirrorID)

	setOrCreateChildElement(mirror, xmlElementID, ArtifactoryMirrorID)
	setOrCreateChildElement(mirror, xmlElementName, repoName)
	setOrCreateChildElement(mirror, xmlElementURL, repoUrl)
	setOrCreateChildElement(mirror, xmlElementMirrorOf, mirrorOfAllRepositories)
}

// configureDeploymentProfile updates or creates the deployment profile.
func (sxm *SettingsXmlManager) configureDeploymentProfile(root *etree.Element, repoUrl string) {
	altDeploymentRepo := fmt.Sprintf("%s::default::%s", ArtifactoryMirrorID, repoUrl)

	profiles := getOrCreateElement(root, xmlElementProfiles)
	profile := findOrCreateElementByID(profiles, xmlElementProfile, ArtifactoryDeployProfileID)

	setOrCreateChildElement(profile, xmlElementID, ArtifactoryDeployProfileID)

	activation := getOrCreateElement(profile, xmlElementActivation)
	setOrCreateChildElement(activation, xmlElementActiveByDefault, "true")

	properties := getOrCreateElement(profile, xmlElementProperties)
	setOrCreateChildElement(properties, AltDeploymentRepositoryProperty, altDeploymentRepo)
}

// getOrCreateElement finds a child element or creates it if it doesn't exist.
func getOrCreateElement(parent *etree.Element, name string) *etree.Element {
	element := parent.SelectElement(name)
	if element == nil {
		element = parent.CreateElement(name)
	}
	return element
}

// findElementByID finds an element with a specific ID within a parent container.
// Returns nil if not found.
func findElementByID(parent *etree.Element, elementName, id string) *etree.Element {
	for _, elem := range parent.SelectElements(elementName) {
		if idElem := elem.SelectElement(xmlElementID); idElem != nil && idElem.Text() == id {
			return elem
		}
	}
	return nil
}

// findOrCreateElementByID finds an element with a specific ID or creates a new one.
func findOrCreateElementByID(parent *etree.Element, elementName, id string) *etree.Element {
	elem := findElementByID(parent, elementName, id)
	if elem == nil {
		elem = parent.CreateElement(elementName)
	}
	return elem
}

// removeElementByID removes an element with a specific ID from its parent container.
func removeElementByID(parent *etree.Element, elementName, id string) {
	elem := findElementByID(parent, elementName, id)
	if elem != nil {
		parent.RemoveChild(elem)
	}
}

// removeEmptyContainer removes a container element from its parent if it has no children.
func removeEmptyContainer(root *etree.Element, containerName string) {
	container := root.SelectElement(containerName)
	if container != nil && len(container.ChildElements()) == 0 {
		root.RemoveChild(container)
	}
}

// setOrCreateChildElement sets or creates a child element with the given name and text.
func setOrCreateChildElement(parent *etree.Element, name, text string) {
	child := getOrCreateElement(parent, name)
	child.SetText(text)
}

// ValidateArtifactoryRepository checks if Artifactory repository configuration exists in settings.xml
// and optionally validates the configuration values match the expected parameters.
// Returns true if all components (mirror, server, profile) are configured correctly, false otherwise.
//
// Parameters:
//   - artifactoryUrl: Base URL to validate against (optional, can be empty to skip validation)
//   - repoName: Repository name to validate against (optional, can be empty to skip validation)
//   - username: Username to validate against (optional, can be empty to skip validation)
//   - password: Password to validate against (optional, can be empty to skip validation)
func (sxm *SettingsXmlManager) ValidateArtifactoryRepository(artifactoryUrl, repoName, username, password string) (bool, error) {
	root := sxm.doc.SelectElement(xmlElementSettings)
	if root == nil {
		return false, fmt.Errorf("invalid settings.xml: missing <%s> root element", xmlElementSettings)
	}

	// Build expected repository URL if parameters provided
	var expectedRepoUrl string
	if artifactoryUrl != "" && repoName != "" {
		expectedRepoUrl = buildRepositoryURL(artifactoryUrl, repoName)
	}

	// Check mirror configuration
	mirrors := root.SelectElement(xmlElementMirrors)
	mirrorConfigured := false
	if mirrors != nil {
		mirror := findElementByID(mirrors, xmlElementMirror, ArtifactoryMirrorID)
		if mirror != nil {
			mirrorConfigured = true
			// Validate URL if provided
			if expectedRepoUrl != "" {
				urlElem := mirror.SelectElement(xmlElementURL)
				if urlElem == nil || urlElem.Text() != expectedRepoUrl {
					return false, nil
				}
			}
		}
	}

	// Check server configuration
	servers := root.SelectElement(xmlElementServers)
	serverConfigured := false
	if servers != nil {
		server := findElementByID(servers, xmlElementServer, ArtifactoryMirrorID)
		if server != nil {
			serverConfigured = true
			// Validate credentials if provided
			if username != "" {
				usernameElem := server.SelectElement(xmlElementUsername)
				if usernameElem == nil || usernameElem.Text() != username {
					return false, nil
				}
			}
			if password != "" {
				passwordElem := server.SelectElement(xmlElementPassword)
				if passwordElem == nil || passwordElem.Text() != password {
					return false, nil
				}
			}
		}
	}

	// Check deployment profile
	profiles := root.SelectElement(xmlElementProfiles)
	profileConfigured := false
	if profiles != nil {
		profile := findElementByID(profiles, xmlElementProfile, ArtifactoryDeployProfileID)
		if profile != nil {
			profileConfigured = true
			// Validate altDeploymentRepository if URL provided
			if expectedRepoUrl != "" {
				properties := profile.SelectElement(xmlElementProperties)
				if properties != nil {
					altDeployElem := properties.SelectElement(AltDeploymentRepositoryProperty)
					expectedAltDeploy := fmt.Sprintf("%s::default::%s", ArtifactoryMirrorID, expectedRepoUrl)
					if altDeployElem == nil || altDeployElem.Text() != expectedAltDeploy {
						return false, nil
					}
				}
			}
		}
	}

	return mirrorConfigured && serverConfigured && profileConfigured, nil
}

// RemoveArtifactoryRepository removes all Artifactory configuration from settings.xml.
// This includes:
//   - Mirror configuration with ArtifactoryMirrorID
//   - Server credentials with ArtifactoryMirrorID
//   - Deployment profile with ArtifactoryDeployProfileID
//
// Parameters:
//   - artifactoryUrl: Base URL of the Artifactory instance (used for verification, optional)
//   - repoName: Name of the Artifactory repository (used for verification, optional)
//
// Returns an error if the settings.xml cannot be updated.
func (sxm *SettingsXmlManager) RemoveArtifactoryRepository(artifactoryUrl, repoName string) error {
	root := sxm.doc.SelectElement(xmlElementSettings)
	if root == nil {
		return fmt.Errorf("invalid settings.xml: missing <%s> root element", xmlElementSettings)
	}

	// Build repository URL for verification if provided
	var repoUrl string
	if artifactoryUrl != "" && repoName != "" {
		repoUrl = buildRepositoryURL(artifactoryUrl, repoName)
	}

	// Remove mirror
	if err := sxm.removeMirror(root, repoUrl); err != nil {
		return err
	}

	// Remove server
	sxm.removeServer(root)

	// Remove deployment profile
	sxm.removeDeploymentProfile(root)

	// Write settings to file
	return sxm.writeSettingsToFile()
}

// removeMirror removes the Artifactory mirror entry.
func (sxm *SettingsXmlManager) removeMirror(root *etree.Element, expectedUrl string) error {
	mirrors := root.SelectElement(xmlElementMirrors)
	if mirrors == nil {
		return nil
	}

	// Verify URL if provided before removing
	if expectedUrl != "" {
		mirror := findElementByID(mirrors, xmlElementMirror, ArtifactoryMirrorID)
		if mirror != nil {
			urlElem := mirror.SelectElement(xmlElementURL)
			if urlElem != nil && urlElem.Text() != expectedUrl {
				return fmt.Errorf("mirror URL mismatch: expected %s, found %s", expectedUrl, urlElem.Text())
			}
		}
	}

	removeElementByID(mirrors, xmlElementMirror, ArtifactoryMirrorID)
	removeEmptyContainer(root, xmlElementMirrors)

	return nil
}

// removeServer removes the Artifactory server entry.
func (sxm *SettingsXmlManager) removeServer(root *etree.Element) {
	servers := root.SelectElement(xmlElementServers)
	if servers == nil {
		return
	}

	removeElementByID(servers, xmlElementServer, ArtifactoryMirrorID)
	removeEmptyContainer(root, xmlElementServers)
}

// removeDeploymentProfile removes the Artifactory deployment profile.
func (sxm *SettingsXmlManager) removeDeploymentProfile(root *etree.Element) {
	profiles := root.SelectElement(xmlElementProfiles)
	if profiles == nil {
		return
	}

	removeElementByID(profiles, xmlElementProfile, ArtifactoryDeployProfileID)
	removeEmptyContainer(root, xmlElementProfiles)
}

// writeSettingsToFile writes the document to the settings.xml file.
func (sxm *SettingsXmlManager) writeSettingsToFile() error {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(sxm.path), 0o755); err != nil {
		return fmt.Errorf("failed to create directory for settings file: %w", err)
	}

	// Write to file
	if err := sxm.doc.WriteToFile(sxm.path); err != nil {
		return fmt.Errorf("failed to write settings to file %s: %w", sxm.path, err)
	}

	return nil
}
