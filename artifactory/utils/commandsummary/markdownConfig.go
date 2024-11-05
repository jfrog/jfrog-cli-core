package commandsummary

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/jfrog/jfrog-client-go/http/httpclient"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
)

// Static struct to hold the Markdown configuration values
type MarkdownConfig struct {
	// Indicates if to generate a basic or extended summary
	extendedSummary bool
	// Base platform URL
	platformUrl string
	// The major version of the Artifactory instance
	platformMajorVersion int
	// Static mapping of scan results to be used in the summary
	scanResultsMapping map[string]ScanResult
}

const extendedSummaryLandPage = "https://jfrog.com/help/access?xinfo:appid=csh-gen-gitbook"

var StaticMarkdownConfig = MarkdownConfig{}

func (mg *MarkdownConfig) setExtendedSummary(value bool) {
	mg.extendedSummary = value
}

func (mg *MarkdownConfig) setPlatformUrl(url string) {
	mg.platformUrl = clientUtils.AddTrailingSlashIfNeeded(url)
}

func (mg *MarkdownConfig) setPlatformMajorVersion(version int) {
	mg.platformMajorVersion = version
}

func (mg *MarkdownConfig) IsExtendedSummary() bool {
	return mg.extendedSummary
}

func (mg *MarkdownConfig) GetPlatformUrl() string {
	return mg.platformUrl
}

func (mg *MarkdownConfig) GetPlatformMajorVersion() int {
	return mg.platformMajorVersion
}

func (mg *MarkdownConfig) GetExtendedSummaryLangPage() string {
	return extendedSummaryLandPage
}

func (mg *MarkdownConfig) SetScanResultsMapping(resultsMap map[string]ScanResult) {
	mg.scanResultsMapping = resultsMap
}

// Initializes the command summary values that effect Markdown generation
func InitMarkdownGenerationValues(serverUrl string, platformMajorVersion int) (err error) {
	entitled, err := CheckExtendedSummaryEntitled(serverUrl)
	if err != nil {
		return
	}
	StaticMarkdownConfig.setExtendedSummary(entitled)
	StaticMarkdownConfig.setPlatformMajorVersion(platformMajorVersion)
	StaticMarkdownConfig.setPlatformUrl(serverUrl)
	return
}

func CheckExtendedSummaryEntitled(serverUrl string) (bool, error) {
	// Parse and validate the URL
	parsedUrl, err := url.Parse(serverUrl)
	if err != nil || !parsedUrl.IsAbs() {
		return false, fmt.Errorf("invalid server URL: %s", serverUrl)
	}

	// Construct the full URL
	fullUrl := fmt.Sprintf("%sui/api/v1/system/auth/screen/footer", parsedUrl.String())

	client, err := httpclient.ClientBuilder().SetRetries(3).Build()
	if err != nil {
		return false, errorutils.CheckError(err)
	}

	resp, body, _, err := client.SendGet(fullUrl, false, httputils.HttpClientDetails{}, "")
	if err != nil {
		fmt.Println("Error making HTTP request:", err)
		return false, err
	}

	if resp.StatusCode != http.StatusOK {
		fmt.Println("Non-OK HTTP status:", resp.StatusCode)
		return false, nil
	}

	var result struct {
		PlatformId string `json:"platformId"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return false, errorutils.CheckError(err)
	}
	entitled := strings.Contains(strings.ToLower(result.PlatformId), "enterprise")
	return entitled, nil
}
