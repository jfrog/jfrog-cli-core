package cisetup

import (
	"fmt"
	"path/filepath"
	"strings"
)

var GithubActionsDir = filepath.Join(".github", "workflows")
var GithubActionsFilePath = filepath.Join(GithubActionsDir, GithubActionsFileName)

const GithubActionsFileName = "build.yml"
const githubActionsTemplate = `
name: 'JFrog CI Integration'
on: [push]
jobs:
 jfrog-ci-integration:
   runs-on: ubuntu-latest
   env:
     JF_ARTIFACTORY_1: ${{ secrets.JF_ARTIFACTORY_SECRET_1 }}
     JFROG_BUILD_STATUS: PASS
   steps:
     - name: Checkout
       uses: actions/checkout@v2
     - name: Setup JFrog CLI
       uses: jfrog/setup-jfrog-cli@v1
     - name: Set up JDK 11
       uses: actions/setup-java@v2
       with:
         java-version: '11'
         distribution: 'adopt'
     - name: Build
       run: |
         # Configure the project
         %s
         # Build the project using JFrog CLI
         %s
     - name: Failure check
       run: |
         echo "JFROG_BUILD_STATUS=FAIL" >> $GITHUB_ENV
       if: failure()
     - name: Publish build
       run: |
         # Collect and store environment variables in the build-info
         jfrog rt bce
         # Collect and store VCS details in the build-info
         jfrog rt bag
         # Publish the build-info to Artifactory
         jfrog rt bp
         # Scan the published build-info with Xray
         jfrog rt bs
       if: always()`

type GithubActionsGenerator struct {
	SetupData *CiSetupData
}

func (gg *GithubActionsGenerator) Generate() (githubActionsBytes []byte, githubActionsName string, err error) {
	// setM2 env variable if maven is used.
	_, setM2 := gg.SetupData.BuiltTechnologies[Maven]
	buildToolsconfigCommands := strings.Join(getTechConfigsCommands(ConfigServerId, setM2, gg.SetupData), "\n          ")
	buildCommand, err := convertBuildCmd(gg.SetupData)
	if err != nil {
		return nil, "", err
	}
	return []byte(fmt.Sprintf(githubActionsTemplate, buildToolsconfigCommands, buildCommand)), GithubActionsFileName, nil
}
