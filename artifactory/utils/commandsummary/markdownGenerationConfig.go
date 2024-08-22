package commandsummary

import (
	"encoding/json"
	"fmt"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"net/http"
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
	entitled, err := checkExtendedSummaryEntitled(serverUrl)
	if err != nil {
		return
	}
	setExtendedSummary(entitled)
	setPlatformMajorVersion(platformMajorVersion)
	setPlatformUrl(serverUrl)
	return
}

func checkExtendedSummaryEntitled(serverUrl string) (entitled bool, err error) {
	url := fmt.Sprintf("%sui/api/v1/system/auth/screen/footer", serverUrl)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("Error making HTTP request:", err)
		return
	}
	defer func() {
		err = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Non-OK HTTP status:", resp.StatusCode)
		return
	}

	var result struct {
		PlatformId string `json:"platformId"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Println("Error decoding JSON response:", err)
		return false, err
	}
	entitled = strings.Contains(strings.ToLower(result.PlatformId), "enterprise")
	return
}
