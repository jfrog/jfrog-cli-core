package utils

import (
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
	"testing"
)

// The test only checks cases of returning an error in case of a violation with FailBuild == true
func TestPrintViolationsTable(t *testing.T) {
	tests := []struct {
		violations    []services.Violation
		expectedError bool
	}{
		{[]services.Violation{{FailBuild: false}, {FailBuild: false}, {FailBuild: false}}, false},
		{[]services.Violation{{FailBuild: false}, {FailBuild: true}, {FailBuild: false}}, true},
		{[]services.Violation{{FailBuild: true}, {FailBuild: true}, {FailBuild: true}}, true},
	}

	for _, test := range tests {
		err := PrintViolationsTable(test.violations, false)
		assert.Equal(t, test.expectedError, err != nil)
	}
}

func TestSplitComponentId(t *testing.T) {
	tests := []struct {
		componentId         string
		expectedCompName    string
		expectedCompVersion string
		expectedCompType    string
	}{
		{"gav://antparent:ant:1.6.5", "antparent:ant", "1.6.5", "Maven"},
		{"docker://jfrog/artifactory-oss:latest", "jfrog/artifactory-oss", "latest", "Docker"},
		{"rpm://7:rpm-python:7:4.11.3-43.el7", "rpm-python", "7:4.11.3-43.el7", "RPM"},
		{"rpm://rpm-python:7:4.11.3-43.el7", "rpm-python", "7:4.11.3-43.el7", "RPM"},
		{"deb://ubuntu:trustee:acl:2.2.49-2", "ubuntu:trustee:acl", "2.2.49-2", "Debian"},
		{"nuget://log4net:9.0.1", "log4net", "9.0.1", "NuGet"},
		{"generic://sha256:244fd47e07d1004f0aed9c156aa09083c82bf8944eceb67c946ff7430510a77b/foo.jar", "foo.jar", "", "Generic"},
		{"npm://mocha:2.4.5", "mocha", "2.4.5", "npm"},
		{"pip://raven:5.13.0", "raven", "5.13.0", "Python"},
		{"composer://nunomaduro/collision:1.1", "nunomaduro/collision", "1.1", "Composer"},
		{"go://github.com/ethereum/go-ethereum:1.8.2", "github.com/ethereum/go-ethereum", "1.8.2", "Go"},
		{"alpine://3.7:htop:2.0.2-r0", "3.7:htop", "2.0.2-r0", "Alpine"},
		{"invalid-component-id:1.0.0", "invalid-component-id:1.0.0", "", ""},
	}

	for _, test := range tests {
		actualCompName, actualCompVersion, actualCompType := splitComponentId(test.componentId)
		assert.Equal(t, test.expectedCompName, actualCompName)
		assert.Equal(t, test.expectedCompVersion, actualCompVersion)
		assert.Equal(t, test.expectedCompType, actualCompType)
	}
}
