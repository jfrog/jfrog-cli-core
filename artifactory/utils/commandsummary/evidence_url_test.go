package commandsummary

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateEvidenceUrlByType(t *testing.T) {
	// Set up test environment
	originalWorkflow := os.Getenv(workflowEnvKey)
	err := os.Setenv(workflowEnvKey, "JFrog CLI Core Tests")
	if err != nil {
		assert.FailNow(t, "Failed to set environment variable", err)
	}
	defer func() {
		if originalWorkflow != "" {
			err = os.Setenv(workflowEnvKey, originalWorkflow)
			if err != nil {
				assert.Fail(t, "Failed to restore workflow environment variable", err)
				return
			}
		} else {
			os.Unsetenv(workflowEnvKey)
		}
	}()

	// Configure static markdown config for tests
	StaticMarkdownConfig.setPlatformUrl("https://myplatform.com/")
	StaticMarkdownConfig.setPlatformMajorVersion(7)

	tests := []struct {
		name        string
		data        EvidenceSummaryData
		expectedURL string
		expectError bool
	}{
		{
			name: "Package evidence URL",
			data: EvidenceSummaryData{
				Subject:     "repo/path/package.jar",
				SubjectType: SubjectTypePackage,
			},
			expectedURL: "https://myplatform.com/ui/repos/tree/Evidence/repo/path/package.jar?clearFilter=true&gh_job_id=JFrog+CLI+Core+Tests&gh_section=evidence&m=3&s=1",
		},
		{
			name: "Artifact evidence URL",
			data: EvidenceSummaryData{
				Subject:     "repo/path/artifact.txt",
				SubjectType: SubjectTypeArtifact,
			},
			expectedURL: "https://myplatform.com/ui/repos/tree/Evidence/repo/path/artifact.txt?clearFilter=true&gh_job_id=JFrog+CLI+Core+Tests&gh_section=evidence&m=3&s=1",
		},
		{
			name: "Release bundle evidence URL",
			data: EvidenceSummaryData{
				Subject:              "release-bundles-v2/my-bundle/1.0.0/release-bundle.json.evd",
				SubjectType:          SubjectTypeReleaseBundle,
				ReleaseBundleName:    "my-bundle",
				ReleaseBundleVersion: "1.0.0",
				RepoKey:              "release-bundles-v2",
			},
			expectedURL: "", // Will be checked with custom assertion
		},
		{
			name: "Build evidence URL",
			data: EvidenceSummaryData{
				Subject:        "artifactory-build-info/my-build/123/1234567890.json",
				SubjectType:    SubjectTypeBuild,
				BuildName:      "my-build",
				BuildNumber:    "123",
				BuildTimestamp: "1234567890",
				RepoKey:        "artifactory-build-info",
			},
			expectedURL: "https://myplatform.com/ui/builds/my-build/123/1234567890/Evidence/my-build?buildRepo=artifactory-build-info&gh_job_id=JFrog+CLI+Core+Tests&gh_section=evidence&m=3&s=1",
		},
		{
			name: "Build with special characters in name",
			data: EvidenceSummaryData{
				Subject:        "artifactory-build-info/my build with spaces/123/1234567890.json",
				SubjectType:    SubjectTypeBuild,
				BuildName:      "my build with spaces",
				BuildNumber:    "123",
				BuildTimestamp: "1234567890",
				RepoKey:        "artifactory-build-info",
			},
			expectedURL: "https://myplatform.com/ui/builds/my+build+with+spaces/123/1234567890/Evidence/my+build+with+spaces?buildRepo=artifactory-build-info&gh_job_id=JFrog+CLI+Core+Tests&gh_section=evidence&m=3&s=1",
		},
		{
			name: "Invalid release bundle falls back to artifact URL",
			data: EvidenceSummaryData{
				Subject:     "invalid/path",
				SubjectType: SubjectTypeReleaseBundle,
			},
			expectedURL: "https://myplatform.com/ui/repos/tree/Evidence/invalid/path?clearFilter=true&gh_job_id=JFrog+CLI+Core+Tests&gh_section=evidence&m=3&s=1",
		},
		{
			name: "Invalid build falls back to artifact URL",
			data: EvidenceSummaryData{
				Subject:     "invalid/build/path",
				SubjectType: SubjectTypeBuild,
			},
			expectedURL: "https://myplatform.com/ui/repos/tree/Evidence/invalid/build/path?clearFilter=true&gh_job_id=JFrog+CLI+Core+Tests&gh_section=evidence&m=3&s=1",
		},
		{
			name: "Default type uses artifact URL",
			data: EvidenceSummaryData{
				Subject:     "some/path/file.txt",
				SubjectType: "", // Empty type should default to artifact
			},
			expectedURL: "https://myplatform.com/ui/repos/tree/Evidence/some/path/file.txt?clearFilter=true&gh_job_id=JFrog+CLI+Core+Tests&gh_section=evidence&m=3&s=1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			url, err := GenerateEvidenceUrlByType(tt.data, evidenceSection)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.expectedURL != "" {
					assert.Equal(t, tt.expectedURL, url)
				} else if tt.name == "Release bundle evidence URL" {
					// Special handling for release bundle URL due to parameter ordering
					assert.Contains(t, url, "https://myplatform.com/ui/artifactory/lifecycle?")
					assert.Contains(t, url, "bundleName=my-bundle")
					assert.Contains(t, url, "repositoryKey=release-bundles-v2")
					assert.Contains(t, url, "releaseBundleVersion=1.0.0")
					assert.Contains(t, url, "activeVersionTab=Evidence+Graph")
					assert.Contains(t, url, "gh_job_id=JFrog+CLI+Core+Tests")
					assert.Contains(t, url, "gh_section=evidence")
					assert.Contains(t, url, "m=3")
					assert.Contains(t, url, "s=1")
					assert.Contains(t, url, "range=Any+Time")
				}
			}
		})
	}
}
