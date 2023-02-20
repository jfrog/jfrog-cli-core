package offlineupdate

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	Vulnerability       = "__vuln"
	Component           = "__comp"
	JxrayDefaultBaseUrl = "https://jxray.jfrog.io/"
	JxrayApiBundles     = "api/v1/updates/bundles"
	JxrayApiOnboarding  = "api/v1/updates/onboarding"
	periodicState       = "periodic"
	onboardingState     = "onboarding"
)

func OfflineUpdate(flags *OfflineUpdatesFlags) error {
	if flags.IsDBSyncV3 {
		return handleDBSyncV3OfflineUpdate(flags)
	}
	return handleDBSyncV1OfflineUpdate(flags)
}

func handleDBSyncV1OfflineUpdate(flags *OfflineUpdatesFlags) error {
	updatesUrl, err := buildUpdatesUrl(flags)
	if err != nil {
		return err
	}
	vulnerabilities, components, lastUpdate, err := getFilesList(updatesUrl, flags)
	if err != nil {
		return err
	}
	zipSuffix := "_" + strconv.FormatInt(lastUpdate, 10)
	xrayTempDir, err := getXrayTempDir()
	if err != nil {
		return err
	}

	if flags.Target != "" && (len(vulnerabilities) > 0 || len(components) > 0) {
		err = os.MkdirAll(flags.Target, 0777)
		if errorutils.CheckError(err) != nil {
			return err
		}
	}

	if len(vulnerabilities) > 0 {
		log.Info("Downloading vulnerabilities...")
		err := saveData(xrayTempDir, "vuln", zipSuffix, flags.Target, vulnerabilities)
		if err != nil {
			return err
		}
	} else {
		log.Info("There are no new vulnerabilities.")
	}

	if len(components) > 0 {
		log.Info("Downloading components...")
		err := saveData(xrayTempDir, "comp", zipSuffix, flags.Target, components)
		if err != nil {
			return err
		}
	} else {
		log.Info("There are no new components.")
	}
	return nil
}

func getURLsToDownloadDBSyncV3(responseBody []byte, isPeriodicUpdate bool) ([]string, error) {
	var onboardingResponse OnboardingResponse
	var periodicResponse V3PeriodicUpdateResponse
	var urlsToDownload []string
	var err error
	if isPeriodicUpdate {
		err = json.Unmarshal(responseBody, &periodicResponse)
		if err != nil {
			return nil, errorutils.CheckError(err)
		}
		for _, packageUrl := range periodicResponse.Update {
			urlsToDownload = append(urlsToDownload, packageUrl.DownloadUrl)
		}
		for _, packageUrl := range periodicResponse.Deletion {
			urlsToDownload = append(urlsToDownload, packageUrl.DownloadUrl)
		}
	} else {
		err = json.Unmarshal(responseBody, &onboardingResponse)
		if err != nil {
			return nil, errorutils.CheckError(err)
		}
		for _, packageUrl := range onboardingResponse {
			urlsToDownload = append(urlsToDownload, packageUrl.DownloadUrl)
		}
	}
	return urlsToDownload, nil
}

func createV3MetadataFile(state string, body []byte, destFolder string) (err error) {
	metaDataFileName := state + ".json"
	metaDataFile := filepath.Join(destFolder, metaDataFileName)
	f, err := os.Create(metaDataFile)
	if err != nil {
		return errorutils.CheckError(err)
	}
	defer func() {
		if cerr := f.Close(); err != nil {
			err = cerr
		}
	}()
	_, err = f.Write(body)
	return errorutils.CheckError(err)
}

