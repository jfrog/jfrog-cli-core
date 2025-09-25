package commandsummary

import (
	"fmt"
	"net/url"
)

const (
	applicationEvidenceFormat   = "%sui/applications/management/%s/versions/%s?activeVersionTab=Content+Graph"
	releaseBundleEvidenceFormat = "%sui/artifactory/lifecycle?range=Any+Time&bundleName=%s&repositoryKey=%s&releaseBundleVersion=%s&activeVersionTab=Content+Graph"
	buildEvidenceFormat         = "%sui/builds/%s/%s/%s/Evidence?buildRepo=%s"
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
	case SubjectTypeApplication:
		return generateApplicationEvidenceUrl(data, section)
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

func generateApplicationEvidenceUrl(data EvidenceSummaryData, section summarySection) (string, error) {
	if data.ApplicationKey == "" || data.ApplicationVersion == "" {
		return generateArtifactEvidenceUrl(data.Subject, section)
	}

	urlStr := fmt.Sprintf(applicationEvidenceFormat,
		StaticMarkdownConfig.GetPlatformUrl(),
		data.ApplicationKey,
		data.ApplicationVersion)

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
		data.RepoKey)

	return addGitHubTrackingToUrl(urlStr, section)
}
