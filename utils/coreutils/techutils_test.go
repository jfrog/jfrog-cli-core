package coreutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTechIndicator(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected []Technology
	}{
		{"simpleMavenTest", "pom.xml", []Technology{Maven}},
		{"npmTest", "../package.json", []Technology{Npm}},
		{"windowsGradleTest", "c:\\users\\test\\package\\build.gradle", []Technology{Gradle}},
		{"windowsPipTest", "c:\\users\\test\\package\\setup.py", []Technology{Pip}},
		{"windowsPipenvTest", "c:\\users\\test\\package\\pipfile", []Technology{Pipenv}},
		{"golangTest", "/Users/eco/dev/jfrog-cli-core/go.mod", []Technology{Go}},
		{"windowsNugetTest", "c:\\users\\test\\package\\project.sln", []Technology{Nuget, Dotnet}},
		{"noTechTest", "pomxml", []Technology{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detectedTech := detectTechnologiesByFile(test.filePath, false)
			assert.Equal(t, test.expected, detectedTech)
		})
	}
}
