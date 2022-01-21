package utils

import (
	"fmt"
	"os"
	"strings"
	"testing"

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

	for index := range ProjectTypes {
		testCreateDefaultPropertiesFile(ProjectType(index), t)
	}
	setProxy(proxyOrg, t)
}

func testCreateDefaultPropertiesFile(projectType ProjectType, t *testing.T) {
	providedConfig := viper.New()
	providedConfig.Set("type", projectType.String())

	props, err := CreateBuildInfoProps("", providedConfig, projectType)
	if err != nil {
		t.Error(err)
	}
	expectedProps := make(map[string]string)
	for _, partialMapping := range buildTypeConfigMapping[projectType] {
		for propertyKey := range *partialMapping {
			if defaultPropertiesValues[propertyKey] != "" {
				expectedProps[propertyKey] = defaultPropertiesValues[propertyKey]
				if strings.HasPrefix(propertyKey, "artifactory.") {
					expectedProps[strings.TrimPrefix(propertyKey, "artifactory.")] = defaultPropertiesValues[propertyKey]
				}
			}
		}
	}
	assert.True(t, fmt.Sprint(props) == fmt.Sprint(expectedProps))
}

func TestCreateSimplePropertiesFileWithProxy(t *testing.T) {
	proxyOrg := getOriginalProxyValue()
	setProxy(proxy, t)
	var propertiesFileConfig = map[string]string{
		"artifactory.resolve.contextUrl": "http://some.url.com",
		"artifactory.publish.contextUrl": "http://some.other.url.com",
		"artifactory.proxy.host":         host,
		"artifactory.proxy.port":         port,
	}
	createSimplePropertiesFile(t, propertiesFileConfig)
	setProxy(proxyOrg, t)
}

func TestCreateSimplePropertiesFileWithoutProxy(t *testing.T) {
	proxyOrg := getOriginalProxyValue()
	setProxy("", t)
	var propertiesFileConfig = map[string]string{
		"artifactory.resolve.contextUrl": "http://some.url.com",
		"artifactory.publish.contextUrl": "http://some.other.url.com",
	}
	createSimplePropertiesFile(t, propertiesFileConfig)
	setProxy(proxyOrg, t)

}

func createSimplePropertiesFile(t *testing.T, propertiesFileConfig map[string]string) {
	var yamlConfig = map[string]string{
		ResolverPrefix + Url: "http://some.url.com",
		DeployerPrefix + Url: "http://some.other.url.com",
	}

	vConfig := viper.New()
	vConfig.Set("type", Maven.String())
	for k, v := range yamlConfig {
		vConfig.Set(k, v)
	}
	props, err := CreateBuildInfoProps("", vConfig, Maven)
	if err != nil {
		t.Error(err)
	}
	expectedProps := make(map[string]string)
	for _, partialMapping := range buildTypeConfigMapping[Maven] {
		for propertyKey := range *partialMapping {
			if defaultPropertiesValues[propertyKey] != "" {
				expectedProps[propertyKey] = defaultPropertiesValues[propertyKey]
				if strings.HasPrefix(propertyKey, "artifactory.") {
					expectedProps[strings.TrimPrefix(propertyKey, "artifactory.")] = defaultPropertiesValues[propertyKey]
				}
			}
		}
	}
	for k, v := range propertiesFileConfig {
		expectedProps[k] = v
		if strings.HasPrefix(k, "artifactory.") {
			expectedProps[strings.TrimPrefix(k, "artifactory.")] = v
		}
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
