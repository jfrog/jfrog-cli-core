package commandsummary

import (
	"os"
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
			os.Unsetenv(workflowEnvKey)
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evidenceSummary.formatEvidenceType(tt.evidence)
			assert.Equal(t, tt.expectedType, result)
		})
	}
}
