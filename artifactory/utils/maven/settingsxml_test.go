package maven

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSettingsXmlManager(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()

	// Set up a test home directory
	setTestHomeDir(t, tempDir)

	// Test with non-existing settings file
	manager, err := NewSettingsXmlManager()
	assert.NoError(t, err, "Expected no error creating manager")

	expectedPath := filepath.Join(tempDir, ".m2", "settings.xml")
	assert.Equal(t, expectedPath, manager.path, "Expected correct path")

	// Should have empty settings initialized with proper structure
	root := manager.doc.SelectElement(xmlElementSettings)
	assert.NotNil(t, root, "Expected root settings element")
	assert.Equal(t, xmlnsURL, root.SelectAttrValue("xmlns", ""), "Expected correct xmlns")
}

func TestNewSettingsXmlManagerWithPath(t *testing.T) {
	tempDir := t.TempDir()
	customPath := filepath.Join(tempDir, "custom-settings.xml")

	manager, err := NewSettingsXmlManagerWithPath(customPath)
	assert.NoError(t, err, "Expected no error creating manager with custom path")
	assert.Equal(t, customPath, manager.path, "Expected custom path")
}

func TestConfigureArtifactoryRepository_NewFile(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.xml")

	manager, err := NewSettingsXmlManagerWithPath(settingsPath)
	require.NoError(t, err)

	// Configure Artifactory repository
	err = manager.ConfigureArtifactoryRepository("https://mycompany.jfrog.io/artifactory", "maven-virtual", "user", "pass")
	assert.NoError(t, err, "Failed to configure Artifactory repository")

	// Verify file was created
	assert.FileExists(t, settingsPath, "Settings file should be created")

	// Read and verify content
	content, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	xmlContent := string(content)

	// Verify expected elements
	expectedElements := []string{
		`<?xml version="1.0" encoding="UTF-8"?>`,
		`<settings xmlns="http://maven.apache.org/SETTINGS/1.2.0"`,
		`<id>artifactory-mirror</id>`,
		`<username>user</username>`,
		`<password>pass</password>`,
		`<url>https://mycompany.jfrog.io/artifactory/maven-virtual</url>`,
		`<mirrorOf>*</mirrorOf>`,
		`<id>artifactory-deploy</id>`,
		`<activeByDefault>true</activeByDefault>`,
		`<altDeploymentRepository>artifactory-mirror::default::https://mycompany.jfrog.io/artifactory/maven-virtual</altDeploymentRepository>`,
	}

	for _, expected := range expectedElements {
		assert.Contains(t, xmlContent, expected, "XML should contain: %s", expected)
	}
}

func TestConfigureArtifactoryRepository_ExistingFile(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.xml")

	// Create existing settings file with some content
	existingContent := `<?xml version="1.0" encoding="UTF-8"?>
<settings xmlns="http://maven.apache.org/SETTINGS/1.2.0"
          xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
          xsi:schemaLocation="http://maven.apache.org/SETTINGS/1.2.0 http://maven.apache.org/xsd/settings-1.2.0.xsd">
  <localRepository>/custom/repo</localRepository>
  <servers>
    <server>
      <id>existing-server</id>
      <username>existing-user</username>
      <password>existing-pass</password>
    </server>
  </servers>
  <profiles>
    <profile>
      <id>existing-profile</id>
      <properties>
        <custom.prop>custom-value</custom.prop>
      </properties>
    </profile>
  </profiles>
</settings>`

	err := os.WriteFile(settingsPath, []byte(existingContent), 0o644)
	require.NoError(t, err)

	manager, err := NewSettingsXmlManagerWithPath(settingsPath)
	require.NoError(t, err)

	// Configure Artifactory repository
	err = manager.ConfigureArtifactoryRepository("https://mycompany.jfrog.io/artifactory", "maven-virtual", "user", "pass")
	assert.NoError(t, err, "Failed to configure Artifactory repository")

	// Read updated content
	content, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	xmlContent := string(content)

	// Verify existing content is preserved
	preservedElements := []string{
		`<localRepository>/custom/repo</localRepository>`,
		`<id>existing-server</id>`,
		`<username>existing-user</username>`,
		`<id>existing-profile</id>`,
		`<custom.prop>custom-value</custom.prop>`,
	}

	for _, preserved := range preservedElements {
		assert.Contains(t, xmlContent, preserved, "Should preserve existing: %s", preserved)
	}

	// Verify new Artifactory content was added
	newElements := []string{
		`<id>artifactory-mirror</id>`,
		`<username>user</username>`,
		`<password>pass</password>`,
		`<id>artifactory-deploy</id>`,
		`<altDeploymentRepository>artifactory-mirror::default::https://mycompany.jfrog.io/artifactory/maven-virtual</altDeploymentRepository>`,
	}

	for _, newElement := range newElements {
		assert.Contains(t, xmlContent, newElement, "Should add new: %s", newElement)
	}
}

