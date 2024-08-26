package commandsummary

import (
	"encoding/json"
	"fmt"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"net/http"
	"net/url"
	"strings"
)

// Static variables used in the context of command summaries that affect their Markdown generation

// Indicates if to generate a basic or extended summary
var extendedSummary bool

// The URL of the Artifactory instance
var platformUrl string

// The major version of the Artifactory instance which used to generate URLS.
var platformMajorVersion int

func setExtendedSummary(value bool) {
	extendedSummary = value
}

func setPlatformUrl(url string) {
	platformUrl = clientUtils.AddTrailingSlashIfNeeded(url)
}

func setPlatformMajorVersion(version int) {
	platformMajorVersion = version
}

func isExtendedSummary() bool {
	return extendedSummary
}

func GetPlatformUrl() string {
	return platformUrl
}

func GetPlatformMajorVersion() int {
	return platformMajorVersion
}

// Initializes the command summary values that effect Markdown generation
func InitMarkdownGenerationValues(serverUrl string, platformMajorVersion int) (err error) {
	_, err = checkExtendedSummaryEntitled(serverUrl)
	if err != nil {
		return
	}
	setExtendedSummary(false)
	setPlatformMajorVersion(platformMajorVersion)
	setPlatformUrl(serverUrl)
	return
}

func checkExtendedSummaryEntitled(serverUrl string) (bool, error) {
	// Parse and validate the URL
	parsedUrl, err := url.Parse(serverUrl)
	if err != nil || !parsedUrl.IsAbs() {
		return false, fmt.Errorf("invalid server URL: %s", serverUrl)
	}

	// Construct the full URL
	fullUrl := fmt.Sprintf("%sui/api/v1/system/auth/screen/footer", parsedUrl.String())
	// Suppress HTTP request security warning:
	// URL is validated, and the request is internal
	// #nosec G107
	resp, err := http.Get(fullUrl)
	if err != nil {
		fmt.Println("Error making HTTP request:", err)
		return false, err
	}
	defer func() {
		err = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Non-OK HTTP status:", resp.StatusCode)
		return false, nil
	}

	var result struct {
		PlatformId string `json:"platformId"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Println("Error decoding JSON response:", err)
		return false, err
	}
	entitled := strings.Contains(strings.ToLower(result.PlatformId), "enterprise")
	return entitled, nil
}
