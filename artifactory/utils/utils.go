package utils

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/jfrog/build-info-go/build"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"

	"github.com/jfrog/jfrog-client-go/utils/io"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/access"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/auth"
	clientConfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/distribution"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

func GetProjectDir(global bool) (string, error) {
	configDir, err := getConfigDir(global)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return filepath.Join(configDir, "projects"), nil
}

func getConfigDir(global bool) (string, error) {
	if !global {
		wd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		return filepath.Join(wd, ".jfrog"), nil
	}
	return coreutils.GetJfrogHomeDir()
}

func GetEncryptedPasswordFromArtifactory(artifactoryAuth auth.ServiceDetails, insecureTls bool) (string, error) {
	u, err := url.Parse(artifactoryAuth.GetUrl())
	if err != nil {
		return "", err
	}
	u.Path = path.Join(u.Path, "api/security/encryptedPassword")
	httpClientsDetails := artifactoryAuth.CreateHttpClientDetails()
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return "", err
	}
	client, err := httpclient.ClientBuilder().
		SetCertificatesPath(certsPath).
		SetInsecureTls(insecureTls).
		SetClientCertPath(artifactoryAuth.GetClientCertPath()).
		SetClientCertKeyPath(artifactoryAuth.GetClientCertKeyPath()).
		Build()
	if err != nil {
		return "", err
	}
	resp, body, _, err := client.SendGet(u.String(), true, httpClientsDetails, "")
	if err != nil {
		return "", err
	}

	if resp.StatusCode == http.StatusOK {
		return string(body), nil
	}

	if resp.StatusCode == http.StatusConflict {
		message := "\nYour Artifactory server is not configured to encrypt passwords.\n" +
			"You may use \"art config --enc-password=false\""
		return "", errorutils.CheckErrorf(message)
	}

	return "", errorutils.CheckErrorf("Artifactory response: " + resp.Status + "\n" + clientutils.IndentJson(body))
}

func CreateServiceManager(serverDetails *config.ServerDetails, httpRetries, httpRetryWaitMilliSecs int, isDryRun bool) (artifactory.ArtifactoryServicesManager, error) {
	return CreateServiceManagerWithThreads(serverDetails, isDryRun, 0, httpRetries, httpRetryWaitMilliSecs)
}

// Create a service manager with threads.
// If the value sent for httpRetries is negative, the default will be used.
func CreateServiceManagerWithThreads(serverDetails *config.ServerDetails, isDryRun bool, threads, httpRetries, httpRetryWaitMilliSecs int) (artifactory.ArtifactoryServicesManager, error) {
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return nil, err
	}
	artAuth, err := serverDetails.CreateArtAuthConfig()
	if err != nil {
		return nil, err
	}
	config := clientConfig.NewConfigBuilder().
		SetServiceDetails(artAuth).
		SetCertificatesPath(certsPath).
		SetInsecureTls(serverDetails.InsecureTls).
		SetDryRun(isDryRun)
	if httpRetries >= 0 {
		config.SetHttpRetries(httpRetries)
		config.SetHttpRetryWaitMilliSecs(httpRetryWaitMilliSecs)
	}
	if threads > 0 {
		config.SetThreads(threads)
	}
	serviceConfig, err := config.Build()
	if err != nil {
		return nil, err
	}
	return artifactory.New(serviceConfig)
}

func CreateServiceManagerWithProgressBar(serverDetails *config.ServerDetails, threads, httpRetries, httpRetryWaitMilliSecs int, dryRun bool, progressBar io.ProgressMgr) (artifactory.ArtifactoryServicesManager, error) {
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return nil, err
	}
	artAuth, err := serverDetails.CreateArtAuthConfig()
	if err != nil {
		return nil, err
	}
	servicesConfig, err := clientConfig.NewConfigBuilder().
		SetServiceDetails(artAuth).
		SetDryRun(dryRun).
		SetCertificatesPath(certsPath).
		SetInsecureTls(serverDetails.InsecureTls).
		SetThreads(threads).
		SetHttpRetries(httpRetries).
		SetHttpRetryWaitMilliSecs(httpRetryWaitMilliSecs).
		Build()

	if err != nil {
		return nil, err
	}
	return artifactory.NewWithProgress(servicesConfig, progressBar)
}

func CreateDistributionServiceManager(serviceDetails *config.ServerDetails, isDryRun bool) (*distribution.DistributionServicesManager, error) {
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return nil, err
	}
	distAuth, err := serviceDetails.CreateDistAuthConfig()
	if err != nil {
		return nil, err
	}
	serviceConfig, err := clientConfig.NewConfigBuilder().
		SetServiceDetails(distAuth).
		SetCertificatesPath(certsPath).
		SetInsecureTls(serviceDetails.InsecureTls).
		SetDryRun(isDryRun).
		Build()
	if err != nil {
		return nil, err
	}
	return distribution.New(serviceConfig)
}

func CreateAccessServiceManager(serviceDetails *config.ServerDetails, isDryRun bool) (*access.AccessServicesManager, error) {
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return nil, err
	}
	accessAuth, err := serviceDetails.CreateAccessAuthConfig()
	if err != nil {
		return nil, err
	}
	serviceConfig, err := clientConfig.NewConfigBuilder().
		SetServiceDetails(accessAuth).
		SetCertificatesPath(certsPath).
		SetInsecureTls(serviceDetails.InsecureTls).
		SetDryRun(isDryRun).
		Build()
	if err != nil {
		return nil, err
	}
	return access.New(serviceConfig)
}

// This error indicates that the build was scanned by Xray, but Xray found issues with the build.
// If Xray failed to scan the build, for example due to a networking issue, a regular error should be returned.
var errBuildScan = errors.New("issues found during xray build scan")

func GetBuildScanError() error {
	return errBuildScan
}

// Download and unmarshal a file from artifactory.
func RemoteUnmarshal(serviceManager artifactory.ArtifactoryServicesManager, remoteFileUrl string, loadTarget interface{}) (err error) {
	ioReaderCloser, err := serviceManager.ReadRemoteFile(remoteFileUrl)
	if err != nil {
		return
	}
	defer func() {
		if localErr := ioReaderCloser.Close(); err == nil {
			err = localErr
		}
	}()
	content, err := ioutil.ReadAll(ioReaderCloser)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return errorutils.CheckError(json.Unmarshal(content, loadTarget))
}

func CreateBuildInfoService() *build.BuildInfoService {
	buildInfoService := build.NewBuildInfoService()
	buildInfoService.SetTempDirPath(filepath.Join(coreutils.GetCliPersistentTempDirPath(), BuildTempPath))
	buildInfoService.SetLogger(log.Logger)
	return buildInfoService
}

// Returns an error if the given repo doesn't exist.
func ValidateRepoExists(repoKey string, serviceDetails auth.ServiceDetails) error {
	servicesManager, err := createServiceManager(serviceDetails)
	if err != nil {
		return err
	}
	exists, err := servicesManager.IsRepoExists(repoKey)
	if err != nil {
		return err
	}

	if !exists {
		return errorutils.CheckErrorf("The repository '" + repoKey + "' does not exist.")
	}
	return nil
}

func createServiceManager(serviceDetails auth.ServiceDetails) (artifactory.ArtifactoryServicesManager, error) {
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return nil, err
	}
	config := clientConfig.NewConfigBuilder().
		SetServiceDetails(serviceDetails).
		SetCertificatesPath(certsPath).
		SetDryRun(false)
	serviceConfig, err := config.Build()
	if err != nil {
		return nil, err
	}
	return artifactory.New(serviceConfig)
}

func GetTestDataPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return filepath.Join(dir, "testdata"), nil
}