func TestConfigureArtifactoryRepository_ComplexExistingFile(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.xml")

	// Create a complex existing settings file
	complexContent := `<?xml version="1.0" encoding="UTF-8"?>
<settings xmlns="http://maven.apache.org/SETTINGS/1.2.0"
          xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
          xsi:schemaLocation="http://maven.apache.org/SETTINGS/1.2.0 http://maven.apache.org/xsd/settings-1.2.0.xsd">
  <localRepository>/custom/maven/repo</localRepository>
  <interactiveMode>true</interactiveMode>
  <offline>false</offline>
  
  <pluginGroups>
    <pluginGroup>org.mortbay.jetty</pluginGroup>
  </pluginGroups>
  
  <servers>
    <server>
      <id>my-company-server</id>
      <username>deployment</username>
      <password>secret123</password>
    </server>
  </servers>
  
  <mirrors>
    <mirror>
      <id>company-mirror</id>
      <name>Company Maven Mirror</name>
      <url>https://company.example.com/maven</url>
      <mirrorOf>external:*</mirrorOf>
    </mirror>
  </mirrors>
  
  <proxies>
    <proxy>
      <id>my-proxy</id>
      <active>true</active>
      <protocol>http</protocol>
      <host>proxy.example.com</host>
      <port>8080</port>
    </proxy>
  </proxies>
  
  <profiles>
    <profile>
      <id>development</id>
      <activation>
        <activeByDefault>true</activeByDefault>
      </activation>
      <properties>
        <maven.compiler.source>1.8</maven.compiler.source>
        <custom.property>custom-value</custom.property>
      </properties>
    </profile>
  </profiles>
  
  <activeProfiles>
    <activeProfile>development</activeProfile>
  </activeProfiles>
</settings>`

	err := os.WriteFile(settingsPath, []byte(complexContent), 0o644)
	require.NoError(t, err)

	manager, err := NewSettingsXmlManagerWithPath(settingsPath)
	require.NoError(t, err)

	// Configure Artifactory repository
	err = manager.ConfigureArtifactoryRepository("https://artifactory.example.com", "libs-repo", "admin", "token123")
	assert.NoError(t, err, "Failed to configure Artifactory repository")

	// Read updated content
	content, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	xmlContent := string(content)

	// Verify ALL existing content is preserved
	preservedElements := []string{
		`<localRepository>/custom/maven/repo</localRepository>`,
		`<interactiveMode>true</interactiveMode>`,
		`<offline>false</offline>`,
		`<pluginGroup>org.mortbay.jetty</pluginGroup>`,
		`<id>my-company-server</id>`,
		`<username>deployment</username>`,
		`<password>secret123</password>`,
		`<id>company-mirror</id>`,
		`<name>Company Maven Mirror</name>`,
		`<url>https://company.example.com/maven</url>`,
		`<mirrorOf>external:*</mirrorOf>`,
		`<id>my-proxy</id>`,
		`<protocol>http</protocol>`,
		`<host>proxy.example.com</host>`,
		`<port>8080</port>`,
		`<id>development</id>`,
		`<maven.compiler.source>1.8</maven.compiler.source>`,
		`<custom.property>custom-value</custom.property>`,
		`<activeProfile>development</activeProfile>`,
	}

	for _, preserved := range preservedElements {
		assert.Contains(t, xmlContent, preserved, "Should preserve complex existing: %s", preserved)
	}

	// Verify new Artifactory content was added correctly
	newElements := []string{
		`<id>artifactory-mirror</id>`,
		`<username>admin</username>`,
		`<password>token123</password>`,
		`<url>https://artifactory.example.com/libs-repo</url>`,
		`<mirrorOf>*</mirrorOf>`,
		`<id>artifactory-deploy</id>`,
		`<altDeploymentRepository>artifactory-mirror::default::https://artifactory.example.com/libs-repo</altDeploymentRepository>`,
	}

	for _, newElement := range newElements {
		assert.Contains(t, xmlContent, newElement, "Should add new: %s", newElement)
	}

	// Verify no xmlns duplication
	xmlnsCount := strings.Count(xmlContent, `xmlns="http://maven.apache.org/SETTINGS/1.2.0"`)
	assert.Equal(t, 1, xmlnsCount, "Should have exactly one xmlns declaration")
}