func handleDBSyncV3OfflineUpdate(flags *OfflineUpdatesFlags) (err error) {
	url := buildUrlDBSyncV3(flags.IsDBSyncV3PeriodicUpdate)
	log.Info("Getting updates...")
	headers := make(map[string]string)
	headers["X-Xray-License"] = flags.License
	httpClientDetails := httputils.HttpClientDetails{
		Headers: headers,
	}
	client, err := httpclient.ClientBuilder().SetRetries(3).Build()
	if err != nil {
		return err
	}
	resp, body, _, err := client.SendGet(url, false, httpClientDetails, "")
	if errorutils.CheckError(err) != nil {
		return err
	}
	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
		return err
	}

	urlsToDownload, err := getURLsToDownloadDBSyncV3(body, flags.IsDBSyncV3PeriodicUpdate)
	if err != nil {
		return err
	}

	var state string
	if flags.IsDBSyncV3PeriodicUpdate {
		state = periodicState
	} else {
		state = onboardingState
	}
	xrayTempDir, err := getXrayTempDir()
	if err != nil {
		return err
	}
	dataDir, err := os.MkdirTemp(xrayTempDir, "xray_downloaded_data")
	if err != nil {
		return err
	}
	defer func() {
		if cerr := fileutils.RemoveTempDir(dataDir); err == nil {
			err = cerr
		}
	}()
	err = downloadData(urlsToDownload, dataDir, createXrayFileNameFromUrlV3)
	if err != nil {
		return err
	}

	err = createV3MetadataFile(state, body, dataDir)
	if err != nil {
		return err
	}

	packageName := "xray_update_package" + "_" + state
	err = createZipArchive(dataDir, flags.Target, packageName, "")
	if err != nil {
		return err
	}
	return nil
}

func buildUrlDBSyncV3(isPeriodic bool) string {
	url := getJxRayBaseUrl() + "api/v3/updates/"
	if isPeriodic {
		return url + periodicState
	} else {
		return url + onboardingState
	}
}

func getJxRayBaseUrl() string {
	jxRayBaseUrl := os.Getenv("JFROG_CLI_JXRAY_BASE_URL")
	jxRayBaseUrl = utils.AddTrailingSlashIfNeeded(jxRayBaseUrl)
	if jxRayBaseUrl == "" {
		jxRayBaseUrl = JxrayDefaultBaseUrl
	}
	return jxRayBaseUrl
}

func getUpdatesBaseUrl(datesSpecified bool) string {
	jxRayBaseUrl := getJxRayBaseUrl()
	if datesSpecified {
		return jxRayBaseUrl + JxrayApiBundles
	}
	return jxRayBaseUrl + JxrayApiOnboarding
}

func buildUpdatesUrl(flags *OfflineUpdatesFlags) (string, error) {
	var queryParams string
	datesSpecified := flags.From > 0 && flags.To > 0
	if datesSpecified {
		if err := validateDates(flags.From, flags.To); err != nil {
			return "", err
		}
		queryParams += fmt.Sprintf("from=%v&to=%v", flags.From, flags.To)
	}
	if flags.Version != "" {
		if queryParams != "" {
			queryParams += "&"
		}
		queryParams += fmt.Sprintf("version=%v", flags.Version)
	}
	url := getUpdatesBaseUrl(datesSpecified)
	if queryParams != "" {
		url += "?" + queryParams
	}
	return url, nil
}

func validateDates(from, to int64) error {
	if from < 0 || to < 0 {
		err := errors.New("invalid dates")
		return errorutils.CheckError(err)
	}
	if from > to {
		err := errors.New("invalid dates range")
		return errorutils.CheckError(err)
	}
	return nil
}

func getXrayTempDir() (string, error) {
	xrayDir := filepath.Join(coreutils.GetCliPersistentTempDirPath(), "jfrog", "xray")
	if err := os.MkdirAll(xrayDir, 0777); err != nil {
		return "", errorutils.CheckError(err)
	}
	return xrayDir, nil
}

func downloadData(urlsList []string, dataDir string, fileNameFromUrlFunc func(string) (string, error)) error {
	for _, url := range urlsList {
		fileName, err := fileNameFromUrlFunc(url)
		if err != nil {
			return err
		}
		log.Info(fmt.Sprintf("Downloading updates package from %s", url))
		client, err := httpclient.ClientBuilder().SetRetries(3).Build()
		if err != nil {
			log.Error(fmt.Sprintf("Couldn't download from %s", url))
			return err
		}

		details := &httpclient.DownloadFileDetails{
			FileName:      fileName,
			DownloadPath:  url,
			LocalPath:     dataDir,
			LocalFileName: fileName}
		_, err = client.DownloadFile(details, "", httputils.HttpClientDetails{}, false)
		if err != nil {
			return errorutils.CheckErrorf("Couldn't download from %s. Error: %s", url, err.Error())
		}
	}
	log.Info("Download completed.")
	return nil
}

