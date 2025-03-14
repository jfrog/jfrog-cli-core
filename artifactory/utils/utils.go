package utils

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	ioutils "github.com/jfrog/gofrog/io"
	"github.com/jfrog/jfrog-client-go/evidence"

	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/access"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/auth"
	clientConfig "github.com/jfrog/jfrog-client-go/config"
	"github.com/jfrog/jfrog-client-go/distribution"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	"github.com/jfrog/jfrog-client-go/jfconnect"
	"github.com/jfrog/jfrog-client-go/lifecycle"
	"github.com/jfrog/jfrog-client-go/metadata"
	clientUtils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	ioUtils "github.com/jfrog/jfrog-client-go/utils/io"
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

	return "", errorutils.CheckErrorf("Artifactory response: " + resp.Status + "\n" + clientUtils.IndentJson(body))
}

func CreateServiceManager(serverDetails *config.ServerDetails, httpRetries, httpRetryWaitMilliSecs int, isDryRun bool) (artifactory.ArtifactoryServicesManager, error) {
	return CreateServiceManagerWithTimeout(serverDetails, httpRetries, httpRetryWaitMilliSecs, isDryRun, 0)
}

func CreateServiceManagerWithTimeout(serverDetails *config.ServerDetails, httpRetries, httpRetryWaitMilliSecs int, isDryRun bool, timeout time.Duration) (artifactory.ArtifactoryServicesManager, error) {
	return CreateServiceManagerWithContext(context.Background(), serverDetails, isDryRun, 0, httpRetries, httpRetryWaitMilliSecs, timeout)
}

// Create a service manager with threads.
// If the value sent for httpRetries is negative, the default will be used.
func CreateServiceManagerWithThreads(serverDetails *config.ServerDetails, isDryRun bool, threads, httpRetries, httpRetryWaitMilliSecs int) (artifactory.ArtifactoryServicesManager, error) {
	return CreateServiceManagerWithContext(context.Background(), serverDetails, isDryRun, threads, httpRetries, httpRetryWaitMilliSecs, 0)
}

func CreateServiceManagerWithContext(context context.Context, serverDetails *config.ServerDetails, isDryRun bool, threads, httpRetries, httpRetryWaitMilliSecs int, timeout time.Duration) (artifactory.ArtifactoryServicesManager, error) {
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return nil, err
	}
	artAuth, err := serverDetails.CreateArtAuthConfig()
	if err != nil {
		return nil, err
	}
	configBuilder := clientConfig.NewConfigBuilder().
		SetServiceDetails(artAuth).
		SetCertificatesPath(certsPath).
		SetInsecureTls(serverDetails.InsecureTls).
		SetDryRun(isDryRun).
		SetContext(context)
	if httpRetries >= 0 {
		configBuilder.SetHttpRetries(httpRetries)
		configBuilder.SetHttpRetryWaitMilliSecs(httpRetryWaitMilliSecs)
	}
	if threads > 0 {
		configBuilder.SetThreads(threads)
	}
	if timeout > 0 {
		configBuilder.SetOverallRequestTimeout(timeout)
	}
	serviceConfig, err := configBuilder.Build()
	if err != nil {
		return nil, err
	}
	return artifactory.New(serviceConfig)
}

func CreateServiceManagerWithProgressBar(serverDetails *config.ServerDetails, threads, httpRetries, httpRetryWaitMilliSecs int, dryRun bool, progressBar ioUtils.ProgressMgr) (artifactory.ArtifactoryServicesManager, error) {
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

func CreateLifecycleServiceManager(serviceDetails *config.ServerDetails, isDryRun bool) (*lifecycle.LifecycleServicesManager, error) {
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return nil, err
	}
	lcAuth, err := serviceDetails.CreateLifecycleAuthConfig()
	if err != nil {
		return nil, err
	}
	serviceConfig, err := clientConfig.NewConfigBuilder().
		SetServiceDetails(lcAuth).
		SetCertificatesPath(certsPath).
		SetInsecureTls(serviceDetails.InsecureTls).
		SetDryRun(isDryRun).
		Build()
	if err != nil {
		return nil, err
	}
	return lifecycle.New(serviceConfig)
}

func CreateEvidenceServiceManager(serviceDetails *config.ServerDetails, isDryRun bool) (*evidence.EvidenceServicesManager, error) {
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return nil, err
	}
	evdAuth, err := serviceDetails.CreateEvidenceAuthConfig()
	if err != nil {
		return nil, err
	}
	serviceConfig, err := clientConfig.NewConfigBuilder().
		SetServiceDetails(evdAuth).
		SetCertificatesPath(certsPath).
		SetInsecureTls(serviceDetails.InsecureTls).
		SetDryRun(isDryRun).
		Build()
	if err != nil {
		return nil, err
	}
	return evidence.New(serviceConfig)
}

func CreateMetadataServiceManager(serviceDetails *config.ServerDetails, isDryRun bool) (metadata.Manager, error) {
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return nil, err
	}
	mdAuth, err := serviceDetails.CreateMetadataAuthConfig()
	if err != nil {
		return nil, err
	}
	serviceConfig, err := clientConfig.NewConfigBuilder().
		SetServiceDetails(mdAuth).
		SetCertificatesPath(certsPath).
		SetInsecureTls(serviceDetails.InsecureTls).
		SetDryRun(isDryRun).
		Build()
	if err != nil {
		return nil, err
	}
	return metadata.NewManager(serviceConfig)
}

func CreateJfConnectServiceManager(serverDetails *config.ServerDetails, httpRetries, httpRetryWaitMilliSecs int) (jfconnect.Manager, error) {
	certsPath, err := coreutils.GetJfrogCertsDir()
	if err != nil {
		return nil, err
	}
	jfConnectAuth, err := serverDetails.CreateJfConnectAuthConfig()
	if err != nil {
		return nil, err
	}
	serviceConfig, err := clientConfig.NewConfigBuilder().
		SetServiceDetails(jfConnectAuth).
		SetCertificatesPath(certsPath).
		SetInsecureTls(serverDetails.InsecureTls).
		SetHttpRetries(httpRetries).
		SetHttpRetryWaitMilliSecs(httpRetryWaitMilliSecs).
		Build()
	if err != nil {
		return nil, err
	}
	return jfconnect.NewManager(serviceConfig)
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
	defer ioutils.Close(ioReaderCloser, &err)
	content, err := io.ReadAll(ioReaderCloser)
	if err != nil {
		return errorutils.CheckError(err)
	}
	return errorutils.CheckError(json.Unmarshal(content, loadTarget))
}

// Returns an error if the given repo doesn't exist.
func ValidateRepoExists(repoKey string, serviceDetails auth.ServiceDetails) error {
	servicesManager, err := createServiceManager(serviceDetails)
	if err != nil {
		return err
	}
	exists, err := servicesManager.IsRepoExists(repoKey)
	if err != nil {
		return fmt.Errorf("failed while attempting to check if repository %q exists in Artifactory: %w", repoKey, err)
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
	serviceConfig, err := clientConfig.NewConfigBuilder().
		SetServiceDetails(serviceDetails).
		SetCertificatesPath(certsPath).
		SetDryRun(false).
		Build()
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

func GetRtMajorVersion(servicesManager artifactory.ArtifactoryServicesManager) (int, error) {
	artVersion, err := servicesManager.GetVersion()
	if err != nil {
		return -1, err
	}
	artVersionSlice := strings.Split(artVersion, ".")
	majorVersion, err := strconv.Atoi(artVersionSlice[0])
	return majorVersion, errorutils.CheckError(err)
}