func TestConfigureArtifactoryRepository_NoCredentials(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.xml")

	manager, err := NewSettingsXmlManagerWithPath(settingsPath)
	require.NoError(t, err)

	// Configure without credentials
	err = manager.ConfigureArtifactoryRepository("https://public.artifactory.com", "public-repo", "", "")
	assert.NoError(t, err, "Failed to configure Artifactory repository without credentials")

	// Read content
	content, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	xmlContent := string(content)

	// Should have mirror and deployment profile but no server credentials
	assert.Contains(t, xmlContent, `<id>artifactory-mirror</id>`)
	assert.Contains(t, xmlContent, `<url>https://public.artifactory.com/public-repo</url>`)
	assert.Contains(t, xmlContent, `<id>artifactory-deploy</id>`)

	// Should not have server credentials
	assert.NotContains(t, xmlContent, `<servers>`)
	assert.NotContains(t, xmlContent, `<username>`)
	assert.NotContains(t, xmlContent, `<password>`)
}

func TestConfigureArtifactoryRepository_ValidationErrors(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.xml")

	manager, err := NewSettingsXmlManagerWithPath(settingsPath)
	require.NoError(t, err)

	// Test empty artifactoryUrl
	err = manager.ConfigureArtifactoryRepository("", "repo", "user", "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "artifactoryUrl cannot be empty")

	// Test empty repoName
	err = manager.ConfigureArtifactoryRepository("https://example.com", "", "user", "pass")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "repoName cannot be empty")
}

func TestConfigureArtifactoryRepository_URLTrimming(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.xml")

	testCases := []struct {
		input    string
		repo     string
		expected string
	}{
		{"https://artifactory.example.com", "my-repo", "https://artifactory.example.com/my-repo"},
		{"https://artifactory.example.com/", "my-repo", "https://artifactory.example.com/my-repo"},
		{"https://artifactory.example.com//", "my-repo", "https://artifactory.example.com/my-repo"},
		{"https://artifactory.example.com///", "my-repo", "https://artifactory.example.com/my-repo"},
	}

	for _, tc := range testCases {
		// Create fresh manager for each test
		manager, err := NewSettingsXmlManagerWithPath(settingsPath)
		require.NoError(t, err)

		err = manager.ConfigureArtifactoryRepository(tc.input, tc.repo, "user", "pass")
		assert.NoError(t, err, "Failed for input: %s", tc.input)

		// Read and verify content
		content, err := os.ReadFile(settingsPath)
		require.NoError(t, err)

		xmlContent := string(content)
		assert.Contains(t, xmlContent, fmt.Sprintf(`<url>%s</url>`, tc.expected),
			"For input '%s', expected URL '%s'", tc.input, tc.expected)

		// Clean up for next iteration
		os.Remove(settingsPath)
	}
}

