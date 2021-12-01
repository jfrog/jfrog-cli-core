package cisetup

import (
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
)

func TestConvertBuildCmd(t *testing.T) {
	tests := []buildCmd{
		{"simpleMvn", coreutils.Maven, "mvn clean install", "jfrog rt mvn clean install"},
		{"simpleGradle", coreutils.Gradle, "gradle clean build", "jfrog rt gradle clean build"},
		{"simpleNpmInstall", coreutils.Npm, "npm install", "jfrog rt npmi"},
		{"simpleNpmI", coreutils.Npm, "npm i", "jfrog rt npmi"},
		{"simpleNpmCi", coreutils.Npm, "npm ci", "jfrog rt npmci"},
		{"hiddenMvn", coreutils.Npm, "npm i FOLDERmvnHERE", "jfrog rt npmi FOLDERmvnHERE"},
		{"hiddenNpm", coreutils.Maven, "mvn clean install -f \"HIDDENnpm/pom.xml\"", "jfrog rt mvn clean install -f \"HIDDENnpm/pom.xml\""},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			data := &CiSetupData{}
			data.BuiltTechnology = &TechnologyInfo{Type: test.tech, BuildCmd: test.original}
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
	tech     coreutils.Technology
	original string
	expected string
}
