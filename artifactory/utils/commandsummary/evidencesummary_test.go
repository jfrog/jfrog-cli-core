package commandsummary

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

const (
	evidenceTable = "evidence.md"
)

func prepareEvidenceTest(t *testing.T) (*EvidenceSummary, func()) {
	originalWorkflow := os.Getenv(workflowEnvKey)
	err := os.Setenv(workflowEnvKey, "JFrog CLI Core Tests")
	if err != nil {
		assert.Fail(t, "Failed to set environment variable", err)
	}

	StaticMarkdownConfig.setPlatformUrl(testPlatformUrl)
	StaticMarkdownConfig.setPlatformMajorVersion(7)
	StaticMarkdownConfig.setExtendedSummary(false)

	cleanup := func() {
		StaticMarkdownConfig.setExtendedSummary(false)
		StaticMarkdownConfig.setPlatformMajorVersion(0)
		StaticMarkdownConfig.setPlatformUrl("")

		if originalWorkflow != "" {
			err = os.Setenv(workflowEnvKey, originalWorkflow)
			if err != nil {
				assert.Fail(t, "Failed to set workflow environment variable", err)
			}
		} else {
			_ = os.Unsetenv(workflowEnvKey)
		}
	}

	evidenceSummary := &EvidenceSummary{}
	return evidenceSummary, cleanup
}

func TestEvidenceTable(t *testing.T) {
	evidenceSummary, cleanUp := prepareEvidenceTest(t)
	defer func() {
		cleanUp()
	}()

	createdTime := time.Date(2024, 12, 1, 10, 0, 0, 0, time.UTC)

	var evidenceData = []EvidenceSummaryData{
		{
			Subject:       "cli-sigstore-test/commons-1.0.0.txt",
			SubjectSha256: "",
			PredicateType: "in-toto",
			PredicateSlug: "in-toto",
			Verified:      true,
			DisplayName:   "cli-sigstore-test/commons-1.0.0.txt",
			SubjectType:   SubjectTypeArtifact,
			RepoKey:       "cli-sigstore-test/commons-1.0.0.txt",
			CreatedAt:     createdTime,
		},
	}

	t.Run("Extended Summary", func(t *testing.T) {
		StaticMarkdownConfig.setExtendedSummary(true)
		res := evidenceSummary.generateEvidenceTable(evidenceData)
		testMarkdownOutput(t, getTestDataFile(t, evidenceTable), res)
	})

	t.Run("Basic Summary", func(t *testing.T) {
		StaticMarkdownConfig.setExtendedSummary(false)
		res := evidenceSummary.generateEvidenceTable(evidenceData)
		testMarkdownOutput(t, getTestDataFile(t, evidenceTable), res)
	})
}

func TestFormatSubjectType(t *testing.T) {
	evidenceSummary := &EvidenceSummary{}

	tests := []struct {
		name         string
		subjectType  SubjectType
		expectedIcon string
	}{
		{
			name:         "Artifact",
			subjectType:  SubjectTypeArtifact,
			expectedIcon: "📄",
		},
		{
			name:         "Package",
			subjectType:  SubjectTypePackage,
			expectedIcon: "📦️",
		},
		{
			name:         "Build",
			subjectType:  SubjectTypeBuild,
			expectedIcon: "🛠️️",
		},
		{
			name:         "Release Bundle",
			subjectType:  SubjectTypeReleaseBundle,
			expectedIcon: "🧩",
		},
		{
			name:         "Unknown",
			subjectType:  SubjectType("unknown"),
			expectedIcon: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evidenceSummary.formatSubjectType(tt.subjectType)
			assert.Equal(t, tt.expectedIcon, result)
		})
	}
}

