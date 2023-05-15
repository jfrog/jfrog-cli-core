package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/jfrog/build-info-go/utils"
	testsutils "github.com/jfrog/jfrog-client-go/utils/tests"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

const (
	host  = "localhost"
	port  = "8888"
	proxy = "http://" + host + ":" + port
)

func TestCreateDefaultPropertiesFile(t *testing.T) {
	proxyOrg := getOriginalProxyValue()
	setProxy("", t)
	testdataPath, err := GetTestDataPath()
	assert.NoError(t, err)
	data := []struct {
		projectType   ProjectType
		expectedProps string
	}{
		{Maven, filepath.Join(testdataPath, "expected_maven_test_create_default_properties_file.json")},
		{Gradle, filepath.Join(testdataPath, "expected_gradle_test_create_default_properties_file.json")},
	}
	for _, d := range data {
		testCreateDefaultPropertiesFile(d.projectType, d.expectedProps, t)
	}
	setProxy(proxyOrg, t)
}

func testCreateDefaultPropertiesFile(projectType ProjectType, expectedPropsFilePath string, t *testing.T) {
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

func TestCreateSimplePropertiesFileWithProxy(t *testing.T) {
	proxyOrg := getOriginalProxyValue()
	setProxy(proxy, t)
	testdataPath, err := GetTestDataPath()
	assert.NoError(t, err)
	createSimplePropertiesFile(t, filepath.Join(testdataPath, "expected_test_create_simple_properties_file_with_proxy.json"))
	setProxy(proxyOrg, t)
}

func TestCreateSimplePropertiesFileWithoutProxy(t *testing.T) {
	proxyOrg := getOriginalProxyValue()
	setProxy("", t)
	testdataPath, err := GetTestDataPath()
	assert.NoError(t, err)
	createSimplePropertiesFile(t, filepath.Join(testdataPath, "expected_test_create_simple_properties_file_without_proxy.json"))
	setProxy(proxyOrg, t)

}

func createSimplePropertiesFile(t *testing.T, expectedPropsFilePath string) {
	var yamlConfig = map[string]string{
		ResolverPrefix + Url: "http://some.url.com",
		DeployerPrefix + Url: "http://some.other.url.com",
	}
	var expectedProps map[string]interface{}
	assert.NoError(t, utils.Unmarshal(expectedPropsFilePath, &expectedProps))
	vConfig := viper.New()
	vConfig.Set("type", Maven.String())
	for k, v := range yamlConfig {
		vConfig.Set(k, v)
	}
	props, err := CreateBuildInfoProps("", vConfig, Maven)
	if err != nil {
		t.Error(err)
	}
	assert.True(t, fmt.Sprint(props) == fmt.Sprint(expectedProps))
}

func compareViperConfigs(t *testing.T, actual, expected *viper.Viper, projectType ProjectType) {
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

func TestSetProxyIfNeeded(t *testing.T) {
	proxyOrg := getOriginalProxyValue()
	setProxy(proxy, t)
	vConfig := viper.New()

	err := setProxyIfDefined(vConfig)
	if err != nil {
		t.Error(err)
	}

	expectedConfig := viper.New()
	expectedConfig.Set(Proxy+Host, host)
	expectedConfig.Set(Proxy+Port, port)
	compareViperConfigs(t, vConfig, expectedConfig, Maven)

	setProxy(proxyOrg, t)
}

func getOriginalProxyValue() string {
	return os.Getenv(HttpProxy)
}

func setProxy(proxy string, t *testing.T) {
	testsutils.SetEnvAndAssert(t, HttpProxy, proxy)
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
