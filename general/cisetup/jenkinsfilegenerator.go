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
		%s

	  JFROG_CLI_BUILD_NAME = "${JOB_NAME}"
	  JFROG_CLI_BUILD_NUMBER = "${BUILD_NUMBER}"

	  // Sets the CI server build URL in the build-info.
	  JFROG_CLI_BUILD_URL = "https://<my-jenkins-domain>/<my-job-uri>/${BUILD_NUMBER}/console"
	  
	}
	stages {
		stage ('Clone') {
			steps {
				// If cloning the code requires credentials, follow these steps:
				// 1. Uncomment the ending of the below 'git' step.
				// 2. Create the 'git_cred_id' credentials as described here - https://www.jenkins.io/doc/book/using/using-credentials/
				git branch: %q, url: %q //, credentialsId: 'git_cred_id'
			}
		}
   
		stage ('Config') {
			steps {
				// Download JFrog CLI.
				sh 'curl -fL https://getcli.jfrog.io | sh && chmod +x jfrog'

				// Configure JFrog CLI 
				withCredentials([string(credentialsId: 'rt-password', variable: 'RT_PASSWORD')]) {
					sh '''./jfrog c add %s --url %s --user ${RT_USERNAME} --password ${RT_PASSWORD}
					./%s
					'''
				}
			}
		}
   
		stage ('Build') {
			steps {
				dir('%s') {
					sh '''./%s'''
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
			// Collect and store environment variables in the build-info
			sh './jfrog rt bce'
			// Collect and store VCS details in the build-info
			sh './jfrog rt bag'
			// Publish the build-info to Artifactory
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
	// Set the M2_HOME env variable if maven is used.
	if jg.SetupData.BuiltTechnology.Type == Maven {
		envSet = m2HomeSet
	}
	return []byte(fmt.Sprintf(jenkinsfileTemplate, envSet, jg.SetupData.GitBranch, jg.SetupData.VcsCredentials.Url, ConfigServerId, serviceDetails.Url, buildToolsconfigCommands, jg.SetupData.RepositoryName, buildCommand, ConfigServerId)), JenkinsfileName, nil
}