func TestFormatVerificationStatus(t *testing.T) {
	evidenceSummary := &EvidenceSummary{}

	tests := []struct {
		name     string
		verified bool
		expected string
	}{
		{
			name:     "Verified",
			verified: true,
			expected: "✅ Verified",
		},
		{
			name:     "Not Verified",
			verified: false,
			expected: "❌ Not Verified",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evidenceSummary.formatVerificationStatus(tt.verified)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatEvidenceType(t *testing.T) {
	evidenceSummary := &EvidenceSummary{}

	tests := []struct {
		name         string
		evidence     EvidenceSummaryData
		expectedType string
	}{
		{
			name: "With PredicateSlug",
			evidence: EvidenceSummaryData{
				PredicateSlug: "in-toto",
				PredicateType: "in-toto",
			},
			expectedType: "in-toto",
		},
		{
			name: "Without PredicateSlug but with PredicateType",
			evidence: EvidenceSummaryData{
				PredicateSlug: "",
				PredicateType: "https://slsa.dev/provenance/v0.2",
			},
			expectedType: "https://slsa.dev/provenance/v0.2",
		},
		{
			name: "Without PredicateSlug and PredicateType",
			evidence: EvidenceSummaryData{
				PredicateSlug: "",
				PredicateType: "",
			},
			expectedType: "⚠️ Unknown",
		},
		{
			name: "PredicateSlug with HTML",
			evidence: EvidenceSummaryData{
				PredicateSlug: `</td></tr><tr><td><a href="https://attacker.example">x</a>`,
			},
			expectedType: "&lt;/td&gt;&lt;/tr&gt;&lt;tr&gt;&lt;td&gt;&lt;a href=&#34;https://attacker.example&#34;&gt;x&lt;/a&gt;",
		},
		{
			name: "PredicateType with HTML",
			evidence: EvidenceSummaryData{
				PredicateType: `https://slsa.dev/provenance/v1"><img src=x>`,
			},
			expectedType: "https://slsa.dev/provenance/v1&#34;&gt;&lt;img src=x&gt;",
		},
		{
			name: "PredicateSlug with line breaks",
			evidence: EvidenceSummaryData{
				PredicateSlug: "in-toto\r\n\n# heading",
			},
			expectedType: "in-toto   # heading",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evidenceSummary.formatEvidenceType(tt.evidence)
			assert.Equal(t, tt.expectedType, result)
		})
	}
}

func TestFormatSubjectWithLink(t *testing.T) {
	evidenceSummary, cleanUp := prepareEvidenceTest(t)
	defer func() {
		cleanUp()
	}()

	const evidenceUrl = "https://myplatform.com/ui/repos/tree/Evidence/repo/file.txt?clearFilter=true&amp;gh_job_id=JFrog+CLI+Core+Tests&amp;gh_section=evidence&amp;m=3&amp;s=1"

	tests := []struct {
		name        string
		platformUrl string
		evidence    EvidenceSummaryData
		expected    string
	}{
		{
			name:        "Benign display name",
			platformUrl: testPlatformUrl,
			evidence: EvidenceSummaryData{
				Subject:     "repo/file.txt",
				DisplayName: "file.txt",
				SubjectType: SubjectTypeArtifact,
			},
			expected: `📄 <a href="` + evidenceUrl + `">file.txt</a>`,
		},
		{
			name:        "Display name with HTML",
			platformUrl: testPlatformUrl,
			evidence: EvidenceSummaryData{
				Subject:     "repo/file.txt",
				DisplayName: `file"<img src=x>`,
				SubjectType: SubjectTypeArtifact,
			},
			expected: `📄 <a href="` + evidenceUrl + `">file&#34;&lt;img src=x&gt;</a>`,
		},
		{
			name:        "Display name with line breaks",
			platformUrl: testPlatformUrl,
			evidence: EvidenceSummaryData{
				Subject:     "repo/file.txt",
				DisplayName: "file.txt\n\n# heading",
				SubjectType: SubjectTypeArtifact,
			},
			expected: `📄 <a href="` + evidenceUrl + `">file.txt  # heading</a>`,
		},
		{
			name:        "Non HTTP evidence URL",
			platformUrl: "javascript:alert(1)//",
			evidence: EvidenceSummaryData{
				Subject:     "repo/file.txt",
				SubjectType: SubjectTypeArtifact,
			},
			expected: "📄 repo/file.txt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			StaticMarkdownConfig.setPlatformUrl(tt.platformUrl)
			defer StaticMarkdownConfig.setPlatformUrl(testPlatformUrl)
			result := evidenceSummary.formatSubjectWithLink(tt.evidence)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEvidenceTableEscapesValues(t *testing.T) {
	evidenceSummary, cleanUp := prepareEvidenceTest(t)
	defer func() {
		cleanUp()
	}()

	var evidenceData = []EvidenceSummaryData{
		{
			Subject:       "repo/file.txt",
			PredicateSlug: `</td></tr><tr><td><a href="https://attacker.example">x</a>`,
			DisplayName:   `file"<img src=x>`,
			SubjectType:   SubjectTypeArtifact,
		},
		{
			Subject:       "repo/file.txt",
			PredicateSlug: "in-toto\n\n# heading",
			SubjectType:   SubjectTypeArtifact,
		},
	}

	res := evidenceSummary.generateEvidenceTable(evidenceData)
	// The header row and one row per evidence
	assert.Equal(t, 3, strings.Count(res, "<tr>"))
	assert.Equal(t, 2, strings.Count(res, "<a "))
	assert.NotContains(t, res, "attacker.example\"")
	assert.NotContains(t, res, "<img")
	assert.NotContains(t, res, "\n\n")
}
