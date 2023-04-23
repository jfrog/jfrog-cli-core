package utils

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sync"

	"net/http"
	"path"

	"github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/http/jfroghttpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	// Deprecated, use DependenciesRemoteEnv instead.
	LegacyExtractorsRemoteEnv = "JFROG_CLI_EXTRACTORS_REMOTE"

	// This env var should be used for downloading the CLI's dependencies (extractor jars, analyzerManager and etc.) through an Artifactory remote
	// repository, instead of downloading directly from releases.jfrog.io. The remote repository should be
	// configured to proxy releases.jfrog.io.
	// This env var should store a server ID and a remote repository in form of '<ServerID>/<RemoteRepo>'
	DependenciesRemoteEnv = "JFROG_CLI_DEPENDENCIES_REPO"

	// TODO: Analyzer manager consts - should move to new analyzermanger.go file
	analyzerManagerDownloadPath = ""
	analyzerManagerDir          = ""
	analyzerManagerZipName      = ""
)

// Download the relevant build-info-extractor jar if it does not already exist locally.
// By default, the jar is downloaded directly from jfrog releases.
//
// targetPath: The local download path (without the file name).
// downloadPath: Artifactory download path.
func DownloadExtractor(targetPath, downloadPath string) error {
	artDetails, remotePath, err := GetExtractorsRemoteDetails(downloadPath)
	if err != nil {
		return err
	}

	return DownloadDependency(artDetails, remotePath, targetPath, false)
}

// Download the latest AnalyzerManager executable if not cached locally.
// By default, the zip is downloaded directly from jfrog releases.
//
// mutex: optional  object for background download support
func DownloadAnalyzerManagerIfNeeded(mutex *sync.Mutex) error {
	if mutex != nil {
		mutex.Lock()
		defer mutex.Unlock()
	}
	artDetails, remotePath, err := GetAnalyzerManagerRemoteDetails(analyzerManagerDownloadPath)
	if err != nil {
		return err
	}
	// Check if the AnalyzerManager should be downloaded.
	// First get the latest AnalyzerManager checksum from Artifactory.
	client, httpClientDetails, err := createHttpClient(artDetails)
	if err != nil {
		return err
	}
	remoteFileDetails, _, err := client.GetRemoteFileDetails(analyzerManagerDownloadPath, &httpClientDetails)
	if err != nil {
		return err
	}
	// Calc current AnalyzerManager checksum.
	_, _, sha2, err := utils.GetFileChecksums(filepath.Join(analyzerManagerDir, analyzerManagerZipName))
	if err != nil {
		return err
	}
	// If ident, no need to download.
	if remoteFileDetails.Checksum.Sha256 == sha2 {
		return nil
	}

	// Download & unzip the analyzer manager files
	log.Info("The JFrog Analyzer manager zip is not cached locally. Downloading it now...")
	return DownloadDependency(artDetails, remotePath, analyzerManagerDir, true)
}

// The GetExtractorsRemoteDetails function is responsible for retrieving the server details necessary to download the build-info extractors.
// downloadPath - specifies the path in the remote repository from which the extractors will be downloaded.
func GetExtractorsRemoteDetails(downloadPath string) (server *config.ServerDetails, remoteRepo string, err error) {
	server, remoteRepo, err = getRemoteDetailsFromEnv(downloadPath)
	if remoteRepo != "" || err != nil {
		return
	}

	log.Info("The build-info-extractor jar is not cached locally. Downloading it now...\n" +
		"You can set the repository from which this jar is downloaded.\n" +
		"Read more about it at " + coreutils.JFrogHelpUrl + "jfrog-cli/downloading-the-maven-and-gradle-extractor-jars")
	log.Debug("'" + coreutils.ReleasesRemoteEnv + "' environment variable is not configured. Downloading directly from releases.jfrog.io.")
	// If not configured to download through a remote repository in Artifactory, download from releases.jfrog.io.
	return &config.ServerDetails{ArtifactoryUrl: "https://releases.jfrog.io/artifactory/"}, path.Join("oss-release-local", downloadPath), nil
}

func getRemoteDetailsFromEnv(downloadPath string) (server *config.ServerDetails, remoteRepo string, err error) {
	server, remoteRepo, err = getRemoteDetails(downloadPath, coreutils.ReleasesRemoteEnv)
	if remoteRepo != "" || err != nil {
		return
	}
	// Fallback to the deprecated JFROG_CLI_EXTRACTORS_REMOTE environment variable
	server, remoteRepo, err = getRemoteDetails(downloadPath, coreutils.ExtractorsRemoteEnv)
	return
}

