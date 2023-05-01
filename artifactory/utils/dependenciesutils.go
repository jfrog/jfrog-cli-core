package utils

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/http/jfroghttpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"net/http"
	"path"
)

// DownloadExtractorIfNeeded Downloads the relevant build-info-extractor jar if it does not already exist locally.
// By default, the jar is downloaded directly from jfrog releases.
// downloadPath: Artifactory download path.
// targetPath: The local download path (without the file name).
func DownloadExtractorIfNeeded(targetPath, downloadPath string) error {
	artDetails, remotePath, err := GetExtractorsRemoteDetails(downloadPath)
	if err != nil {
		return err
	}

	return DownloadExtractor(artDetails, remotePath, targetPath)
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

func DownloadExtractor(artDetails *config.ServerDetails, downloadPath, targetPath string) error {
	downloadUrl := fmt.Sprintf("%s%s", artDetails.ArtifactoryUrl, downloadPath)
	log.Info("Downloading build-info-extractor from", downloadUrl)
	filename, localDir := fileutils.GetFileAndDirFromPath(targetPath)

	downloadFileDetails := &httpclient.DownloadFileDetails{
		FileName:      filename,
		DownloadPath:  downloadUrl,
		LocalPath:     localDir,
		LocalFileName: filename,
	}

	auth, err := artDetails.CreateArtAuthConfig()
	if err != nil {
		return err
	}
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return err
	}

	client, err := jfroghttpclient.JfrogClientBuilder().
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
