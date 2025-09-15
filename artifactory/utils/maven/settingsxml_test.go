package maven

import (
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewSettingsXmlManager(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "maven-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set up a test home directory
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Test with non-existing settings file
	manager, err := NewSettingsXmlManager()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	expectedPath := filepath.Join(tempDir, ".m2", "settings.xml")
	if manager.path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, manager.path)
	}

	// Should have empty settings initialized
	if manager.settings.XMLName.Local != "settings" {
		t.Errorf("Expected XMLName.Local to be 'settings', got %s", manager.settings.XMLName.Local)
	}
}

func TestLoadSettings_NonExistentFile(t *testing.T) {
	manager := &SettingsXmlManager{
		path: "/non/existent/path/settings.xml",
	}

	err := manager.loadSettings()
	if err != nil {
		t.Fatalf("Expected no error for non-existent file, got: %v", err)
	}

	if manager.settings.XMLName.Local != "settings" {
		t.Errorf("Expected XMLName.Local to be 'settings', got %s", manager.settings.XMLName.Local)
	}
}

func TestLoadSettings_ExistingFile(t *testing.T) {
	// Create a temporary settings file
	tempDir, err := os.MkdirTemp("", "maven-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	settingsPath := filepath.Join(tempDir, "settings.xml")
	settingsContent := `<?xml version="1.0" encoding="UTF-8"?>
<settings>
  <localRepository>/path/to/local/repo</localRepository>
  <servers>
    <server>
      <id>test-server</id>
      <username>testuser</username>
      <password>testpass</password>
    </server>
  </servers>
  <mirrors>
    <mirror>
      <id>test-mirror</id>
      <name>Test Mirror</name>
      <url>http://test.mirror.com</url>
      <mirrorOf>*</mirrorOf>
    </mirror>
  </mirrors>
</settings>`

	err = os.WriteFile(settingsPath, []byte(settingsContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write test settings file: %v", err)
	}

	manager := &SettingsXmlManager{path: settingsPath}
	err = manager.loadSettings()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify loaded settings
	if manager.settings.LocalRepository != "/path/to/local/repo" {
		t.Errorf("Expected LocalRepository '/path/to/local/repo', got %s", manager.settings.LocalRepository)
	}

	if len(manager.settings.Servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(manager.settings.Servers))
	} else {
		server := manager.settings.Servers[0]
		if server.ID != "test-server" {
			t.Errorf("Expected server ID 'test-server', got %s", server.ID)
		}
		if server.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got %s", server.Username)
		}
		if server.Password != "testpass" {
			t.Errorf("Expected password 'testpass', got %s", server.Password)
		}
	}

	if len(manager.settings.Mirrors) != 1 {
		t.Errorf("Expected 1 mirror, got %d", len(manager.settings.Mirrors))
	} else {
		mirror := manager.settings.Mirrors[0]
		if mirror.ID != "test-mirror" {
			t.Errorf("Expected mirror ID 'test-mirror', got %s", mirror.ID)
		}
		if mirror.Name != "Test Mirror" {
			t.Errorf("Expected mirror name 'Test Mirror', got %s", mirror.Name)
		}
		if mirror.URL != "http://test.mirror.com" {
			t.Errorf("Expected mirror URL 'http://test.mirror.com', got %s", mirror.URL)
		}
		if mirror.MirrorOf != "*" {
			t.Errorf("Expected mirror mirrorOf '*', got %s", mirror.MirrorOf)
		}
	}
}

func TestUpdateMirror_NewMirror(t *testing.T) {
	manager := &SettingsXmlManager{
		settings: Settings{
			XMLName: xml.Name{Local: "settings"},
		},
	}

	err := manager.updateMirror("https://artifactory.example.com", "my-repo")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(manager.settings.Mirrors) != 1 {
		t.Errorf("Expected 1 mirror, got %d", len(manager.settings.Mirrors))
	} else {
		mirror := manager.settings.Mirrors[0]
		if mirror.ID != ArtifactoryMirrorID {
			t.Errorf("Expected mirror ID %s, got %s", ArtifactoryMirrorID, mirror.ID)
		}
		if mirror.Name != "my-repo" {
			t.Errorf("Expected mirror name 'my-repo', got %s", mirror.Name)
		}
		if mirror.URL != "https://artifactory.example.com/my-repo" {
			t.Errorf("Expected mirror URL 'https://artifactory.example.com/my-repo', got %s", mirror.URL)
		}
		if mirror.MirrorOf != "*" {
			t.Errorf("Expected mirror mirrorOf '*', got %s", mirror.MirrorOf)
		}
	}
}

