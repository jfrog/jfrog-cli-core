package utils

import (
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	xrayutils "github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/http/jfroghttpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/jfrog/jfrog-client-go/utils/io/httputils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"net/http"
	"os"
	"path"
	"path/filepath"
)

const (
	ChecksumFileName = "checksum.sha2"
)

// Download the relevant build-info-extractor jar.
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
func DownloadAnalyzerManagerIfNeeded() error {
	downloadPath, err := xrayutils.GetAnalyzerManagerDownloadPath()
	if err != nil {
		return err
	}
	artDetails, remotePath, err := getAnalyzerManagerRemoteDetails(downloadPath)
	if err != nil {
		return err
	}
	// Check if the AnalyzerManager should be downloaded.
	// First get the latest AnalyzerManager checksum from Artifactory.
	client, httpClientDetails, err := createHttpClient(artDetails)
	if err != nil {
		return err
	}
	downloadUrl := artDetails.ArtifactoryUrl + remotePath
	remoteFileDetails, _, err := client.GetRemoteFileDetails(downloadUrl, &httpClientDetails)
	if err != nil {
		return err
	}
	analyzerManagerDir, err := xrayutils.GetAnalyzerManagerDirAbsolutePath()
	if err != nil {
		return err
	}
	// Find current AnalyzerManager checksum.
	checksumFilePath := filepath.Join(analyzerManagerDir, ChecksumFileName)
	exist, err := fileutils.IsFileExists(checksumFilePath, false)
	if err != nil {
		return err
	}
	if exist {
		sha2, err := fileutils.ReadFile(checksumFilePath)
		if err != nil {
			return err
		}
		// If the checksums are identical, there's no need to download.
		if remoteFileDetails.Checksum.Sha256 == string(sha2) {
			return nil
		}
	}
	// Download & unzip the analyzer manager files
	log.Debug("The 'Analyzer Manager' app is not cached locally. Downloading it now...")
	if err = DownloadDependency(artDetails, remotePath, filepath.Join(analyzerManagerDir, xrayutils.AnalyzerManagerZipName), true); err != nil {
		return err
	}
	// Add permission for all unzipped files
	filesList, err := fileutils.ListFilesRecursiveWalkIntoDirSymlink(analyzerManagerDir, false)
	if err != nil {
		return err
	}
	for _, file := range filesList {
		if err = os.Chmod(file, 0777); err != nil {
			return errorutils.CheckError(err)
		}
	}

	return createChecksumFile(checksumFilePath, remoteFileDetails.Checksum.Sha256)
}

func createChecksumFile(targetPath, checksum string) (err error) {
	out, err := os.Create(targetPath)
	defer func() {
		e := errorutils.CheckError(out.Close())
		err = errors.Join(err, e)
	}()
	if errorutils.CheckError(err) != nil {
		return err
	}
	if _, err = out.Write([]byte(checksum)); err != nil {
		return errorutils.CheckError(err)
	}
	return
}

// GetExtractorsRemoteDetails retrieves the server details necessary to download the build-info extractors from a remote repository.
// downloadPath - specifies the path in the remote repository from which the extractors will be downloaded.
func GetExtractorsRemoteDetails(downloadPath string) (server *config.ServerDetails, remoteRepo string, err error) {
	// Download from the remote repository that proxies https://releases.jfrog.io
	server, remoteRepo, err = getExtractorsRemoteDetailsFromEnv(downloadPath)
	if remoteRepo == "" && err == nil {
		// Fallback to the deprecated JFROG_CLI_EXTRACTORS_REMOTE environment variable
		server, remoteRepo, err = getExtractorsRemoteDetailsFromLegacyEnv(downloadPath)
	}
	if remoteRepo != "" || err != nil {
		return
	}
	// Download directly from https://releases.jfrog.io
	log.Info("The build-info-extractor jar is not cached locally. Downloading it now...\n" +
		"You can set the repository from which this jar is downloaded.\n" +
		"Read more about it at " + coreutils.JFrogHelpUrl + "jfrog-cli/downloading-the-maven-and-gradle-extractor-jars")
	log.Debug("'" + coreutils.ReleasesRemoteEnv + "' environment variable is not configured. Downloading directly from releases.jfrog.io.")
	// If not configured to download through a remote repository in Artifactory, download from releases.jfrog.io.
	return &config.ServerDetails{ArtifactoryUrl: coreutils.JfrogReleasesUrl}, path.Join("oss-release-local", downloadPath), nil
}

func getExtractorsRemoteDetailsFromEnv(downloadPath string) (server *config.ServerDetails, remoteRepo string, err error) {
	server, remoteRepo, err = getRemoteDetails(coreutils.ReleasesRemoteEnv)
	if remoteRepo != "" && err == nil {
		remoteRepo = getFullExtractorsPathInArtifactory(remoteRepo, coreutils.ReleasesRemoteEnv, downloadPath)
	}
	return
}

