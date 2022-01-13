package pip

import (
	"testing"
)

func TestGetProjectNameFromFileContent(t *testing.T) {
	tests := []struct {
		fileContent         string
		expectedProjectName string
	}{
		{"Metadata-Version: 1.0\nName: jfrog-python-example-1\nVersion: 1.0\nSummary: Project example for building Python project with JFrog products\nHome-page: https://github.com/jfrog/project-examples\nAuthor: JFrog\nAuthor-email: jfrog@jfrog.com\nLicense: UNKNOWN\nDescription: UNKNOWN\nPlatform: UNKNOWN", "jfrog-python-example-1"},
		{"Metadata-Version: Name: jfrog-python-example-2\nLicense: UNKNOWN\nDescription: UNKNOWN\nPlatform: UNKNOWN\nName: jfrog-python-example-2\nVersion: 1.0\nSummary: Project example for building Python project with JFrog products\nHome-page: https://github.com/jfrog/project-examples\nAuthor: JFrog\nAuthor-email: jfrog@jfrog.com", "jfrog-python-example-2"},
		{"Name:Metadata-Version: 3.0\nName: jfrog-python-example-3\nVersion: 1.0\nSummary: Project example for building Python project with JFrog products\nHome-page: https://github.com/jfrog/project-examples\nAuthor: JFrog\nAuthor-email: jfrog@jfrog.com\nName: jfrog-python-example-4", "jfrog-python-example-3"},
	}

	for _, test := range tests {
		actualValue, err := getProjectNameFromFileContent([]byte(test.fileContent))
		if err != nil {
			t.Error(err)
		}
		if actualValue != test.expectedProjectName {
			t.Errorf("Expected value: %s, got: %s.", test.expectedProjectName, actualValue)
		}
	}
}
