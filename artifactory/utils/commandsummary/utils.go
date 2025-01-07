package commandsummary

import (
	"crypto/sha1" // #nosec G505 - This is only used for encoding, not security.
	"encoding/hex"
	"fmt"
	"net/url"
	"os"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

const (
	artifactory7UiFormat              = "%sui/repos/tree/General/%s?clearFilter=true"
	artifactory6UiFormat              = "%sartifactory/webapp/#/artifacts/browse/tree/General/%s"
	artifactoryDockerPackagesUiFormat = "%s/ui/packages/docker:%s/sha256__%s"
)

func GenerateArtifactUrl(pathInRt string, section summarySection) (url string, err error) {
	if StaticMarkdownConfig.GetPlatformMajorVersion() == 6 {
		url = fmt.Sprintf(artifactory6UiFormat, StaticMarkdownConfig.GetPlatformUrl(), pathInRt)
	} else {
		url = fmt.Sprintf(artifactory7UiFormat, StaticMarkdownConfig.GetPlatformUrl(), pathInRt)
	}
	url, err = addGitHubTrackingToUrl(url, section)
	return
}

func WrapCollapsableMarkdown(title, markdown string, headerSize int) string {
	return fmt.Sprintf("\n\n\n<details open>\n\n<summary> <h%d> %s </h%d></summary><p></p>\n\n%s\n\n</details>\n\n\n", headerSize, title, headerSize, markdown)
}

// Map containing indexed data recorded to the file system.
// The key is the index and the value is a map of file names as SHA1 to their full path.
type IndexedFilesMap map[Index]map[string]string

func fileNameToSha1(fileName string) string {
	hash := sha1.New() // #nosec G401 - This is only used for encoding, not security.
	hash.Write([]byte(fileName))
	hashBytes := hash.Sum(nil)
	return hex.EncodeToString(hashBytes)
}

type summarySection string

const (
	artifactsSection summarySection = "artifacts"
	packagesSection  summarySection = "packages"
	buildInfoSection summarySection = "buildInfo"
)

const (
	// The source of the request
	sourceParamKey    = "s"
	githubSourceValue = "1"
	// The metric to track
	metricParamKey    = "m"
	githubMetricValue = "3"

	jobIDKey       = "gh_job_id"
	sectionKey     = "gh_section"
	workflowEnvKey = "GITHUB_WORKFLOW"
)

func addGitHubTrackingToUrl(urlStr string, section summarySection) (string, error) {
	// Check if GITHUB_WORKFLOW environment variable is set
	githubWorkflow := os.Getenv(workflowEnvKey)
	if githubWorkflow == "" {
		return urlStr, nil
	}

	// Parse the input URL
	parsedUrl, err := url.Parse(urlStr)
	if errorutils.CheckError(err) != nil {
		return "", err
	}

	// Get the query parameters and add the GitHub tracking parameters
	queryParams := parsedUrl.Query()
	queryParams.Set(sourceParamKey, githubSourceValue)
	queryParams.Set(metricParamKey, githubMetricValue)
	queryParams.Set(jobIDKey, githubWorkflow)
	queryParams.Set(sectionKey, string(section))
	parsedUrl.RawQuery = queryParams.Encode()

	// Return the modified URL
	return parsedUrl.String(), nil
}
