package utils

import (
	"fmt"
	"os"
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
	data := []struct {
		projectType   ProjectType
		expectedProps string
	}{
		{Maven, "map[artifactory.buildInfo.agent.name:/ artifactory.buildInfoConfig.envVarsExcludePatterns:*password*,*psw*,*secret*,*key*,*token* artifactory.buildInfoConfig.includeEnvVars:false artifactory.org.jfrog.build.extractor.maven.recorder.activate:true artifactory.publish.artifacts:true artifactory.publish.buildInfo:false artifactory.publish.filterExcludedArtifactsFromBuild:true artifactory.publish.forkCount:3 artifactory.publish.unstable:false buildInfo.agent.name:/ buildInfoConfig.envVarsExcludePatterns:*password*,*psw*,*secret*,*key*,*token* buildInfoConfig.includeEnvVars:false org.jfrog.build.extractor.maven.recorder.activate:true publish.artifacts:true publish.buildInfo:false publish.filterExcludedArtifactsFromBuild:true publish.forkCount:3 publish.unstable:false]"},
		{Gradle, "map[artifactory.buildInfo.agent.name:/ artifactory.buildInfo.env.extractor.used:true artifactory.buildInfoConfig.envVarsExcludePatterns:*password*,*psw*,*secret*,*key*,*token* artifactory.buildInfoConfig.includeEnvVars:false artifactory.org.jfrog.build.extractor.maven.recorder.activate:true artifactory.publish.artifacts:true artifactory.publish.buildInfo:false artifactory.publish.forkCount:3 artifactory.publish.ivy:false artifactory.publish.maven:false artifactory.publish.unstable:false buildInfo.agent.name:/ buildInfo.env.extractor.used:true buildInfoConfig.envVarsExcludePatterns:*password*,*psw*,*secret*,*key*,*token* buildInfoConfig.includeEnvVars:false org.jfrog.build.extractor.maven.recorder.activate:true publish.artifacts:true publish.buildInfo:false publish.forkCount:3 publish.ivy:false publish.maven:false publish.unstable:false]"},
	}
	for _, d := range data {
		testCreateDefaultPropertiesFile(d.projectType, d.expectedProps, t)
	}
	setProxy(proxyOrg, t)
}

func testCreateDefaultPropertiesFile(projectType ProjectType, expectedProps string, t *testing.T) {
	providedConfig := viper.New()
	providedConfig.Set("type", projectType.String())

	props, err := CreateBuildInfoProps("", providedConfig, projectType)
	if err != nil {
		t.Error(err)
	}
	assert.True(t, expectedProps == fmt.Sprint(props), "unexpected "+projectType.String()+" props. got:\n"+fmt.Sprint(props)+"\nwant: "+expectedProps+"\n")
}

func TestCreateSimplePropertiesFileWithProxy(t *testing.T) {
	proxyOrg := getOriginalProxyValue()
	setProxy(proxy, t)
	var propertiesFileConfig = map[string]string{
		"resolve.contextUrl": "http://some.url.com",
		"publish.contextUrl": "http://some.other.url.com",
		"proxy.host":         host,
		"proxy.port":         port,
	}
	createSimplePropertiesFile(t, "map[artifactory.buildInfo.agent.name:/ artifactory.buildInfoConfig.envVarsExcludePatterns:*password*,*psw*,*secret*,*key*,*token* artifactory.buildInfoConfig.includeEnvVars:false artifactory.org.jfrog.build.extractor.maven.recorder.activate:true artifactory.proxy.host:localhost artifactory.proxy.port:8888 artifactory.publish.artifacts:true artifactory.publish.buildInfo:false artifactory.publish.contextUrl:http://some.other.url.com artifactory.publish.filterExcludedArtifactsFromBuild:true artifactory.publish.forkCount:3 artifactory.publish.unstable:false artifactory.resolve.contextUrl:http://some.url.com buildInfo.agent.name:/ buildInfoConfig.envVarsExcludePatterns:*password*,*psw*,*secret*,*key*,*token* buildInfoConfig.includeEnvVars:false org.jfrog.build.extractor.maven.recorder.activate:true proxy.host:localhost proxy.port:8888 publish.artifacts:true publish.buildInfo:false publish.contextUrl:http://some.other.url.com publish.filterExcludedArtifactsFromBuild:true publish.forkCount:3 publish.unstable:false resolve.contextUrl:http://some.url.com]", propertiesFileConfig)
	setProxy(proxyOrg, t)
}

func TestCreateSimplePropertiesFileWithoutProxy(t *testing.T) {
	proxyOrg := getOriginalProxyValue()
	setProxy("", t)
	var propertiesFileConfig = map[string]string{
		"resolve.contextUrl": "http://some.url.com",
		"publish.contextUrl": "http://some.other.url.com",
	}
	createSimplePropertiesFile(t, "map[artifactory.buildInfo.agent.name:/ artifactory.buildInfoConfig.envVarsExcludePatterns:*password*,*psw*,*secret*,*key*,*token* artifactory.buildInfoConfig.includeEnvVars:false artifactory.org.jfrog.build.extractor.maven.recorder.activate:true artifactory.publish.artifacts:true artifactory.publish.buildInfo:false artifactory.publish.contextUrl:http://some.other.url.com artifactory.publish.filterExcludedArtifactsFromBuild:true artifactory.publish.forkCount:3 artifactory.publish.unstable:false artifactory.resolve.contextUrl:http://some.url.com buildInfo.agent.name:/ buildInfoConfig.envVarsExcludePatterns:*password*,*psw*,*secret*,*key*,*token* buildInfoConfig.includeEnvVars:false org.jfrog.build.extractor.maven.recorder.activate:true publish.artifacts:true publish.buildInfo:false publish.contextUrl:http://some.other.url.com publish.filterExcludedArtifactsFromBuild:true publish.forkCount:3 publish.unstable:false resolve.contextUrl:http://some.url.com]", propertiesFileConfig)
	setProxy(proxyOrg, t)

}

func createSimplePropertiesFile(t *testing.T, expectedProps string, propertiesFileConfig map[string]string) {
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
	assert.True(t, fmt.Sprint(props) == expectedProps)
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
