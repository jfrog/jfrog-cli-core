package cisetup

const JenkinsfileName = "Jenkinsfile"
const jenkinsfileTemplate = `pipeline {
	agent any
	environment {
	  JFROG_CLI_BUILD_NAME = "${JOB_NAME}"
	  JFROG_CLI_BUILD_NUMBER = "${BUILD_NUMBER}"
	  // Sets the CI server build URL in the build-info.
	  JFROG_CLI_BUILD_URL = "https://<my-jenkins-domain>/<my-job-uri>/${BUILD_NUMBER}/console"
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
				sh './jfrog c add rt-server --url %s --user ${RT_USERNAME} --password ${RT_PASSWORD}'
				sh '%s'
			}
		}
   
		stage ('Build') {
			steps {
				dir('%s') {
					sh '%s'
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
		}
	}
  }`

type JenkinsfileGenerator struct {
	SetupData *CiSetupData
}

func (jg *JenkinsfileGenerator) Generate() (jenkinsfileBytes []byte, jenkinsfileName string, err error) {
	return nil, "", nil
}