func TestUpdateMirror_ExistingMirror(t *testing.T) {
	existingMirror := Mirror{
		ID:       ArtifactoryMirrorID,
		Name:     "old-repo",
		URL:      "https://old.artifactory.com/old-repo",
		MirrorOf: "*",
	}

	manager := &SettingsXmlManager{
		settings: Settings{
			XMLName: xml.Name{Local: "settings"},
			Mirrors: []Mirror{existingMirror},
		},
	}

	err := manager.updateMirror("https://new.artifactory.com/", "new-repo")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(manager.settings.Mirrors) != 1 {
		t.Errorf("Expected 1 mirror, got %d", len(manager.settings.Mirrors))
	} else {
		mirror := manager.settings.Mirrors[0]
		if mirror.ID != ArtifactoryMirrorID {
			t.Errorf("Expected mirror ID %s, got %s", ArtifactoryMirrorID, mirror.ID)
		}
		if mirror.Name != "new-repo" {
			t.Errorf("Expected mirror name 'new-repo', got %s", mirror.Name)
		}
		if mirror.URL != "https://new.artifactory.com/new-repo" {
			t.Errorf("Expected mirror URL 'https://new.artifactory.com/new-repo', got %s", mirror.URL)
		}
	}
}

func TestUpdateServerCredentials_NewServer(t *testing.T) {
	manager := &SettingsXmlManager{
		settings: Settings{
			XMLName: xml.Name{Local: "settings"},
		},
	}

	err := manager.updateServerCredentials("testuser", "testpass")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(manager.settings.Servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(manager.settings.Servers))
	} else {
		server := manager.settings.Servers[0]
		if server.ID != ArtifactoryMirrorID {
			t.Errorf("Expected server ID %s, got %s", ArtifactoryMirrorID, server.ID)
		}
		if server.Username != "testuser" {
			t.Errorf("Expected username 'testuser', got %s", server.Username)
		}
		if server.Password != "testpass" {
			t.Errorf("Expected password 'testpass', got %s", server.Password)
		}
	}
}

func TestUpdateServerCredentials_ExistingServer(t *testing.T) {
	existingServer := Server{
		XMLName:  xml.Name{Local: "server"},
		ID:       ArtifactoryMirrorID,
		Username: "olduser",
		Password: "oldpass",
	}

	manager := &SettingsXmlManager{
		settings: Settings{
			XMLName: xml.Name{Local: "settings"},
			Servers: []Server{existingServer},
		},
	}

	err := manager.updateServerCredentials("newuser", "newpass")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	if len(manager.settings.Servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(manager.settings.Servers))
	} else {
		server := manager.settings.Servers[0]
		if server.ID != ArtifactoryMirrorID {
			t.Errorf("Expected server ID %s, got %s", ArtifactoryMirrorID, server.ID)
		}
		if server.Username != "newuser" {
			t.Errorf("Expected username 'newuser', got %s", server.Username)
		}
		if server.Password != "newpass" {
			t.Errorf("Expected password 'newpass', got %s", server.Password)
		}
	}
}

func TestWriteSettingsToFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "maven-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	settingsPath := filepath.Join(tempDir, ".m2", "settings.xml")

	manager := &SettingsXmlManager{
		path: settingsPath,
		settings: Settings{
			XMLName:         xml.Name{Local: "settings"},
			LocalRepository: "/path/to/local/repo",
			Servers: []Server{
				{
					XMLName:  xml.Name{Local: "server"},
					ID:       ArtifactoryMirrorID,
					Username: "testuser",
					Password: "testpass",
				},
			},
			Mirrors: []Mirror{
				{
					ID:       ArtifactoryMirrorID,
					Name:     "test-repo",
					URL:      "https://artifactory.example.com/test-repo",
					MirrorOf: "*",
				},
			},
		},
	}

	err = manager.writeSettingsToFile()
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Errorf("Settings file was not created")
	}

	// Read and verify file contents
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Failed to read settings file: %v", err)
	}

	contentStr := string(content)

	// Check for XML header
	if !strings.HasPrefix(contentStr, "<?xml version=\"1.0\" encoding=\"UTF-8\"?>") {
		t.Errorf("Settings file should start with XML header")
	}

	// Check for expected content
	expectedParts := []string{
		"<settings",
		"<localRepository>/path/to/local/repo</localRepository>",
		"<servers>",
		"<server>",
		"<id>artifactory-mirror</id>",
		"<username>testuser</username>",
		"<password>testpass</password>",
		"</server>",
		"</servers>",
		"<mirrors>",
		"<mirror>",
		"<name>test-repo</name>",
		"<url>https://artifactory.example.com/test-repo</url>",
		"<mirrorOf>*</mirrorOf>",
		"</mirror>",
		"</mirrors>",
		"</settings>",
	}

	for _, part := range expectedParts {
		if !strings.Contains(contentStr, part) {
			t.Errorf("Settings file should contain '%s'", part)
		}
	}
}

