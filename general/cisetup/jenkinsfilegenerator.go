package cisetup

import (
	"fmt"
	"strings"

	"github.com/jfrog/jfrog-cli-core/utils/config"
)

const JenkinsfileName = "Jenkinsfile"
const m2HomeSet = `
	  // The M2_HOME environment variable should be set to the local maven installation path.
	  M2_HOME = ""`
const jenkinsfileTemplate = `pipeline {
	agent any
	environment {
	  JFROG_CLI_BUILD_NAME = "${JOB_NAME}"
	  JFROG_CLI_BUILD_NUMBER = "${BUILD_NUMBER}"
	  // Sets the CI server build URL in the build-info.
	  JFROG_CLI_BUILD_URL = "https://<my-jenkins-domain>/<my-job-uri>/${BUILD_NUMBER}/console"
	  %s
	}
	stages {
		stage ('Clone') {
			steps {
				git branch: %q, url: %q
			}
		}
   
		stage ('Config') {
			steps {
				sh 'curl -fL https://getcli.jfrog.io | sh && chmod +x jfrog'
				// General JFrog CLI config
				withCredentials([string(credentialsId: 'rt-password', variable: 'RT_PASSWORD')]) {
					sh './jfrog c add %s --url %s --user ${RT_USERNAME} --password ${RT_PASSWORD}'
				}
				// Specific build tools config
				sh '''%s'''
			}
		}
   
		stage ('Build') {
			steps {
				dir('%s') {
					sh '''%s'''
				}
			}
		}
	}
	   
	post {
		success {
			script {
				env.JFROG_BUILD_STATUS="PASS"
			}
		}
		 
		failure {
			script {
				env.JFROG_BUILD_STATUS="FAIL"
			}
		}
		 
		cleanup {
			sh './jfrog rt bce'
			sh './jfrog rt bag'
			sh './jfrog rt bp'
			sh './jfrog c remove %s --quiet'
		}
	}
  }`

type JenkinsfileGenerator struct {
	SetupData *CiSetupData
}

func (jg *JenkinsfileGenerator) Generate() (jenkinsfileBytes []byte, jenkinsfileName string, err error) {
	serviceDetails, err := config.GetSpecificConfig(ConfigServerId, false, false)
	if err != nil {
		return nil, "", err
	}
	buildToolsconfigCommands := strings.Join(getTechConfigsCommands(ConfigServerId, false, jg.SetupData), cmdAndOperator)
	buildCommand, err := convertBuildCmd(jg.SetupData)
	if err != nil {
		return nil, "", err
	}
	var envSet string
	if _, used := jg.SetupData.BuiltTechnologies[Maven]; used {
		envSet = m2HomeSet
	}
	return []byte(fmt.Sprintf(jenkinsfileTemplate, envSet, jg.SetupData.GitBranch, jg.SetupData.VcsCredentials.Url, ConfigServerId, serviceDetails.Url, buildToolsconfigCommands, jg.SetupData.RepositoryName, buildCommand, ConfigServerId)), JenkinsfileName, nil
}
