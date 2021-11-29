package cisetup

import (
	"github.com/jfrog/jfrog-cli-core/v2/general/techindicators"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConvertBuildCmd(t *testing.T) {
	tests := []buildCmd{
		{"simpleMvn", techindicators.Maven, "mvn clean install", "jfrog rt mvn clean install"},
		{"simpleGradle", techindicators.Gradle, "gradle clean build", "jfrog rt gradle clean build"},
		{"simpleNpmInstall", techindicators.Npm, "npm install", "jfrog rt npmi"},
		{"simpleNpmI", techindicators.Npm, "npm i", "jfrog rt npmi"},
		{"simpleNpmCi", techindicators.Npm, "npm ci", "jfrog rt npmci"},
		{"hiddenMvn", techindicators.Npm, "npm i FOLDERmvnHERE", "jfrog rt npmi FOLDERmvnHERE"},
		{"hiddenNpm", techindicators.Maven, "mvn clean install -f \"HIDDENnpm/pom.xml\"", "jfrog rt mvn clean install -f \"HIDDENnpm/pom.xml\""},
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
	tech     techindicators.Technology
	original string
	expected string
}
