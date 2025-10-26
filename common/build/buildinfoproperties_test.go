package build

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jfrog/build-info-go/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/project"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

const (
	host              = "proxy.mydomain"
	port              = "8888"
	username          = "login"
	password          = "password"
	httpProxyForTest  = "http://" + username + ":" + password + "@" + host + ":" + port
	httpsHost         = "proxy.mydomains"
	httpsPort         = "8889"
	httpsUsername     = "logins"
	httpsPassword     = "passwords"
	httpsProxyForTest = "http://" + httpsUsername + ":" + httpsPassword + "@" + httpsHost + ":" + httpsPort
)

func GetTestDataPath() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", errorutils.CheckError(err)
	}
	return filepath.Join(dir, "testdata"), nil
}

func TestCreateDefaultPropertiesFile(t *testing.T) {
	proxyOrg := getOriginalProxyValue()
	setAndAssertProxy("", t)
	testdataPath, err := GetTestDataPath()
	assert.NoError(t, err)
	data := []struct {
		projectType   project.ProjectType
		expectedProps string
	}{
		{project.Maven, filepath.Join(testdataPath, "expected_maven_test_create_default_properties_file.json")},
		{project.Gradle, filepath.Join(testdataPath, "expected_gradle_test_create_default_properties_file.json")},
	}
	for _, d := range data {
		testCreateDefaultPropertiesFile(d.projectType, d.expectedProps, t)
	}
	setAndAssertProxy(proxyOrg, t)
}

func testCreateDefaultPropertiesFile(projectType project.ProjectType, expectedPropsFilePath string, t *testing.T) {
	providedConfig := viper.New()
	providedConfig.Set("type", projectType.String())
	expectedProps := map[string]string{}
	assert.NoError(t, utils.Unmarshal(expectedPropsFilePath, &expectedProps))
	props, err := CreateBuildInfoProps("", providedConfig, projectType)
	if err != nil {
		t.Error(err)
	}
	assert.True(t, fmt.Sprint(expectedProps) == fmt.Sprint(props), "unexpected "+projectType.String()+" props. got:\n"+fmt.Sprint(props)+"\nwant: "+fmt.Sprint(expectedProps)+"\n")
}

func TestCreateSimplePropertiesFileWithHttpProxy(t *testing.T) {
	// Save old configuration
	proxyOrg := getOriginalProxyValue()

	// Prepare
	setAndAssertProxy(httpProxyForTest, t)
	testdataPath, err := GetTestDataPath()
	assert.NoError(t, err)

	// Run
	createSimplePropertiesFile(t, filepath.Join(testdataPath, "expected_test_create_simple_properties_file_with_proxy.json"))

	// Cleanup
	setAndAssertProxy(proxyOrg, t)
}

func TestCreateSimplePropertiesFileWithNoProxy(t *testing.T) {
	// Save old configuration
	proxyOrg := getOriginalNoProxyValue()

	// Prepare
	setAndAssertNoProxy(httpProxyForTest, t)
	testdataPath, err := GetTestDataPath()
	assert.NoError(t, err)

	// Run
	createSimplePropertiesFile(t, filepath.Join(testdataPath, "expected_test_create_simple_properties_file_with_no_proxy.json"))

	// Cleanup
	setAndAssertNoProxy(proxyOrg, t)
}

func TestCreateSimplePropertiesFileWithHttpsProxy(t *testing.T) {
	// Save old configuration
	oldProxy := getOriginalHttpsProxyValue()

	// Prepare
	setAndAssertHttpsProxy(httpsProxyForTest, t)
	testdataPath, err := GetTestDataPath()
	assert.NoError(t, err)

	// Run
	createSimplePropertiesFile(t, filepath.Join(testdataPath, "expected_test_create_simple_properties_file_with_https_proxy.json"))

	// Cleanup
	setAndAssertHttpsProxy(oldProxy, t)
}

func TestCreateSimplePropertiesFileWithHttpAndHttpsProxy(t *testing.T) {
	// Save old configuration
	oldProxy := getOriginalProxyValue()
	oldHttpsProxy := getOriginalHttpsProxyValue()

	// Prepare
	setAndAssertProxy(httpProxyForTest, t)
	setAndAssertHttpsProxy(httpsProxyForTest, t)
	testdataPath, err := GetTestDataPath()
	assert.NoError(t, err)

	// Run
	createSimplePropertiesFile(t, filepath.Join(testdataPath, "expected_test_create_simple_properties_file_with_http_https_proxy.json"))

	// Cleanup
	setAndAssertProxy(oldProxy, t)
	setAndAssertHttpsProxy(oldHttpsProxy, t)
}

func TestCreateSimplePropertiesFileWithoutProxy(t *testing.T) {
	proxyOrg := getOriginalProxyValue()
	setAndAssertProxy("", t)
	testdataPath, err := GetTestDataPath()
	assert.NoError(t, err)
	createSimplePropertiesFile(t, filepath.Join(testdataPath, "expected_test_create_simple_properties_file_without_proxy.json"))
	setAndAssertProxy(proxyOrg, t)
}

