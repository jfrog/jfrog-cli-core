package commandsummary

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	buildInfo "github.com/jfrog/build-info-go/entities"
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
)

const (
	buildInfoTable        = "build-info-table.md"
	dockerImageModule     = "docker-image-module.md"
	genericModule         = "generic-module.md"
	mavenModule           = "maven-module.md"
	mavenNestedModule     = "maven-nested-module.md"
	dockerMultiArchModule = "multiarch-docker-image.md"
)

type MockScanResult struct {
	Violations      string
	Vulnerabilities string
}

// GetViolations returns the mock violations
func (m *MockScanResult) GetViolations() string {
	return m.Violations
}

// GetVulnerabilities returns the mock vulnerabilities
func (m *MockScanResult) GetVulnerabilities() string {
	return m.Vulnerabilities
}

func prepareBuildInfoTest() (*BuildInfoSummary, func()) {
	// Mock the scan results defaults
	StaticMarkdownConfig.scanResultsMapping = make(map[string]ScanResult)
	StaticMarkdownConfig.scanResultsMapping[NonScannedResult] = &MockScanResult{
		Violations:      "Not scanned",
		Vulnerabilities: "Not scanned",
	}
	// Mock config
	StaticMarkdownConfig.setPlatformUrl(testPlatformUrl)
	StaticMarkdownConfig.setPlatformMajorVersion(7)
	StaticMarkdownConfig.setExtendedSummary(false)
	// Cleanup config
	cleanup := func() {
		StaticMarkdownConfig.setExtendedSummary(false)
		StaticMarkdownConfig.setPlatformMajorVersion(0)
		StaticMarkdownConfig.setPlatformUrl("")
		_ = os.Unsetenv(githubWorkflowEnv)
	}
	setWorkFlowEnvIfNeeded()
	// Create build info instance
	buildInfoSummary := &BuildInfoSummary{}
	return buildInfoSummary, cleanup
}

func setWorkFlowEnvIfNeeded() {
	// Sets the GitHub workflow environment variable to allow testing locally
	isGitHub := os.Getenv("GITHUB_ACTIONS")
	if isGitHub == "" {
		// This is the name of the GitHub action that executes the JFrog CLI Core Tests
		_ = os.Setenv(githubWorkflowEnv, "JFrog CLI Core Tests")
	}
}

const buildUrl = "http://myJFrogPlatform/builds/buildName/123?gh_job_id=JFrog+CLI+Core+Tests&gh_section=buildInfo"

func TestBuildInfoTable(t *testing.T) {
	buildInfoSummary, cleanUp := prepareBuildInfoTest()
	defer func() {
		cleanUp()
	}()
	var builds = []*buildinfo.BuildInfo{
		{
			Name:     "buildName",
			Number:   "123",
			Started:  "2024-05-05T12:47:20.803+0300",
			BuildUrl: buildUrl,
		},
	}
	t.Run("Extended Summary", func(t *testing.T) {
		StaticMarkdownConfig.setExtendedSummary(true)
		res, err := buildInfoSummary.buildInfoTable(builds)
		assert.NoError(t, err)
		testMarkdownOutput(t, getTestDataFile(t, buildInfoTable), res)
	})
	t.Run("Basic Summary", func(t *testing.T) {
		StaticMarkdownConfig.setExtendedSummary(false)
		res, err := buildInfoSummary.buildInfoTable(builds)
		assert.NoError(t, err)
		testMarkdownOutput(t, getTestDataFile(t, buildInfoTable), res)
	})
}

func TestBuildInfoModulesMaven(t *testing.T) {
	buildInfoSummary, cleanUp := prepareBuildInfoTest()
	defer func() {
		cleanUp()
	}()
	var builds = []*buildinfo.BuildInfo{
		{
			Name:     "buildName",
			Number:   "123",
			Started:  "2024-05-05T12:47:20.803+0300",
			BuildUrl: buildUrl,
			Modules: []buildinfo.Module{
				{
					Id:   "maven",
					Type: buildinfo.Maven,
					Artifacts: []buildinfo.Artifact{{
						Name:                   "artifact1",
						Path:                   "path/to/artifact1",
						OriginalDeploymentRepo: "libs-release",
					}},
					Dependencies: []buildinfo.Dependency{{
						Id: "dep1",
					}},
				},
			},
		},
	}

	t.Run("Extended Summary", func(t *testing.T) {
		StaticMarkdownConfig.setExtendedSummary(true)
		res, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		testMarkdownOutput(t, getTestDataFile(t, mavenModule), res)
	})
	t.Run("Basic Summary", func(t *testing.T) {
		StaticMarkdownConfig.setExtendedSummary(false)
		res, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		testMarkdownOutput(t, getTestDataFile(t, mavenModule), res)
	})
}

