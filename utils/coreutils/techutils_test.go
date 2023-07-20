package coreutils

import (
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestDetectTechnologiesByFilePaths(t *testing.T) {
	tests := []struct {
		name     string
		paths    []string
		expected map[Technology]bool
	}{
		{"simpleMavenTest", []string{"pom.xml"}, map[Technology]bool{Maven: true}},
		{"npmTest", []string{"../package.json"}, map[Technology]bool{Npm: true}},
		{"yarnTest", []string{"./package.json", "./.yarn"}, map[Technology]bool{Yarn: true}},
		{"windowsGradleTest", []string{"c:\\users\\test\\package\\build.gradle"}, map[Technology]bool{Gradle: true}},
		{"windowsPipTest", []string{"c:\\users\\test\\package\\setup.py"}, map[Technology]bool{Pip: true}},
		{"windowsPipenvTest", []string{"c:\\users\\test\\package\\Pipfile"}, map[Technology]bool{Pipenv: true}},
		{"golangTest", []string{"/Users/eco/dev/jfrog-cli-core/go.mod"}, map[Technology]bool{Go: true}},
		{"windowsNugetTest", []string{"c:\\users\\test\\package\\project.sln"}, map[Technology]bool{Nuget: true, Dotnet: true}},
		{"noTechTest", []string{"pomxml"}, map[Technology]bool{}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detectedTech := detectTechnologiesByFilePaths(test.paths, false)
			assert.True(t, reflect.DeepEqual(test.expected, detectedTech), "expected: %s, actual: %s", test.expected, detectedTech)
		})
	}
}

func TestContainsApplicabilityScannableTech(t *testing.T) {
	tests := []struct {
		name         string
		technologies []Technology
		want         bool
	}{
		{name: "contains supported and unsupported techs", technologies: []Technology{Nuget, Go, Npm}, want: true},
		{name: "contains supported techs only", technologies: []Technology{Maven, Yarn, Npm}, want: true},
		{name: "contains unsupported techs only", technologies: []Technology{Dotnet, Nuget, Go}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, ContainsApplicabilityScannableTech(tt.technologies))
		})
	}
}
