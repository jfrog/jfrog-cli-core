package java

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/sca"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestMavenTreesMultiModule(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := sca.CreateTestWorkspace(t, "maven-example")
	defer cleanUp()

	expectedUniqueDeps := []string{
		GavPackageTypeIdentifier + "javax.mail:mail:1.4",
		GavPackageTypeIdentifier + "org.testng:testng:5.9",
		GavPackageTypeIdentifier + "javax.servlet:servlet-api:2.5",
		GavPackageTypeIdentifier + "org.jfrog.test:multi:3.7-SNAPSHOT",
		GavPackageTypeIdentifier + "org.jfrog.test:multi3:3.7-SNAPSHOT",
		GavPackageTypeIdentifier + "org.jfrog.test:multi2:3.7-SNAPSHOT",
		GavPackageTypeIdentifier + "junit:junit:3.8.1",
		GavPackageTypeIdentifier + "org.jfrog.test:multi1:3.7-SNAPSHOT",
		GavPackageTypeIdentifier + "commons-io:commons-io:1.4",
		GavPackageTypeIdentifier + "org.apache.commons:commons-email:1.1",
		GavPackageTypeIdentifier + "javax.activation:activation:1.1",
		GavPackageTypeIdentifier + "hsqldb:hsqldb:1.8.0.10",
	}
	// Run getModulesDependencyTrees
	modulesDependencyTrees, uniqueDeps, err := buildMavenDependencyTree(&DepTreeParams{})
	if assert.NoError(t, err) && assert.NotEmpty(t, modulesDependencyTrees) {
		assert.ElementsMatch(t, uniqueDeps, expectedUniqueDeps, "First is actual, Second is Expected")
		// Check root module
		multi := sca.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi:3.7-SNAPSHOT")
		if assert.NotNil(t, multi) {
			assert.Len(t, multi.Nodes, 1)
			// Check multi1 with a transitive dependency
			multi1 := sca.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi1:3.7-SNAPSHOT")
			assert.Len(t, multi1.Nodes, 4)
			commonsEmail := sca.GetAndAssertNode(t, multi1.Nodes, "org.apache.commons:commons-email:1.1")
			assert.Len(t, commonsEmail.Nodes, 2)

			// Check multi2 and multi3
			multi2 := sca.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi2:3.7-SNAPSHOT")
			assert.Len(t, multi2.Nodes, 1)
			multi3 := sca.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi3:3.7-SNAPSHOT")
			assert.Len(t, multi3.Nodes, 4)
		}
	}
}