func TestBuildInfoModulesMavenWithSubModules(t *testing.T) {
	buildInfoSummary, cleanUp := prepareBuildInfoTest()
	defer func() {
		cleanUp()
	}()
	var builds = []*buildinfo.BuildInfo{
		{
			Name:     "buildName",
			Number:   "123",
			Started:  "2024-05-05T12:47:20.803+0300",
			BuildUrl: buildUrl,
			Modules: []buildinfo.Module{
				{
					Id:   "maven",
					Type: buildinfo.Maven,
					Artifacts: []buildinfo.Artifact{{
						Name:                   "artifact1",
						Path:                   "path/to/artifact1",
						OriginalDeploymentRepo: "libs-release",
					}},
					Dependencies: []buildinfo.Dependency{{
						Id: "dep1",
					}},
				},
				{
					Id:     "submodule1",
					Parent: "maven",
					Type:   buildinfo.Maven,
					Artifacts: []buildinfo.Artifact{{
						Name:                   "artifact2",
						Path:                   "path/to/artifact2",
						OriginalDeploymentRepo: "libs-release",
					}},
					Dependencies: []buildinfo.Dependency{{
						Id: "dep2",
					}},
				},
				{
					Id:     "submodule2",
					Parent: "maven",
					Type:   buildinfo.Maven,
					Artifacts: []buildinfo.Artifact{{
						Name:                   "artifact3",
						Path:                   "path/to/artifact3",
						OriginalDeploymentRepo: "libs-release",
					}},
					Dependencies: []buildinfo.Dependency{{
						Id: "dep3",
					}},
				},
			},
		},
	}

	t.Run("Extended Summary", func(t *testing.T) {
		StaticMarkdownConfig.setExtendedSummary(true)
		res, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		testMarkdownOutput(t, getTestDataFile(t, mavenNestedModule), res)
	})
	t.Run("Basic Summary", func(t *testing.T) {
		StaticMarkdownConfig.setExtendedSummary(false)
		res, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		testMarkdownOutput(t, getTestDataFile(t, mavenNestedModule), res)
	})
}

func TestBuildInfoModulesGradle(t *testing.T) {
	buildInfoSummary, cleanUp := prepareBuildInfoTest()
	defer func() {
		cleanUp()
	}()
	var builds = []*buildinfo.BuildInfo{
		{
			Name:     "buildName",
			Number:   "123",
			Started:  "2024-05-05T12:47:20.803+0300",
			BuildUrl: buildUrl,
			Modules: []buildinfo.Module{
				{
					Id:   "gradle",
					Type: buildinfo.Gradle,
					Artifacts: []buildinfo.Artifact{
						{
							Name:                   "gradleArtifact",
							Path:                   "dir/gradleArtifact",
							OriginalDeploymentRepo: "gradle-local",
						},
					},
				},
			},
		},
	}

	t.Run("Extended Summary", func(t *testing.T) {
		StaticMarkdownConfig.setExtendedSummary(true)
		res, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		assert.Empty(t, res)
	})
	t.Run("Basic Summary", func(t *testing.T) {
		StaticMarkdownConfig.setExtendedSummary(false)
		res, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		assert.Empty(t, res)
	})
}

func TestBuildInfoModulesGeneric(t *testing.T) {
	buildInfoSummary, cleanUp := prepareBuildInfoTest()
	defer func() {
		cleanUp()
	}()
	var builds = []*buildinfo.BuildInfo{
		{
			Name:     "buildName",
			Number:   "123",
			Started:  "2024-05-05T12:47:20.803+0300",
			BuildUrl: buildUrl,
			Modules: []buildinfo.Module{
				{
					Id:   "generic",
					Type: buildinfo.Generic,
					Artifacts: []buildinfo.Artifact{{
						Name:                   "artifact2",
						Path:                   "path/to/artifact2",
						OriginalDeploymentRepo: "generic-local",
					}},
				},
			},
		},
	}

	t.Run("Extended Summary", func(t *testing.T) {
		StaticMarkdownConfig.setExtendedSummary(true)
		res, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		testMarkdownOutput(t, getTestDataFile(t, genericModule), res)
	})
	t.Run("Basic Summary", func(t *testing.T) {
		StaticMarkdownConfig.setExtendedSummary(false)
		res, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		testMarkdownOutput(t, getTestDataFile(t, genericModule), res)
	})
}