func TestConfigureArtifactoryRepository_IdempotentOperations(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.xml")

	manager, err := NewSettingsXmlManagerWithPath(settingsPath)
	require.NoError(t, err)

	// Configure once
	err = manager.ConfigureArtifactoryRepository("https://mycompany.jfrog.io/artifactory", "maven-virtual", "user", "pass")
	assert.NoError(t, err)

	// Read first result
	content1, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	// Configure again with same parameters
	manager2, err := NewSettingsXmlManagerWithPath(settingsPath)
	require.NoError(t, err)

	err = manager2.ConfigureArtifactoryRepository("https://mycompany.jfrog.io/artifactory", "maven-virtual", "user", "pass")
	assert.NoError(t, err)

	// Read second result
	content2, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	// Content should be functionally identical (idempotent) - compare element counts, not exact formatting
	xmlContent1 := string(content1)
	xmlContent2 := string(content2)

	// Count key elements in both files - they should be the same
	keyElements := []string{
		`<id>artifactory-mirror</id>`,
		`<username>user</username>`,
		`<password>pass</password>`,
		`<url>https://mycompany.jfrog.io/artifactory/maven-virtual</url>`,
		`<id>artifactory-deploy</id>`,
		`<altDeploymentRepository>artifactory-mirror::default::https://mycompany.jfrog.io/artifactory/maven-virtual</altDeploymentRepository>`,
	}

	for _, element := range keyElements {
		count1 := strings.Count(xmlContent1, element)
		count2 := strings.Count(xmlContent2, element)
		assert.Equal(t, count1, count2, "Element counts should be identical for: %s", element)
	}

	// Should not have duplicate entries
	mirrorCount := strings.Count(xmlContent2, `<id>artifactory-mirror</id>`)
	assert.Equal(t, 2, mirrorCount, "Should have exactly 2 artifactory-mirror IDs (server + mirror)")

	deployProfileCount := strings.Count(xmlContent2, `<id>artifactory-deploy</id>`)
	assert.Equal(t, 1, deployProfileCount, "Should have exactly 1 deployment profile")
}

func TestWriteSettingsToFile_DirectoryCreation(t *testing.T) {
	tempDir := t.TempDir()
	nestedPath := filepath.Join(tempDir, "nested", "deep", "settings.xml")

	manager, err := NewSettingsXmlManagerWithPath(nestedPath)
	require.NoError(t, err)

	err = manager.ConfigureArtifactoryRepository("https://example.com/artifactory", "repo", "user", "pass")
	assert.NoError(t, err, "Should create nested directories")

	// Verify file exists
	assert.FileExists(t, nestedPath, "File should be created in nested directory")
}

