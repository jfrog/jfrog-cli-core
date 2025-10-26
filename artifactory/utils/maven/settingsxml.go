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
	repoUrl := strings.TrimRight(artifactoryUrl, "/") + "/" + repoName

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

// findOrCreateElementByID finds an element with a specific ID or creates a new one.
func findOrCreateElementByID(parent *etree.Element, elementName, id string) *etree.Element {
	for _, elem := range parent.SelectElements(elementName) {
		if idElem := elem.SelectElement(xmlElementID); idElem != nil && idElem.Text() == id {
			return elem
		}
	}
	return parent.CreateElement(elementName)
}

// setOrCreateChildElement sets or creates a child element with the given name and text.
func setOrCreateChildElement(parent *etree.Element, name, text string) {
	child := getOrCreateElement(parent, name)
	child.SetText(text)
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
