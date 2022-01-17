package coreutils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTechIndicator(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		expected Technology
	}{
		{"simpleMavenTest", "pom.xml", Maven},
		{"npmTest", "../package.json", Npm},
		{"windowsGradleTest", "c://users/test/package/build.gradle", Gradle},
		{"windowsPipTest", "c://users/test/package/setup.py", Pip},
		{"windowsPipenvTest", "c://users/test/package/pipfile", Pipenv},
		{"windowsGolangTest", "c://users/test/package/go.mod", Go},
		{"noTechTest", "pomxml", ""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			detectedTech := detectTechnologyByFile(test.filePath, false)
			assert.Equal(t, test.expected, detectedTech)
		})
	}
}