// TestComprehensiveXMLPreservation tests that we don't lose ANY data when parsing
// a comprehensive settings.xml file with xmlns declarations and complex structure.
// This is the critical test to ensure our DOM-based approach preserves everything.
func TestComprehensiveXMLPreservation(t *testing.T) {
	tempDir := t.TempDir()
	settingsPath := filepath.Join(tempDir, "settings.xml")

	// Comprehensive settings.xml with all possible Maven elements and xmlns
	comprehensiveXML := `<?xml version="1.0" encoding="UTF-8"?>
<!-- Comprehensive Maven settings.xml -->
<settings xmlns="http://maven.apache.org/SETTINGS/1.2.0"
          xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
          xsi:schemaLocation="http://maven.apache.org/SETTINGS/1.2.0 http://maven.apache.org/xsd/settings-1.2.0.xsd">

  <!-- Basic configuration -->
  <localRepository>/custom/maven/repository</localRepository>
  <interactiveMode>false</interactiveMode>
  <offline>true</offline>
  <usePluginRegistry>false</usePluginRegistry>

  <!-- Plugin groups -->
  <pluginGroups>
    <pluginGroup>org.mortbay.jetty</pluginGroup>
    <pluginGroup>org.springframework.boot</pluginGroup>
    <pluginGroup>com.company.maven.plugins</pluginGroup>
  </pluginGroups>

  <!-- Servers configuration -->
  <servers>
    <server>
      <id>company-releases</id>
      <username>release-user</username>
      <password>release-password</password>
      <privateKey>${user.home}/.ssh/id_rsa</privateKey>
      <passphrase>ssh-passphrase</passphrase>
      <filePermissions>644</filePermissions>
      <directoryPermissions>755</directoryPermissions>
    </server>
    <server>
      <id>company-snapshots</id>
      <username>snapshot-user</username>
      <password>{ENCRYPTED_PASSWORD_123}</password>
    </server>
    <server>
      <id>external-repo</id>
      <username>external-user</username>
      <password>external-pass</password>
    </server>
  </servers>

  <!-- Mirrors configuration -->
  <mirrors>
    <mirror>
      <id>company-central</id>
      <name>Company Central Mirror</name>
      <url>https://maven.company.com/central</url>
      <mirrorOf>central</mirrorOf>
    </mirror>
    <mirror>
      <id>company-all</id>
      <name>Company All Repos Mirror</name>
      <url>https://maven.company.com/all</url>
      <mirrorOf>*,!local-repo</mirrorOf>
    </mirror>
    <mirror>
      <id>blocked-repo</id>
      <name>Blocked Repository</name>
      <url>http://0.0.0.0/</url>
      <mirrorOf>blocked-*</mirrorOf>
      <blocked>true</blocked>
    </mirror>
  </mirrors>

  <!-- Proxies configuration -->
  <proxies>
    <proxy>
      <id>company-proxy</id>
      <active>true</active>
      <protocol>http</protocol>
      <host>proxy.company.com</host>
      <port>8080</port>
      <username>proxy-user</username>
      <password>proxy-pass</password>
      <nonProxyHosts>localhost|127.0.0.1|*.company.com|internal-*</nonProxyHosts>
    </proxy>
    <proxy>
      <id>secure-proxy</id>
      <active>false</active>
      <protocol>https</protocol>
      <host>secure-proxy.company.com</host>
      <port>8443</port>
    </proxy>
  </proxies>

  <!-- Profiles with complex configurations -->
  <profiles>
    <profile>
      <id>development</id>
      <activation>
        <activeByDefault>true</activeByDefault>
        <jdk>11</jdk>
        <os>
          <name>Windows 10</name>
          <family>Windows</family>
          <arch>amd64</arch>
          <version>10.0</version>
        </os>
        <property>
          <name>environment</name>
          <value>development</value>
        </property>
        <file>
          <exists>dev.properties</exists>
          <missing>prod.properties</missing>
        </file>
      </activation>
      <properties>
        <maven.compiler.source>11</maven.compiler.source>
        <maven.compiler.target>11</maven.compiler.target>
        <spring.version>5.3.21</spring.version>
        <junit.version>5.8.2</junit.version>
        <database.url>jdbc:h2:mem:devdb</database.url>
        <custom.dev.property>development-value</custom.dev.property>
        <special.chars>&lt;&gt;&amp;&quot;&apos;</special.chars>
        <unicode.chars>αβγδε测试</unicode.chars>
      </properties>
      <repositories>
        <repository>
          <id>company-dev-releases</id>
          <name>Company Development Releases</name>
          <url>https://maven.company.com/dev-releases</url>
          <layout>default</layout>
          <releases>
            <enabled>true</enabled>
            <updatePolicy>daily</updatePolicy>
            <checksumPolicy>fail</checksumPolicy>
          </releases>
          <snapshots>
            <enabled>false</enabled>
            <updatePolicy>never</updatePolicy>
            <checksumPolicy>ignore</checksumPolicy>
          </snapshots>
        </repository>
        <repository>
          <id>company-dev-snapshots</id>
          <name>Company Development Snapshots</name>
          <url>https://maven.company.com/dev-snapshots</url>
          <releases>
            <enabled>false</enabled>
          </releases>
          <snapshots>
            <enabled>true</enabled>
            <updatePolicy>always</updatePolicy>
          </snapshots>
        </repository>
      </repositories>
      <pluginRepositories>
        <pluginRepository>
          <id>company-dev-plugins</id>
          <name>Company Development Plugins</name>
          <url>https://maven.company.com/dev-plugins</url>
          <releases>
            <enabled>true</enabled>
            <updatePolicy>never</updatePolicy>
          </releases>
          <snapshots>
            <enabled>false</enabled>
          </snapshots>
        </pluginRepository>
      </pluginRepositories>
    </profile>
    <profile>
      <id>production</id>
      <activation>
        <property>
          <name>environment</name>
          <value>production</value>
        </property>
      </activation>
      <properties>
        <environment>production</environment>
        <skip.tests>false</skip.tests>
        <maven.test.skip>false</maven.test.skip>
        <database.url>jdbc:postgresql://prod-db:5432/myapp</database.url>
        <custom.prod.property>production-value</custom.prod.property>
      </properties>
    </profile>
    <profile>
      <id>testing</id>
      <properties>
        <test.database.url>jdbc:h2:mem:testdb</test.database.url>
        <test.database.driver>org.h2.Driver</test.database.driver>
        <test.environment>true</test.environment>
        <custom.test.property>testing-value</custom.test.property>
      </properties>
    </profile>
  </profiles>

  <!-- Active profiles -->
  <activeProfiles>
    <activeProfile>development</activeProfile>
    <activeProfile>testing</activeProfile>
  </activeProfiles>

</settings>`

	// Write the comprehensive settings.xml
	err := os.WriteFile(settingsPath, []byte(comprehensiveXML), 0o644)
	require.NoError(t, err)

	// Parse with our SettingsXmlManager and configure Artifactory
	manager, err := NewSettingsXmlManagerWithPath(settingsPath)
	require.NoError(t, err)

	err = manager.ConfigureArtifactoryRepository("https://artifactory.example.com", "maven-virtual", "admin", "secret123")
	require.NoError(t, err)

	// Read the modified content
	modifiedContent, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	modifiedXML := string(modifiedContent)

	// === CRITICAL PRESERVATION TESTS ===

	// Test 1: xmlns declarations are not duplicated
	xmlnsCount := strings.Count(modifiedXML, `xmlns="http://maven.apache.org/SETTINGS/1.2.0"`)
	assert.Equal(t, 1, xmlnsCount, "Should have exactly ONE xmlns declaration")

	xsiCount := strings.Count(modifiedXML, `xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`)
	assert.Equal(t, 1, xsiCount, "Should have exactly ONE xmlns:xsi declaration")

	schemaLocationCount := strings.Count(modifiedXML, `xsi:schemaLocation=`)
	assert.Equal(t, 1, schemaLocationCount, "Should have exactly ONE schemaLocation declaration")

	// Test 2: ALL basic configuration is preserved
	basicConfigElements := []string{
		`<localRepository>/custom/maven/repository</localRepository>`,
		`<interactiveMode>false</interactiveMode>`,
		`<offline>true</offline>`,
		`<usePluginRegistry>false</usePluginRegistry>`,
	}
	for _, element := range basicConfigElements {
		assert.Contains(t, modifiedXML, element, "Basic config should be preserved: %s", element)
	}

	// Test 3: ALL plugin groups are preserved
	pluginGroups := []string{
		`<pluginGroup>org.mortbay.jetty</pluginGroup>`,
		`<pluginGroup>org.springframework.boot</pluginGroup>`,
		`<pluginGroup>com.company.maven.plugins</pluginGroup>`,
	}
	for _, group := range pluginGroups {
		assert.Contains(t, modifiedXML, group, "Plugin group should be preserved: %s", group)
	}

	// Test 4: ALL original servers are preserved (3 original + 1 new = 4 total)
	originalServers := []string{
		`<id>company-releases</id>`,
		`<username>release-user</username>`,
		`<password>release-password</password>`,
		`<privateKey>${user.home}/.ssh/id_rsa</privateKey>`,
		`<passphrase>ssh-passphrase</passphrase>`,
		`<filePermissions>644</filePermissions>`,
		`<directoryPermissions>755</directoryPermissions>`,
		`<id>company-snapshots</id>`,
		`<username>snapshot-user</username>`,
		`<password>{ENCRYPTED_PASSWORD_123}</password>`,
		`<id>external-repo</id>`,
		`<username>external-user</username>`,
		`<password>external-pass</password>`,
	}
	for _, server := range originalServers {
		assert.Contains(t, modifiedXML, server, "Original server data should be preserved: %s", server)
	}

	// Test 5: ALL original mirrors are preserved (3 original + 1 new = 4 total)
	originalMirrors := []string{
		`<id>company-central</id>`,
		`<name>Company Central Mirror</name>`,
		`<url>https://maven.company.com/central</url>`,
		`<mirrorOf>central</mirrorOf>`,
		`<id>company-all</id>`,
		`<mirrorOf>*,!local-repo</mirrorOf>`,
		`<id>blocked-repo</id>`,
		`<mirrorOf>blocked-*</mirrorOf>`,
		`<blocked>true</blocked>`,
	}
	for _, mirror := range originalMirrors {
		assert.Contains(t, modifiedXML, mirror, "Original mirror data should be preserved: %s", mirror)
	}

	// Test 6: ALL proxy configurations are preserved
	proxyElements := []string{
		`<id>company-proxy</id>`,
		`<active>true</active>`,
		`<protocol>http</protocol>`,
		`<host>proxy.company.com</host>`,
		`<port>8080</port>`,
		`<username>proxy-user</username>`,
		`<password>proxy-pass</password>`,
		`<nonProxyHosts>localhost|127.0.0.1|*.company.com|internal-*</nonProxyHosts>`,
		`<id>secure-proxy</id>`,
		`<active>false</active>`,
		`<protocol>https</protocol>`,
	}
	for _, proxy := range proxyElements {
		assert.Contains(t, modifiedXML, proxy, "Proxy configuration should be preserved: %s", proxy)
	}

	// Test 7: ALL complex profile configurations are preserved
	profileElements := []string{
		`<id>development</id>`,
		`<activeByDefault>true</activeByDefault>`,
		`<jdk>11</jdk>`,
		`<name>Windows 10</name>`,
		`<family>Windows</family>`,
		`<arch>amd64</arch>`,
		`<version>10.0</version>`,
		`<name>environment</name>`,
		`<value>development</value>`,
		`<exists>dev.properties</exists>`,
		`<missing>prod.properties</missing>`,
		`<maven.compiler.source>11</maven.compiler.source>`,
		`<maven.compiler.target>11</maven.compiler.target>`,
		`<spring.version>5.3.21</spring.version>`,
		`<junit.version>5.8.2</junit.version>`,
		`<database.url>jdbc:h2:mem:devdb</database.url>`,
		`<custom.dev.property>development-value</custom.dev.property>`,
		`<special.chars>&lt;&gt;&amp;&quot;&apos;</special.chars>`,
		`<unicode.chars>αβγδε测试</unicode.chars>`,
		`<id>production</id>`,
		`<environment>production</environment>`,
		`<custom.prod.property>production-value</custom.prod.property>`,
		`<id>testing</id>`,
		`<test.database.url>jdbc:h2:mem:testdb</test.database.url>`,
		`<custom.test.property>testing-value</custom.test.property>`,
	}
	for _, profile := range profileElements {
		assert.Contains(t, modifiedXML, profile, "Profile configuration should be preserved: %s", profile)
	}

	// Test 8: Complex repository configurations are preserved
	repositoryElements := []string{
		`<id>company-dev-releases</id>`,
		`<name>Company Development Releases</name>`,
		`<url>https://maven.company.com/dev-releases</url>`,
		`<layout>default</layout>`,
		`<enabled>true</enabled>`,
		`<updatePolicy>daily</updatePolicy>`,
		`<checksumPolicy>fail</checksumPolicy>`,
		`<enabled>false</enabled>`,
		`<updatePolicy>never</updatePolicy>`,
		`<checksumPolicy>ignore</checksumPolicy>`,
		`<id>company-dev-snapshots</id>`,
		`<updatePolicy>always</updatePolicy>`,
		`<id>company-dev-plugins</id>`,
		`<name>Company Development Plugins</name>`,
	}
	for _, repo := range repositoryElements {
		assert.Contains(t, modifiedXML, repo, "Repository configuration should be preserved: %s", repo)
	}

	// Test 9: Active profiles are preserved
	activeProfileElements := []string{
		`<activeProfile>development</activeProfile>`,
		`<activeProfile>testing</activeProfile>`,
	}
	for _, activeProfile := range activeProfileElements {
		assert.Contains(t, modifiedXML, activeProfile, "Active profile should be preserved: %s", activeProfile)
	}

	// Test 10: NEW Artifactory configuration is properly added
	newArtifactoryElements := []string{
		`<id>artifactory-mirror</id>`,
		`<username>admin</username>`,
		`<password>secret123</password>`,
		`<url>https://artifactory.example.com/maven-virtual</url>`,
		`<mirrorOf>*</mirrorOf>`,
		`<id>artifactory-deploy</id>`,
		`<altDeploymentRepository>artifactory-mirror::default::https://artifactory.example.com/maven-virtual</altDeploymentRepository>`,
	}
	for _, artifactory := range newArtifactoryElements {
		assert.Contains(t, modifiedXML, artifactory, "New Artifactory configuration should be added: %s", artifactory)
	}

	// Test 11: Element counts are correct (preserved + new)
	serverCount := strings.Count(modifiedXML, `<server>`)
	assert.Equal(t, 4, serverCount, "Should have 4 servers (3 original + 1 new)")

	mirrorCount := strings.Count(modifiedXML, `<mirror>`)
	assert.Equal(t, 4, mirrorCount, "Should have 4 mirrors (3 original + 1 new)")

	profileCount := strings.Count(modifiedXML, `<profile>`)
	assert.Equal(t, 4, profileCount, "Should have 4 profiles (3 original + 1 new)")

	// Test 12: Comments are preserved (if any)
	if strings.Contains(comprehensiveXML, "<!--") {
		assert.Contains(t, modifiedXML, "<!-- Comprehensive Maven settings.xml -->", "Comments should be preserved")
	}

	t.Logf("✅ Comprehensive XML preservation test PASSED!")
	t.Logf("   - ALL original configuration preserved")
	t.Logf("   - xmlns declarations not duplicated")
	t.Logf("   - Complex nested structures intact")
	t.Logf("   - Special characters and Unicode preserved")
	t.Logf("   - New Artifactory configuration properly added")
	t.Logf("   - Element counts correct: %d servers, %d mirrors, %d profiles", serverCount, mirrorCount, profileCount)
}

// Helper function to set home directory for cross-platform testing
func setTestHomeDir(t *testing.T, tempDir string) {
	originalHome, err := os.UserHomeDir()
	require.NoError(t, err)

	homeEnv := "HOME"
	if runtime.GOOS == "windows" {
		homeEnv = "USERPROFILE"
	}

	err = os.Setenv(homeEnv, tempDir)
	require.NoError(t, err)

	t.Cleanup(func() {
		err := os.Setenv(homeEnv, originalHome)
		if err != nil {
			t.Logf("Failed to restore %s environment variable: %v", homeEnv, err)
		}
	})
}
