package cisetup

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertBuildCmd(t *testing.T) {
	tests := []buildCmd{
		{"simpleMvn", Maven, "mvn clean install", "jfrog rt mvn clean install"},
		{"simpleGradle", Gradle, "gradle clean build", "jfrog rt gradle clean build"},
		{"simpleNpmInstall", Npm, "npm install", "jfrog rt npmi"},
		{"simpleNpmI", Npm, "npm i", "jfrog rt npmi"},
		{"simpleNpmCi", Npm, "npm ci", "jfrog rt npmci"},
		{"hiddenMvn", Npm, "npm i FOLDERmvnHERE", "jfrog rt npmi FOLDERmvnHERE"},
		{"hiddenNpm", Maven, "mvn clean install -f \"HIDDENnpm/pom.xml\"", "jfrog rt mvn clean install -f \"HIDDENnpm/pom.xml\""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := &CiSetupData{}
			data.BuiltTechnology = &TechnologyInfo{TechnologyType: test.tech, BuildCmd: test.original}
			converted, err := convertBuildCmd(data)
			if err != nil {
				assert.NoError(t, err)
				return
			}
			assert.Equal(t, test.expected, converted)
		})
	}
}

type buildCmd struct {
	name     string
	tech     Technology
	original string
	expected string
}