func TestDockerModule(t *testing.T) {
	buildInfoSummary, cleanUp := prepareBuildInfoTest()
	defer func() {
		cleanUp()
	}()
	var builds = []*buildinfo.BuildInfo{
		{
			Name:    "dockerx",
			Number:  "1",
			Started: "2024-08-12T11:11:50.198+0300",
			Modules: []buildinfo.Module{
				{
					Properties: map[string]interface{}{
						"docker.image.tag": "ecosysjfrog.jfrog.io/docker-local/multiarch-image:1",
					},
					Type:   "docker",
					Parent: "image:2",
					Id:     "image:2",
					Checksum: buildinfo.Checksum{
						Sha256: "aae9",
					},
					Artifacts: []buildinfo.Artifact{
						{
							Checksum: buildinfo.Checksum{
								Sha1:   "32c1416f8430fbbabd82cb014c5e09c5fe702404",
								Sha256: "aae9",
								Md5:    "f568bfb1c9576a1f06235ebe0389d2d8",
							},
							Name:                   "sha256__aae9",
							Path:                   "image2/sha256:552c/sha256__aae9",
							OriginalDeploymentRepo: "docker-local",
						},
					},
				},
			},
		},
	}

	t.Run("Extended Summary", func(t *testing.T) {
		StaticMarkdownConfig.setExtendedSummary(true)
		res, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		testMarkdownOutput(t, getTestDataFile(t, dockerImageModule), res)
	})
	t.Run("Basic Summary", func(t *testing.T) {
		StaticMarkdownConfig.setExtendedSummary(false)
		res, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		testMarkdownOutput(t, getTestDataFile(t, dockerImageModule), res)
	})

}

func TestDockerMultiArchModule(t *testing.T) {
	buildInfoSummary, cleanUp := prepareBuildInfoTest()
	defer func() {
		cleanUp()
	}()
	var builds = []*buildinfo.BuildInfo{
		{
			Name:    "dockerx",
			Number:  "1",
			Started: "2024-08-12T11:11:50.198+0300",
			Modules: []buildinfo.Module{
				{
					Properties: map[string]interface{}{
						"docker.image.tag": "ecosysjfrog.jfrog.io/docker-local/multiarch-image:1",
					},
					Type: "docker",
					Id:   "multiarch-image:1",
					Artifacts: []buildinfo.Artifact{
						{
							Type: "json",
							Checksum: buildinfo.Checksum{
								Sha1:   "fa",
								Sha256: "2217",
								Md5:    "ba0",
							},
							Name:                   "list.manifest.json",
							Path:                   "multiarch-image/1/list.manifest.json",
							OriginalDeploymentRepo: "docker-local",
						},
					},
				},
				{
					Type:   "docker",
					Parent: "multiarch-image:1",
					Id:     "linux/amd64/multiarch-image:1",
					Artifacts: []buildinfo.Artifact{
						{
							Checksum: buildinfo.Checksum{
								Sha1:   "32",
								Sha256: "sha256:552c",
								Md5:    "f56",
							},
							Name:                   "manifest.json",
							Path:                   "multiarch-image/sha256",
							OriginalDeploymentRepo: "docker-local",
						},
						{
							Checksum: buildinfo.Checksum{
								Sha1:   "32c",
								Sha256: "aae9",
								Md5:    "f56",
							},
							Name:                   "sha256__aae9",
							Path:                   "multiarch-image/sha256:552c/sha256",
							OriginalDeploymentRepo: "docker-local",
						},
					},
				},
			},
		},
	}

	t.Run("Extended Summary", func(t *testing.T) {
		StaticMarkdownConfig.setExtendedSummary(true)
		res, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		testMarkdownOutput(t, getTestDataFile(t, dockerMultiArchModule), res)
	})
	t.Run("Basic Summary", func(t *testing.T) {
		StaticMarkdownConfig.setExtendedSummary(false)
		res, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		testMarkdownOutput(t, getTestDataFile(t, dockerMultiArchModule), res)
	})

}

