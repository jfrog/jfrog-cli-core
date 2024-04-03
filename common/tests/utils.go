package tests

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	testsutils "github.com/jfrog/jfrog-cli-core/v2/utils/config/tests"
	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	"github.com/jfrog/jfrog-client-go/access"
	"github.com/jfrog/jfrog-client-go/artifactory"
	"github.com/jfrog/jfrog-client-go/distribution"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"github.com/stretchr/testify/assert"
)

func ConfigTestServer(t *testing.T) (cleanUp func(), err error) {
	cleanUp = testsutils.CreateTempEnv(t, false)
	serverDetails := CreateTestServerDetails()
	err = config.SaveServersConf([]*config.ServerDetails{serverDetails})
	return
}

func CreateTestServerDetails() *config.ServerDetails {
	return &config.ServerDetails{
		Url:               "http://localhost:8080/",
		ArtifactoryUrl:    "http://localhost:8080/artifactory/",
		DistributionUrl:   "http://localhost:8080/distribution/",
		XrayUrl:           "http://localhost:8080/xray/",
		MissionControlUrl: "http://localhost:8080/mc/",
		PipelinesUrl:      "http://localhost:8080/pipelines/",
		ServerId:          "test",
		IsDefault:         true,
		ClientCertPath:    "ClientCertPath", ClientCertKeyPath: "ClientCertKeyPath",
	}
}

type restsTestHandler func(w http.ResponseWriter, r *http.Request)

// Create mock server to test REST APIs.
// testHandler - The HTTP handler of the test
func CreateRestsMockServer(testHandler restsTestHandler) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(testHandler))
}

func CreateRtRestsMockServer(t *testing.T, testHandler restsTestHandler) (*httptest.Server, *config.ServerDetails, artifactory.ArtifactoryServicesManager) {
	testServer := CreateRestsMockServer(testHandler)
	serverDetails := &config.ServerDetails{Url: testServer.URL + "/", ArtifactoryUrl: testServer.URL + "/"}

	serviceManager, err := utils.CreateServiceManager(serverDetails, -1, 0, false)
	assert.NoError(t, err)
	return testServer, serverDetails, serviceManager
}

func CreateAccessRestsMockServer(t *testing.T, testHandler restsTestHandler) (*httptest.Server, *config.ServerDetails, *access.AccessServicesManager) {
	testServer := CreateRestsMockServer(testHandler)
	serverDetails := &config.ServerDetails{Url: testServer.URL + "/", ServerId: "test-server"}

	serviceManager, err := utils.CreateAccessServiceManager(serverDetails, false)
	assert.NoError(t, err)
	return testServer, serverDetails, serviceManager
}

func CreateDsRestsMockServer(t *testing.T, testHandler restsTestHandler) (*httptest.Server, *config.ServerDetails, *distribution.DistributionServicesManager) {
	testServer := CreateRestsMockServer(testHandler)
	serverDetails := &config.ServerDetails{DistributionUrl: testServer.URL + "/"}

	serviceManager, err := utils.CreateDistributionServiceManager(serverDetails, false)
	assert.NoError(t, err)
	return testServer, serverDetails, serviceManager
}

// Set progressbar.ShouldInitProgressBar func to always return true
// so the progress bar library will be initialized and progress will be displayed.
// The returned callback sets the original func back.
func MockProgressInitialization() func() {
	originFunc := progressbar.ShouldInitProgressBar
	progressbar.ShouldInitProgressBar = func() (bool, error) { return true, nil }
	return func() {
		progressbar.ShouldInitProgressBar = originFunc
	}
}

// Replace all variables in the form of ${VARIABLE} in the input file, according to the substitution map.
// path - Path to the input file.
// destPath - Path to the output file. If empty, the output file will be under ${CWD}/tmp/.
func ReplaceTemplateVariables(path, destPath string, subMap map[string]string) (string, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	for name, value := range subMap {
		content = bytes.ReplaceAll(content, []byte(name), []byte(value))
	}
	if destPath == "" {
		destPath, err = os.Getwd()
		if err != nil {
			return "", errorutils.CheckError(err)
		}
		destPath = filepath.Join(destPath, "tmp")
	}
	err = os.MkdirAll(destPath, 0700)
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	specPath := filepath.Join(destPath, filepath.Base(path))
	log.Info("Creating spec file at:", specPath)
	err = os.WriteFile(specPath, content, 0700)
	if err != nil {
		return "", errorutils.CheckError(err)
	}

	return specPath, nil
}