func getExtractorsRemoteDetailsFromLegacyEnv(downloadPath string) (server *config.ServerDetails, remoteRepo string, err error) {
	server, remoteRepo, err = getRemoteDetails(coreutils.DeprecatedExtractorsRemoteEnv)
	if remoteRepo != "" && err == nil {
		log.Warn(fmt.Sprintf("You are using the deprecated %q environment variable. Use %q instead.\nRead more about it at %sjfrog-cli/downloading-the-maven-and-gradle-extractor-jars",
			coreutils.DeprecatedExtractorsRemoteEnv, coreutils.ReleasesRemoteEnv, coreutils.JFrogHelpUrl))
		remoteRepo = getFullExtractorsPathInArtifactory(remoteRepo, coreutils.DeprecatedExtractorsRemoteEnv, downloadPath)
	}
	return
}

// getRemoteDetails function retrieves the server details and downloads path for the build-info extractor file.
// serverAndRepo - the server id and the remote repository that proxies releases.jfrog.io, in form of '<ServerID>/<RemoteRepo>'.
// downloadPath - specifies the path in the remote repository from which the extractors will be downloaded.
// remoteEnv - the relevant environment variable that was used: releasesRemoteEnv/ExtractorsRemoteEnv.
// The function returns the server that matches the given server ID, the complete path of the build-info extractor concatenated with the specified remote repository, and an error if occurred.
func getRemoteDetails(remoteEnv string) (server *config.ServerDetails, repoName string, err error) {
	serverID, repoName, err := coreutils.GetServerIdAndRepo(remoteEnv)
	if err != nil {
		return
	}
	if serverID == "" && repoName == "" {
		// Remote details weren't configured. Assuming that https://releases.jfrog.io should be used.
		return
	}
	server, err = config.GetSpecificConfig(serverID, false, true)
	return
}

func getFullExtractorsPathInArtifactory(repoName, remoteEnv, downloadPath string) string {
	if remoteEnv == coreutils.ReleasesRemoteEnv {
		return path.Join(repoName, "artifactory", "oss-release-local", downloadPath)
	}
	return path.Join(repoName, downloadPath)
}

// Downloads the requested resource.
//
// artDetails: The artifactory server details to download the resource from.
// downloadPath: Artifactory download path.
// targetPath: The local download path (without the file name).
func DownloadDependency(artDetails *config.ServerDetails, downloadPath, targetPath string, shouldExplode bool) (err error) {
	downloadUrl := artDetails.ArtifactoryUrl + downloadPath
	log.Info("Downloading JFrog's Dependency from", downloadUrl)
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
	expectedSha1 := ""
	remoteFileDetails, _, err := client.GetRemoteFileDetails(downloadUrl, &httpClientDetails)
	if err == nil {
		expectedSha1 = remoteFileDetails.Checksum.Sha1
	} else {
		log.Warn(fmt.Sprintf("Failed to get remote file details.\n Got: %s", err))
	}
	// Download the file
	downloadFileDetails := &httpclient.DownloadFileDetails{
		FileName:      filename,
		DownloadPath:  downloadUrl,
		LocalPath:     tempDirPath,
		LocalFileName: filename,
		ExpectedSha1:  expectedSha1,
	}
	client, httpClientDetails, err = createHttpClient(artDetails)
	if err != nil {
		return err
	}
	resp, err := client.DownloadFile(downloadFileDetails, "", &httpClientDetails, shouldExplode, false)
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
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return
	}

	httpClientDetails = auth.CreateHttpClientDetails()
	rtHttpClient, err = jfroghttpclient.JfrogClientBuilder().
		SetCertificatesPath(certsPath).
		SetInsecureTls(artDetails.InsecureTls).
		SetClientCertPath(auth.GetClientCertPath()).
		SetClientCertKeyPath(auth.GetClientCertKeyPath()).
		AppendPreRequestInterceptor(auth.RunPreRequestFunctions).
		Build()
	return
}

func getAnalyzerManagerRemoteDetails(downloadPath string) (server *config.ServerDetails, fullRemotePath string, err error) {
	var remoteRepo string
	server, remoteRepo, err = getRemoteDetails(coreutils.ReleasesRemoteEnv)
	if err != nil {
		return
	}
	if remoteRepo != "" {
		fullRemotePath = path.Join(remoteRepo, "artifactory", downloadPath)
		return
	}
	log.Debug("'" + coreutils.ReleasesRemoteEnv + "' environment variable is not configured. The Analyzer Manager app will be downloaded directly from releases.jfrog.io if needed.")
	// If not configured to download through a remote repository in Artifactory, download from releases.jfrog.io.
	return &config.ServerDetails{ArtifactoryUrl: coreutils.JfrogReleasesUrl}, downloadPath, nil
}