func createZipArchive(dataDir, targetPath, filesPrefix, zipSuffix string) error {
	log.Info("Zipping files.")
	err := fileutils.ZipFolderFiles(dataDir, filepath.Join(targetPath, filesPrefix+zipSuffix+".zip"))
	if err != nil {
		return err
	}
	log.Info("Done zipping files.")
	return nil
}

func saveData(xrayTmpDir, filesPrefix, zipSuffix, targetPath string, urlsList []string) (err error) {
	dataDir, err := os.MkdirTemp(xrayTmpDir, filesPrefix)
	if err != nil {
		return err
	}
	defer func() {
		if cerr := fileutils.RemoveTempDir(dataDir); cerr != nil && err == nil {
			err = cerr
		}
	}()
	err = downloadData(urlsList, dataDir, createXrayFileNameFromUrl)
	if err != nil {
		return err
	}
	err = createZipArchive(dataDir, targetPath, filesPrefix, zipSuffix)
	if err != nil {
		return err
	}
	return nil
}

func getUrlSections(url string) []string {
	index := strings.Index(url, "?")
	if index != -1 {
		url = url[:index]
	}
	index = strings.Index(url, ";")
	if index != -1 {
		url = url[:index]
	}
	return strings.Split(url, "/")
}

func createXrayFileNameFromUrlV3(url string) (string, error) {
	sections := getUrlSections(url)
	length := len(sections)
	return sections[length-1], nil
}

func createXrayFileNameFromUrl(url string) (fileName string, err error) {
	sections := getUrlSections(url)
	length := len(sections)
	if length < 2 {
		err = errorutils.CheckErrorf("Unexpected URL format: %s", url)
		return
	}
	fileName = fmt.Sprintf("%s__%s", sections[length-2], sections[length-1])
	return
}

func getFilesList(updatesUrl string, flags *OfflineUpdatesFlags) (vulnerabilities []string, components []string, lastUpdate int64, err error) {
	log.Info("Getting updates...")
	headers := make(map[string]string)
	headers["X-Xray-License"] = flags.License
	httpClientDetails := httputils.HttpClientDetails{
		Headers: headers,
	}
	client, err := httpclient.ClientBuilder().SetRetries(3).Build()
	if err != nil {
		return
	}
	resp, body, _, err := client.SendGet(updatesUrl, false, httpClientDetails, "")
	if errorutils.CheckError(err) != nil {
		return
	}
	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
		return
	}

	var urls FilesList
	err = json.Unmarshal(body, &urls)
	if err != nil {
		err = errorutils.CheckErrorf("Failed parsing json response: " + string(body))
		return
	}

	for _, v := range urls.Urls {
		if strings.Contains(v, Vulnerability) {
			vulnerabilities = append(vulnerabilities, v)
		} else if strings.Contains(v, Component) {
			components = append(components, v)
		}
	}
	lastUpdate = urls.LastUpdate
	return
}

type OfflineUpdatesFlags struct {
	License                  string
	From                     int64
	To                       int64
	Version                  string
	Target                   string
	IsDBSyncV3               bool
	IsDBSyncV3PeriodicUpdate bool
}

type FilesList struct {
	LastUpdate int64
	Urls       []string
}

type V3UpdateResponseItem struct {
	DownloadUrl string `json:"download_url"`
	Timestamp   int64  `json:"timestamp"`
}

type V3PeriodicUpdateResponse struct {
	Update   []V3UpdateResponseItem `json:"update"`
	Deletion []V3UpdateResponseItem `json:"deletion"`
}

type OnboardingResponse []V3UpdateResponseItem