func TestGroupModules(t *testing.T) {
	tests := []struct {
		name     string
		modules  []buildInfo.Module
		expected map[string][]buildInfo.Module
	}{
		{
			name: "Single module",
			modules: []buildInfo.Module{
				{Id: "module1", Artifacts: []buildInfo.Artifact{{Name: "artifact1"}}},
			},
			expected: map[string][]buildInfo.Module{
				"module1": {
					{Id: "module1", Artifacts: []buildInfo.Artifact{{Name: "artifact1"}}},
				},
			},
		},
		{
			name: "Module with subModules",
			modules: []buildInfo.Module{
				{Id: "module1", Parent: "root", Artifacts: []buildInfo.Artifact{{Name: "artifact1"}}},
				{Id: "module2", Parent: "root", Artifacts: []buildInfo.Artifact{{Name: "artifact2"}}},
			},
			expected: map[string][]buildInfo.Module{
				"root": {
					{Id: "module1", Parent: "root", Artifacts: []buildInfo.Artifact{{Name: "artifact1"}}},
					{Id: "module2", Parent: "root", Artifacts: []buildInfo.Artifact{{Name: "artifact2"}}},
				},
			},
		},
		{
			name: "Multiple Modules",
			modules: []buildInfo.Module{
				{Id: "module1", Parent: "root1", Artifacts: []buildInfo.Artifact{{Name: "artifact1"}}},
				{Id: "module2", Parent: "root2", Artifacts: []buildInfo.Artifact{{Name: "artifact2"}}},
			},
			expected: map[string][]buildInfo.Module{
				"root1": {
					{Id: "module1", Parent: "root1", Artifacts: []buildInfo.Artifact{{Name: "artifact1"}}},
				},
				"root2": {
					{Id: "module2", Parent: "root2", Artifacts: []buildInfo.Artifact{{Name: "artifact2"}}},
				},
			},
		},
		{
			name: "Multiple Modules with subModules",
			modules: []buildInfo.Module{
				{Id: "module1", Parent: "root1", Artifacts: []buildInfo.Artifact{{Name: "artifact1"}}},
				{Id: "module2", Parent: "root1", Artifacts: []buildInfo.Artifact{{Name: "artifact1"}}},
				{Id: "module3", Parent: "root2", Artifacts: []buildInfo.Artifact{{Name: "artifact2"}}},
				{Id: "module4", Parent: "root2", Artifacts: []buildInfo.Artifact{{Name: "artifact2"}}},
			},
			expected: map[string][]buildInfo.Module{
				"root1": {
					{Id: "module1", Parent: "root1", Artifacts: []buildInfo.Artifact{{Name: "artifact1"}}},
					{Id: "module2", Parent: "root1", Artifacts: []buildInfo.Artifact{{Name: "artifact1"}}},
				},
				"root2": {
					{Id: "module3", Parent: "root2", Artifacts: []buildInfo.Artifact{{Name: "artifact2"}}},
					{Id: "module4", Parent: "root2", Artifacts: []buildInfo.Artifact{{Name: "artifact2"}}},
				},
			},
		},
		{
			name: "Module with no artifacts",
			modules: []buildInfo.Module{
				{Id: "module1", Parent: "root1"},
			},
			expected: map[string][]buildInfo.Module{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := groupModules(tt.modules)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Tests data files are location artifactory/commands/testdata/command_summary
func getTestDataFile(t *testing.T, fileName string) string {
	var modulesPath string
	if StaticMarkdownConfig.IsExtendedSummary() {
		modulesPath = filepath.Join("../", "testdata", "command_summaries", "extended", fileName)
	} else {
		modulesPath = filepath.Join("../", "testdata", "command_summaries", "basic", fileName)
	}

	content, err := os.ReadFile(modulesPath)
	assert.NoError(t, err)
	contentStr := string(content)
	if coreutils.IsWindows() {
		contentStr = strings.ReplaceAll(contentStr, "\r\n", "\n")
	}
	return contentStr
}

func TestIsSupportedModule(t *testing.T) {
	tests := []struct {
		name     string
		module   buildInfo.Module
		expected bool
	}{
		{
			name: "Supported Maven Module",
			module: buildInfo.Module{
				Type: buildInfo.Maven,
			},
			expected: true,
		},
		{
			name: "Supported Npm Module",
			module: buildInfo.Module{
				Type: buildInfo.Npm,
			},
			expected: true,
		},
		{
			name: "Unsupported Module Type",
			module: buildInfo.Module{
				Type: buildInfo.ModuleType("unsupported"),
			},
			expected: false,
		},
		{
			name: "Docker Module with Attestations Prefix",
			module: buildInfo.Module{
				Type: buildInfo.Docker,
				Id:   "attestations-module",
			},
			expected: false,
		},
		{
			name: "Docker Module without Attestations Prefix",
			module: buildInfo.Module{
				Type: buildInfo.Docker,
				Id:   "docker-module",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isSupportedModule(&tt.module)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFilterModules(t *testing.T) {
	tests := []struct {
		name     string
		modules  []buildInfo.Module
		expected []buildInfo.Module
	}{
		{
			name: "All Supported Modules",
			modules: []buildInfo.Module{
				{Type: buildInfo.Maven},
				{Type: buildInfo.Npm},
				{Type: buildInfo.Go},
			},
			expected: []buildInfo.Module{
				{Type: buildInfo.Maven},
				{Type: buildInfo.Npm},
				{Type: buildInfo.Go},
			},
		},
		{
			name: "Mixed Supported and Unsupported Modules",
			modules: []buildInfo.Module{
				{Type: buildInfo.Maven},
				{Type: buildInfo.ModuleType("unsupported")},
				{Type: buildInfo.Npm},
			},
			expected: []buildInfo.Module{
				{Type: buildInfo.Maven},
				{Type: buildInfo.Npm},
			},
		},
		{
			name: "All Unsupported Modules",
			modules: []buildInfo.Module{
				{Type: buildInfo.ModuleType("unsupported1")},
				{Type: buildInfo.ModuleType("unsupported2")},
			},
			expected: []buildInfo.Module{},
		},
		{
			name: "Docker Module with Attestations Prefix",
			modules: []buildInfo.Module{
				{Type: buildInfo.Docker, Id: "attestations-module"},
			},
			expected: []buildInfo.Module{},
		},
		{
			name: "Docker Module without Attestations Prefix",
			modules: []buildInfo.Module{
				{Type: buildInfo.Docker, Id: "docker-module"},
			},
			expected: []buildInfo.Module{
				{Type: buildInfo.Docker, Id: "docker-module"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterModules(tt.modules...)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Sometimes there are inconsistencies in the Markdown output, this function normalizes the output for comparison
// This allows easy debugging when tests fails
func normalizeMarkdown(md string) string {
	// Remove the extra spaces added for equal table length
	md = strings.ReplaceAll(md, markdownSpaceFiller, "")
	md = strings.ReplaceAll(md, "\r\n", "\n")
	md = strings.ReplaceAll(md, "\r", "\n")
	md = strings.ReplaceAll(md, `\n`, "\n")
	// Regular expression to match the table rows and header separators
	re := regexp.MustCompile(`\s*\|\s*`)
	// Normalize spaces around the pipes and colons in the Markdown
	lines := strings.Split(md, "\n")
	for i, line := range lines {
		if strings.Contains(line, "|") {
			// Remove extra spaces around pipes and colons
			line = re.ReplaceAllString(line, " | ")
			lines[i] = strings.TrimSpace(line)
		}
	}
	return strings.Join(lines, "\n")
}

func testMarkdownOutput(t *testing.T, expected, actual string) {
	expected = normalizeMarkdown(expected)
	actual = normalizeMarkdown(actual)

	// If the compared string length exceeds the maximum length,
	// the string is not formatted, leading to an unequal comparison.
	// Ensure to test small units of Markdown for better unit testing
	// and to facilitate testing.
	maxCompareLength := 950
	if len(expected) > maxCompareLength || len(actual) > maxCompareLength {
		t.Fatalf("Markdown output is too long to compare, limit the length to %d chars", maxCompareLength)
	}
	assert.Equal(t, expected, actual)
}