func TestConfigureArtifactoryMirror_Complete(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "maven-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set up a test home directory
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	manager, err := NewSettingsXmlManager()
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.ConfigureArtifactoryMirror("https://artifactory.example.com", "my-repo", "myuser", "mypass")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify mirror was added
	if len(manager.settings.Mirrors) != 1 {
		t.Errorf("Expected 1 mirror, got %d", len(manager.settings.Mirrors))
	} else {
		mirror := manager.settings.Mirrors[0]
		if mirror.ID != ArtifactoryMirrorID {
			t.Errorf("Expected mirror ID %s, got %s", ArtifactoryMirrorID, mirror.ID)
		}
		if mirror.URL != "https://artifactory.example.com/my-repo" {
			t.Errorf("Expected mirror URL 'https://artifactory.example.com/my-repo', got %s", mirror.URL)
		}
	}

	// Verify server was added
	if len(manager.settings.Servers) != 1 {
		t.Errorf("Expected 1 server, got %d", len(manager.settings.Servers))
	} else {
		server := manager.settings.Servers[0]
		if server.ID != ArtifactoryMirrorID {
			t.Errorf("Expected server ID %s, got %s", ArtifactoryMirrorID, server.ID)
		}
		if server.Username != "myuser" {
			t.Errorf("Expected username 'myuser', got %s", server.Username)
		}
		if server.Password != "mypass" {
			t.Errorf("Expected password 'mypass', got %s", server.Password)
		}
	}

	// Verify file was written
	settingsPath := filepath.Join(tempDir, ".m2", "settings.xml")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		t.Errorf("Settings file was not created")
	}
}

func TestConfigureArtifactoryMirror_NoCredentials(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "maven-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set up a test home directory
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	manager, err := NewSettingsXmlManager()
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	err = manager.ConfigureArtifactoryMirror("https://artifactory.example.com/", "my-repo", "", "")
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	// Verify mirror was added
	if len(manager.settings.Mirrors) != 1 {
		t.Errorf("Expected 1 mirror, got %d", len(manager.settings.Mirrors))
	}

	// Verify no server was added (empty credentials)
	if len(manager.settings.Servers) != 0 {
		t.Errorf("Expected 0 servers when no credentials provided, got %d", len(manager.settings.Servers))
	}
}

func TestURLTrimming(t *testing.T) {
	manager := &SettingsXmlManager{
		settings: Settings{
			XMLName: xml.Name{Local: "settings"},
		},
	}

	testCases := []struct {
		input    string
		repo     string
		expected string
	}{
		{"https://artifactory.example.com", "my-repo", "https://artifactory.example.com/my-repo"},
		{"https://artifactory.example.com/", "my-repo", "https://artifactory.example.com/my-repo"},
		{"https://artifactory.example.com//", "my-repo", "https://artifactory.example.com/my-repo"},
	}

	for _, tc := range testCases {
		err := manager.updateMirror(tc.input, tc.repo)
		if err != nil {
			t.Fatalf("Expected no error for input %s, got: %v", tc.input, err)
		}

		if len(manager.settings.Mirrors) == 0 {
			t.Errorf("Expected mirror to be added for input %s", tc.input)
			continue
		}

		mirror := manager.settings.Mirrors[len(manager.settings.Mirrors)-1]
		if mirror.URL != tc.expected {
			t.Errorf("For input %s, expected URL %s, got %s", tc.input, tc.expected, mirror.URL)
		}

		// Reset for next test
		manager.settings.Mirrors = []Mirror{}
	}
}