// getRemoteDetails function retrieves the server details and downloads path for the build-info extractor file.
// serverAndRepo - the server id and the remote repository that proxies releases.jfrog.io, in form of '<ServerID>/<RemoteRepo>'.
// downloadPath - specifies the path in the remote repository from which the extractors will be downloaded.
// remoteEnv - the relevant environment variable that was used: releasesRemoteEnv/ExtractorsRemoteEnv.
// The function returns the server that matches xthe given server ID, the complete path of the build-info extractor concatenated with the specified remote repository, and an error if occurred.
func getRemoteDetails(downloadPath, remoteEnv string) (server *config.ServerDetails, fullRemoteRepoPath string, err error) {
	serverID, repoName, err := coreutils.GetServerIdAndRepo(remoteEnv)
	if err != nil {
		return
	}
	if serverID == "" && repoName == "" {
		// Remote details weren't configured. Assuming that https://releases.jfro.io should be used.
		return
	}
	server, err = config.GetSpecificConfig(serverID, false, true)
	if err != nil {
		return
	}
	fullRemoteRepoPath = getFullExtractorsPathInArtifactory(repoName, remoteEnv, downloadPath)
	return
}

func getFullExtractorsPathInArtifactory(repoName, remoteEnv, downloadPath string) string {
	if remoteEnv == coreutils.ReleasesRemoteEnv {
		return path.Join(repoName, "artifactory", "oss-release-local", downloadPath)
	}
	return path.Join(repoName, downloadPath)
}

func DownloadDependency(artDetails *config.ServerDetails, downloadPath, targetPath string, shouldExplode bool) (err error) {
	downloadUrl := fmt.Sprintf("%s%s", artDetails.ArtifactoryUrl, downloadPath)
	log.Info("Downloading JFrog's Dependency from ", downloadUrl)
	filename, localDir := fileutils.GetFileAndDirFromPath(targetPath)
	tempDirPath, err := fileutils.CreateTempDir()
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Join(err, fileutils.RemoveTempDir(tempDirPath))
	}()

	// Get the expected check-sum before downloading
	client, httpClientDetails, err := createHttpClient(artDetails)
	if err != nil {
		return err
	}
	remoteFileDetails, _, err := client.GetRemoteFileDetails(downloadUrl, &httpClientDetails)
	if err != nil {
		return err
	}
	// Download the file
	downloadFileDetails := &httpclient.DownloadFileDetails{
		FileName:      filename,
		DownloadPath:  downloadUrl,
		LocalPath:     tempDirPath,
		LocalFileName: filename,
		ExpectedSha1:  remoteFileDetails.Checksum.Sha1,
	}
	client, httpClientDetails, err = createHttpClient(artDetails)
	if err != nil {
		return err
	}
	resp, err := client.DownloadFile(downloadFileDetails, "", &httpClientDetails, shouldExplode)
	if err == nil && resp.StatusCode != http.StatusOK {
		err = errorutils.CheckErrorf(resp.Status + " received when attempting to download " + downloadUrl)
	}
	if err != nil {
		return err
	}
	return fileutils.CopyDir(tempDirPath, localDir, true, nil)
}

func createHttpClient(artDetails *config.ServerDetails) (rtHttpClient *jfroghttpclient.JfrogHttpClient, httpClientDetails httputils.HttpClientDetails, err error) {
	auth, err := artDetails.CreateArtAuthConfig()
	if err != nil {
		return
	}
	httpClientDetails = auth.CreateHttpClientDetails()

	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return
	}

	rtHttpClient, err = jfroghttpclient.JfrogClientBuilder().
		SetCertificatesPath(certsPath).
		SetInsecureTls(artDetails.InsecureTls).
		SetClientCertPath(auth.GetClientCertPath()).
		SetClientCertKeyPath(auth.GetClientCertKeyPath()).
		AppendPreRequestInterceptor(auth.RunPreRequestFunctions).
		Build()
	if err != nil {
		return err
	}

	httpClientDetails := auth.CreateHttpClientDetails()
	resp, err := client.DownloadFile(downloadFileDetails, "", &httpClientDetails, false, false)
	if err == nil && resp.StatusCode != http.StatusOK {
		err = errorutils.CheckErrorf(resp.Status + " received when attempting to download " + downloadUrl)
	}

	return err
}

func GetAnalyzerManagerRemoteDetails(downloadPath string) (*config.ServerDetails, string, error) {
	extractorsRemote := os.Getenv(DependenciesRemoteEnv)
	if extractorsRemote != "" {
		return getDependenciesRemoteRepo(extractorsRemote, path.Join("artifactory", downloadPath))
	}
	log.Debug("'" + DependenciesRemoteEnv + "' environment variable is not configured. JFrog analyzer manager will be downloaded directly from releases.jfrog.io if needed.")
	return &config.ServerDetails{ArtifactoryUrl: "https://releases.jfrog.io/artifactory/"}, downloadPath, nil
}
