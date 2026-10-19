package commandsummary

import (
	"fmt"
	"html"
	"net/url"
	"strings"
	"time"

	"github.com/jfrog/jfrog-client-go/utils/log"
)

const evidenceHeaderSize = 3

type EvidenceSummaryData struct {
	Subject              string      `json:"subject"`
	SubjectSha256        string      `json:"subjectSha256"`
	PredicateType        string      `json:"predicateType"`
	PredicateSlug        string      `json:"predicateSlug"`
	Verified             bool        `json:"verified"`
	DisplayName          string      `json:"displayName,omitempty"`
	SubjectType          SubjectType `json:"subjectType"`
	BuildName            string      `json:"buildName"`
	BuildNumber          string      `json:"buildNumber"`
	BuildTimestamp       string      `json:"buildTimestamp"`
	ReleaseBundleName    string      `json:"releaseBundleName"`
	ReleaseBundleVersion string      `json:"releaseBundleVersion"`
	ApplicationKey       string      `json:"applicationKey"`
	ApplicationVersion   string      `json:"applicationVersion"`
	RepoKey              string      `json:"repoKey"`
	CreatedAt            time.Time   `json:"createdAt"`
}

type SubjectType string

const (
	SubjectTypeArtifact      SubjectType = "artifact"
	SubjectTypeBuild         SubjectType = "build"
	SubjectTypePackage       SubjectType = "package"
	SubjectTypeReleaseBundle SubjectType = "release-bundle"
	SubjectTypeApplication   SubjectType = "application"
)

type EvidenceSummary struct {
	CommandSummary
}

func NewEvidenceSummary() (*CommandSummary, error) {
	return New(&EvidenceSummary{}, "evidence")
}

func (es *EvidenceSummary) GetSummaryTitle() string {
	return "🔎 Evidence"
}

func (es *EvidenceSummary) GenerateMarkdownFromFiles(dataFilePaths []string) (finalMarkdown string, err error) {
	log.Debug("Generating evidence summary markdown.")
	var evidenceData []EvidenceSummaryData
	for _, filePath := range dataFilePaths {
		var evidence EvidenceSummaryData
		if err = UnmarshalFromFilePath(filePath, &evidence); err != nil {
			log.Warn("Failed to unmarshal evidence data from file %s: %v", filePath, err)
			return
		}
		evidenceData = append(evidenceData, evidence)
	}

	if len(evidenceData) == 0 {
		return
	}

	tableMarkdown := es.generateEvidenceTable(evidenceData)
	return WrapCollapsableMarkdown(es.GetSummaryTitle(), tableMarkdown, evidenceHeaderSize), nil
}

func (es *EvidenceSummary) generateEvidenceTable(evidenceData []EvidenceSummaryData) string {
	var tableBuilder strings.Builder
	tableBuilder.WriteString(es.getTableHeader())

	for _, evidence := range evidenceData {
		es.appendEvidenceRow(&tableBuilder, evidence)
	}

	tableBuilder.WriteString("</tbody></table> \n")
	return tableBuilder.String()
}

func (es *EvidenceSummary) getTableHeader() string {
	return "<table><thead><tr><th>Evidence Subject</th><th>Evidence Type</th><th>Verification Status</th></tr></thead><tbody>\n"
}

func (es *EvidenceSummary) appendEvidenceRow(tableBuilder *strings.Builder, evidence EvidenceSummaryData) {
	subject := es.formatSubjectWithLink(evidence)
	evidenceType := es.formatEvidenceType(evidence)
	verificationStatus := es.formatVerificationStatus(evidence.Verified)

	fmt.Fprintf(tableBuilder, "<tr><td>%s</td><td>%s</td><td>%s</td></tr>\n", subject, evidenceType, verificationStatus)
}

func (es *EvidenceSummary) formatSubjectWithLink(evidence EvidenceSummaryData) string {
	if evidence.Subject == "" {
		return "evidence"
	}

	evidenceUrl, err := GenerateEvidenceUrlByType(evidence, evidenceSection)
	if err != nil {
		log.Warn("Failed to generate evidence URL: %v", err)
		evidenceUrl = ""
	}

	displayName := evidence.DisplayName
	if displayName == "" {
		displayName = evidence.Subject
	}
	displayName = escapeHtmlValue(displayName)

	var viewLink string
	subjectType := es.formatSubjectType(evidence.SubjectType)
	if isHttpUrl(evidenceUrl) {
		viewLink = fmt.Sprintf(`%s <a href="%s">%s</a>`, subjectType, escapeHtmlValue(evidenceUrl), displayName)
	} else {
		viewLink = fmt.Sprintf("%s %s", subjectType, displayName)
	}

	return viewLink
}

func (es *EvidenceSummary) formatVerificationStatus(verified bool) string {
	if verified {
		return fmt.Sprintf("%s Verified", "✅")
	}
	return fmt.Sprintf("%s Not Verified", "❌")
}

func (es *EvidenceSummary) formatEvidenceType(evidence EvidenceSummaryData) string {
	if evidence.PredicateSlug == "" {
		if evidence.PredicateType == "" {
			return "⚠️ Unknown"
		}
		return escapeHtmlValue(evidence.PredicateType)
	}
	return escapeHtmlValue(evidence.PredicateSlug)
}

func (es *EvidenceSummary) formatSubjectType(subjectType SubjectType) string {
	switch subjectType {
	case SubjectTypePackage:
		return "📦️"
	case SubjectTypeBuild:
		return "🛠️️"
	case SubjectTypeReleaseBundle:
		return "🧩"
	case SubjectTypeArtifact:
		return "📄"
	case SubjectTypeApplication:
		return "📱"
	default:
		return ""
	}
}

// escapeHtmlValue makes a value safe to embed in the evidence HTML table.
// Line breaks are replaced as well, since a blank line ends the HTML block and the rest is rendered as Markdown.
func escapeHtmlValue(value string) string {
	return html.EscapeString(strings.NewReplacer("\r", " ", "\n", " ").Replace(value))
}

func isHttpUrl(rawUrl string) bool {
	parsedUrl, err := url.Parse(rawUrl)
	return err == nil && (parsedUrl.Scheme == "http" || parsedUrl.Scheme == "https")
}
