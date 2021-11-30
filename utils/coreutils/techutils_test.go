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
		{"windowsPipTest", "c://users/test/package/setup.py", Pypi},
		{"windowsGolangTest", "c://users/test/package/go.mod", Go},
		{"noTechTest", "pomxml", ""},
	}
	indicators := GetTechIndicators()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var detectedTech Technology
			for _, indicator := range indicators {
				if indicator.Indicates(test.filePath) {
					detectedTech = indicator.GetTechnology()
					break
				}
			}
			assert.Equal(t, test.expected, detectedTech)
		})
	}
}
