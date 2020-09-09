package commands

import (
	"encoding/json"
	"github.com/jfrog/jfrog-cli-core/utils/config"
	"github.com/jfrog/jfrog-cli-core/utils/log"
	"testing"
)

func TestConfig(t *testing.T) {
	log.SetDefaultLogger()
	expected := &config.BintrayDetails{
		ApiUrl:            "https://api.bintray.com/",
		DownloadServerUrl: "https://dl.bintray.com/",
		User:              "user",
		Key:               "api-key",
		DefPackageLicense: "Apache-2.0"}
	Config(expected, nil, false)
	details, err := GetConfig()
	if err != nil {
		t.Error(err.Error())
	}
	if configStructToString(expected) != configStructToString(details) {
		t.Error("Unexpected configuration was saved to file. Expected: " + configStructToString(expected) + " Got " + configStructToString(details))
	}
}

func configStructToString(config *config.BintrayDetails) string {
	marshaledStruct, _ := json.Marshal(*config)
	return string(marshaledStruct)
}
