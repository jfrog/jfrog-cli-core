package commandsummary

import (
	"fmt"
	"net/url"
)

const (
	releaseBundleEvidenceFormat = "%sui/artifactory/lifecycle?range=Any+Time&bundleName=%s&repositoryKey=%s&releaseBundleVersion=%s&activeVersionTab=Evidence+Graph"
	buildEvidenceFormat         = "%sui/builds/%s/%s/%s/Evidence/%s?buildRepo=%s"
	artifactEvidenceFormat      = "%sui/repos/tree/Evidence/%s?clearFilter=true"
)

func GenerateEvidenceUrlByType(data EvidenceSummaryData, section summarySection) (string, error) {
	switch data.SubjectType {
	// Currently, it is not possible to generate a link to the evidence tab for packages in the Artifactory UI.
	// The link will point to the lead artifact of the package instead.
	// This logic will be updated once UI support is available
	case SubjectTypePackage, SubjectTypeArtifact:
		return generateArtifactEvidenceUrl(data.Subject, section)
	case SubjectTypeReleaseBundle:
		return generateReleaseBundleEvidenceUrl(data, section)
	case SubjectTypeBuild:
		return generateBuildEvidenceUrl(data, section)
	default:
		return generateArtifactEvidenceUrl(data.Subject, section)
	}
}

func generateArtifactEvidenceUrl(pathInRt string, section summarySection) (string, error) {
	urlStr := fmt.Sprintf(artifactEvidenceFormat, StaticMarkdownConfig.GetPlatformUrl(), pathInRt)
	return addGitHubTrackingToUrl(urlStr, section)
}

func generateReleaseBundleEvidenceUrl(data EvidenceSummaryData, section summarySection) (string, error) {
	if data.ReleaseBundleName == "" || data.ReleaseBundleVersion == "" {
		return generateArtifactEvidenceUrl(data.Subject, section)
	}

	urlStr := fmt.Sprintf(releaseBundleEvidenceFormat,
		StaticMarkdownConfig.GetPlatformUrl(),
		data.ReleaseBundleName,
		data.RepoKey,
		data.ReleaseBundleVersion)

	return addGitHubTrackingToUrl(urlStr, section)
}

func generateBuildEvidenceUrl(data EvidenceSummaryData, section summarySection) (string, error) {
	if data.BuildName == "" || data.BuildNumber == "" || data.BuildTimestamp == "" {
		return generateArtifactEvidenceUrl(data.Subject, section)
	}

	urlStr := fmt.Sprintf(buildEvidenceFormat,
		StaticMarkdownConfig.GetPlatformUrl(),
		url.QueryEscape(data.BuildName),
		data.BuildNumber,
		data.BuildTimestamp,
		url.QueryEscape(data.BuildName),
		data.RepoKey)

	return addGitHubTrackingToUrl(urlStr, section)
}
