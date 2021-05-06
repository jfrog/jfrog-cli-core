package cisetup

/*
func TestConvertBuildCmd(t *testing.T) {
	tests := []buildCmd{
		{"simpleMvn", "mvn clean install", "jfrog rt mvn clean install"},
		{"simpleGradle", "gradle clean build", "jfrog rt gradle clean build"},
		{"simpleNpmInstall", "npm install", "jfrog rt npmi"},
		{"simpleNpmI", "npm i", "jfrog rt npmi"},
		{"simpleNpmCi", "npm ci", "jfrog rt npmci"},
		{"complexMvnGradle", "mvn clean install && gradle clean build", "jfrog rt mvn clean install && jfrog rt gradle clean build"},
		{"hiddenMvn", "npm i FOLDERmvnHERE", "jfrog rt npmi FOLDERmvnHERE"},
		{"complexNpm", "gradle clean build && npm i && npm ci", "jfrog rt gradle clean build && jfrog rt npmi && jfrog rt npmci"},
		{"hiddenNpm", "mvn clean install -f \"HIDDENnpm/pom.xml\" && gradle clean build", "jfrog rt mvn clean install -f \"HIDDENnpm/pom.xml\" && jfrog rt gradle clean build"},
	}

	generator := JFrogPipelinesYamlGenerator{SetupData: &CiSetupData{}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			generator.SetupData.BuildCommand = test.original
			converted, err := generator.convertBuildCmd()
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
	original string
	expected string
}
*/ // todo am i still needed?