func createSimplePropertiesFile(t *testing.T, expectedPropsFilePath string) {
	var yamlConfig = map[string]string{
		// jfrog-ignore - unsafe url for tests
		ResolverPrefix + Url: "http://some.url.com",
		// jfrog-ignore - unsafe url for tests
		DeployerPrefix + Url: "http://some.other.url.com",
	}
	var expectedProps map[string]interface{}
	assert.NoError(t, utils.Unmarshal(expectedPropsFilePath, &expectedProps))
	vConfig := viper.New()
	vConfig.Set("type", project.Maven.String())
	for k, v := range yamlConfig {
		vConfig.Set(k, v)
	}
	props, err := CreateBuildInfoProps("", vConfig, project.Maven)
	if err != nil {
		t.Error(err)
	}
	assert.True(t, fmt.Sprint(props) == fmt.Sprint(expectedProps))
}

func compareViperConfigs(t *testing.T, actual, expected *viper.Viper, projectType project.ProjectType) {
	for _, key := range expected.AllKeys() {
		value := expected.GetString(key)
		if !actual.IsSet(key) {
			t.Error("["+projectType.String()+"]: Expected key was not found:", "'"+key+"'")
			continue
		}
		if actual.GetString(key) != value {
			t.Error("["+projectType.String()+"]: Expected:", "('"+key+"','"+value+"'),", "found:", "('"+key+"','"+actual.GetString(key)+"').")
		}
	}

	for _, key := range actual.AllKeys() {
		value := actual.GetString(key)
		if !expected.IsSet(key) {
			t.Error("["+projectType.String()+"]: Unexpected key, value found:", "('"+key+"','"+value+"')")
		}
	}
}

func TestSetHttpProxy(t *testing.T) {
	// Save old configuration
	proxyOrg := getOriginalProxyValue()
	backupProxyPass := os.Getenv(httpProxy + Password)

	// Prepare
	setAndAssertProxy(httpProxyForTest, t)
	vConfig := viper.New()
	expectedConfig := viper.New()
	expectedConfig.Set(httpProxy+Host, host)
	expectedConfig.Set(httpProxy+Port, port)
	expectedConfig.Set(httpProxy+Username, username)

	// Run
	err := setProxyIfDefined(vConfig)
	assert.NoError(t, err)

	// Assert
	compareViperConfigs(t, vConfig, expectedConfig, project.Maven)
	assert.Equal(t, password, os.Getenv(httpProxy+Password))

	// Cleanup
	setAndAssertProxy(proxyOrg, t)
	assert.NoError(t, os.Setenv(httpProxy+Password, backupProxyPass))
}

func TestSetHttpsProxy(t *testing.T) {
	// Save old configuration
	oldHttpsProxy := getOriginalHttpsProxyValue()
	backupProxyPass := os.Getenv(httpsProxy + Password)

	// Prepare
	setAndAssertHttpsProxy(httpsProxyForTest, t)
	vConfig := viper.New()
	expectedConfig := viper.New()
	expectedConfig.Set(httpsProxy+Host, httpsHost)
	expectedConfig.Set(httpsProxy+Port, httpsPort)
	expectedConfig.Set(httpsProxy+Username, httpsUsername)

	// Run
	assert.NoError(t, setProxyIfDefined(vConfig))

	// Assert
	compareViperConfigs(t, vConfig, expectedConfig, project.Maven)
	assert.Equal(t, httpsPassword, os.Getenv(httpsProxy+Password))

	// Cleanup
	setAndAssertHttpsProxy(oldHttpsProxy, t)
	assert.NoError(t, os.Setenv(httpsProxy+Password, backupProxyPass))
}

func getOriginalProxyValue() string {
	return os.Getenv(HttpProxyEnvKey)
}

func getOriginalNoProxyValue() string {
	return os.Getenv(NoProxyEnvKey)
}

func getOriginalHttpsProxyValue() string {
	return os.Getenv(HttpsProxyEnvKey)
}

func setAndAssertProxy(proxy string, t *testing.T) {
	testsutils.SetEnvAndAssert(t, HttpProxyEnvKey, proxy)
}

func setAndAssertHttpsProxy(proxy string, t *testing.T) {
	testsutils.SetEnvAndAssert(t, HttpsProxyEnvKey, proxy)
}

func setAndAssertNoProxy(proxy string, t *testing.T) {
	testsutils.SetEnvAndAssert(t, NoProxyEnvKey, proxy)
}

func TestCreateDefaultConfigWithParams(t *testing.T) {
	params := map[string]any{
		"usewrapper":   true,
		"resolver.url": "http://localhost",
	}
	config := createDefaultConfigWithParams("YAML", "gradle", params)
	assert.True(t, config.IsSet("usewrapper"))
	assert.True(t, config.IsSet("resolver.url"))
	assert.True(t, config.IsSet("type"))
}