func TestMavenWrapperTrees(t *testing.T) {
	// Create and change directory to test workspace
	_, cleanUp := sca.CreateTestWorkspace(t, "maven-example-with-wrapper")
	err := os.Chmod("mvnw", 0700)
	defer cleanUp()
	assert.NoError(t, err)
	expectedUniqueDeps := []string{
		GavPackageTypeIdentifier + "org.jfrog.test:multi1:3.7-SNAPSHOT",
		GavPackageTypeIdentifier + "org.codehaus.plexus:plexus-utils:1.5.1",
		GavPackageTypeIdentifier + "org.springframework:spring-beans:2.5.6",
		GavPackageTypeIdentifier + "commons-logging:commons-logging:1.1.1",
		GavPackageTypeIdentifier + "org.jfrog.test:multi3:3.7-SNAPSHOT",
		GavPackageTypeIdentifier + "org.apache.commons:commons-email:1.1",
		GavPackageTypeIdentifier + "org.springframework:spring-aop:2.5.6",
		GavPackageTypeIdentifier + "org.springframework:spring-core:2.5.6",
		GavPackageTypeIdentifier + "org.jfrog.test:multi:3.7-SNAPSHOT",
		GavPackageTypeIdentifier + "org.jfrog.test:multi2:3.7-SNAPSHOT",
		GavPackageTypeIdentifier + "org.testng:testng:5.9",
		GavPackageTypeIdentifier + "hsqldb:hsqldb:1.8.0.10",
		GavPackageTypeIdentifier + "junit:junit:3.8.1",
		GavPackageTypeIdentifier + "javax.activation:activation:1.1",
		GavPackageTypeIdentifier + "javax.mail:mail:1.4",
		GavPackageTypeIdentifier + "aopalliance:aopalliance:1.0",
		GavPackageTypeIdentifier + "commons-io:commons-io:1.4",
		GavPackageTypeIdentifier + "javax.servlet.jsp:jsp-api:2.1",
		GavPackageTypeIdentifier + "javax.servlet:servlet-api:2.5",
	}

	modulesDependencyTrees, uniqueDeps, err := buildMavenDependencyTree(&DepTreeParams{UseWrapper: true})
	if assert.NoError(t, err) && assert.NotEmpty(t, modulesDependencyTrees) {
		assert.ElementsMatch(t, uniqueDeps, expectedUniqueDeps, "First is actual, Second is Expected")
		// Check root module
		multi := sca.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi:3.7-SNAPSHOT")
		if assert.NotNil(t, multi) {
			assert.Len(t, multi.Nodes, 1)
			// Check multi1 with a transitive dependency
			multi1 := sca.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi1:3.7-SNAPSHOT")
			assert.Len(t, multi1.Nodes, 7)
			commonsEmail := sca.GetAndAssertNode(t, multi1.Nodes, "org.apache.commons:commons-email:1.1")
			assert.Len(t, commonsEmail.Nodes, 2)
			// Check multi2 and multi3
			multi2 := sca.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi2:3.7-SNAPSHOT")
			assert.Len(t, multi2.Nodes, 1)
			multi3 := sca.GetAndAssertNode(t, modulesDependencyTrees, "org.jfrog.test:multi3:3.7-SNAPSHOT")
			assert.Len(t, multi3.Nodes, 4)
		}
	}
}

func TestGetMavenPluginInstallationArgs(t *testing.T) {
	args := GetMavenPluginInstallationArgs("testPlugin")
	assert.Equal(t, "org.apache.maven.plugins:maven-install-plugin:2.5.2:install-file", args[0])
	assert.Equal(t, "-Dfile=testPlugin", args[1])
}

func TestCreateMvnProps(t *testing.T) {
	// Valid server details with username and password
	mockServerDetails1 := &config.ServerDetails{
		User:           "user1",
		Password:       "password1",
		Url:            "https://jfrog.com",
		ArtifactoryUrl: "https://jfrog.com/artifactory",
	}
	result1 := createMvnProps("repo1", mockServerDetails1)
	assert.NotNil(t, result1)
	assert.Equal(t, "user1", result1["resolver.username"])
	assert.Equal(t, "password1", result1["resolver.password"])
	assert.Equal(t, "https://jfrog.com/artifactory", result1["resolver.url"])
	assert.Equal(t, "repo1", result1["resolver.releaseRepo"])
	assert.Equal(t, "repo1", result1["resolver.snapshotRepo"])
	assert.True(t, result1["buildInfoConfig.artifactoryResolutionEnabled"].(bool))

	// Valid server details with access token
	mockServerDetails2 := &config.ServerDetails{
		AccessToken:    "token2",
		Url:            "https://jfrog.com",
		ArtifactoryUrl: "https://jfrog.com/artifactory",
	}
	result2 := createMvnProps("repo2", mockServerDetails2)
	assert.NotNil(t, result2)
	assert.Equal(t, "", result2["resolver.username"])
	assert.Equal(t, "token2", result2["resolver.password"])
	assert.Equal(t, "https://jfrog.com/artifactory", result2["resolver.url"])
	assert.Equal(t, "repo2", result2["resolver.releaseRepo"])
	assert.Equal(t, "repo2", result2["resolver.snapshotRepo"])
	assert.True(t, result2["buildInfoConfig.artifactoryResolutionEnabled"].(bool))

	// Empty server details
	result3 := createMvnProps("repo3", nil)
	assert.Nil(t, result3)
}
