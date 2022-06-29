package offlineupdate

import (
	"github.com/magiconair/properties/assert"
	"io/ioutil"
	"path/filepath"
	"strings"
	"testing"
)

func TestCreateXrayFileNameFromUrl(t *testing.T) {
	tests := []struct {
		url      string
		fileName string
	}{
		{url: "https://dl.bintray.com/jfrog/jxray-data-dumps/2018-05/onboarding__vuln9__.json?expiry=1528473414900&id=K8v%2BJBItDfdcU9%2BBa2lxhmJRitQZPWsH89MtXs3pYfWKvRUwGNuUB8vcHv7EvJydaJGrwKm%2B%2FYAIAjMR3TaTm5VIRouiChTABPYbDNTNf%2F4%3D&signature=ePBfZuVZBljVvQTFHIpPH6lo7Qby%2Ban44resdLMo5f16W%2FX4Ou6gOleBHo5ViyWFy1tnFoPopk1XIQgB918ZFg%3D%3D", fileName: "2018-05__onboarding__vuln9__.json"},
		{url: "https://dl.bintray.com/jfrog/jxray-data-dumps/2018-06-07/onboarding__vulnR1_1__.zip?expiry=1528711288481&id=K8v%2BJBItDfdcU9%2BBa2lxhmJRitQZPWsH89MtXs3pYfWKvRUwGNuUB8vcHv7EvJyd3g6UkTiZXV2mGxN1HD6KtovwjhKr4IdWuYKciRkl1agY487O8kk4jIOibc7paR2t&signature=eiB%2FcOMjWKjJdSybFr%2BPo56FmusgHvzvRMTnHSuHwIWvY5QnX2dIumsbIlMaVvL9xzoQObWHJ%2FMZyWnTVcv67g%3D%3D", fileName: "2018-06-07__onboarding__vulnR1_1__.zip"},
		{url: "https://dl.bintray.com/jfrog/jxray-data-dumps/2018-05/onboarding__vulnR1_16__.zip?expiry=1528711287386&id=K8v%2BJBItDfdcU9%2BBa2lxhmJRitQZPWsH89MtXs3pYfWKvRUwGNuUB8vcHv7EvJydaJGrwKm%2B%2FYAIAjMR3TaTm9Wd2NqK5BiRQc33JGl4b0DZ9e%2B1cTtPyVtm5jxX9kIL&signature=HQKhgmRBtvJ1EwomgggR47M9TAWSegvWFUw9NItI5Cj22EZ2BqbhxIfcVAti8WJTjsKfAS2ap7yb%2BBmQilnSng%3D%3D", fileName: "2018-05__onboarding__vulnR1_16__.zip"},
		{url: "https://dl.bintray.com/jfrog/jxray-data-dumps/2018-06-07/onboarding__vulnS1_1__.zip?expiry=1528711288397&id=K8v%2BJBItDfdcU9%2BBa2lxhmJRitQZPWsH89MtXs3pYfWKvRUwGNuUB8vcHv7EvJyd3g6UkTiZXV2mGxN1HD6KtozvQ8zhzPTSbLjftsv4zhgY487O8kk4jIOibc7paR2t&signature=P9XPWugJkqz5ekpEQrOkAbIsogAB7EOgq7ou1%2BTAPSOFfsKA9j1I%2Fhj94y%2BoIipYRtUUtSGCqXyRP%2B%2BG%2FscwmA%3D%3D", fileName: "2018-06-07__onboarding__vulnS1_1__.zip"},
	}

	for _, test := range tests {
		fileName, err := createXrayFileNameFromUrl(test.url)
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, fileName, test.fileName)
	}
}

// DBSync V3 test data

var periodicUpdateResponse = "[{\"download_url\":\"some_url_to_package_update\",\"timestamp\":1234}]"
var periodicDeletionResponse = "[{\"download_url\":\"some_url_to_package_delete\",\"timestamp\":1234}]"
var periodicUpdateResponseSection = "\"update\":" + periodicUpdateResponse
var periodicDeleteResponseSection = "\"deletion\":" + periodicDeletionResponse

var periodicResponse = "{" + periodicUpdateResponseSection + "," + periodicDeleteResponseSection + "}"
var onboardingResponse = "[{\"download_url\":\"some_url_to_package_onboard\",\"timestamp\":1234}]"

func TestDBSyncV3BuildURL(t *testing.T) {
	tests := []struct {
		isPeriodic bool
		expected   string
	}{
		{true, "api/v3/updates/periodic"}, {false, "api/v3/updates/onboarding"},
	}
	for _, test := range tests {
		url := buildUrlDBSyncV3(test.isPeriodic)
		assert.Equal(t, strings.HasSuffix(url, test.expected), true)
	}
}

func TestDBSyncV3getURLsToDownload(t *testing.T) {
	tests := []struct {
		serverResponse []byte
		isPeriodic     bool
		expected       []string
	}{
		{[]byte(periodicResponse), true, []string{"some_url_to_package_update", "some_url_to_package_delete"}},
		{[]byte(onboardingResponse), false, []string{"some_url_to_package_onboard"}},
	}

	for _, test := range tests {
		urls, err := getURLsToDownloadDBSyncV3(test.serverResponse, test.isPeriodic)
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, urls, test.expected)
	}
}

func TestDBSyncV3createXrayFileNameFromURL(t *testing.T) {
	tests := []struct {
		url      string
		expected string
	}{{"a/b/c/d.zip", "d.zip"}, {"x/y.zip", "y.zip"}}

	for _, test := range tests {
		expected, err := createXrayFileNameFromUrlV3(test.url)
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, expected, test.expected)
	}
}

func TestDBSyncV3createV3MetadataFile(t *testing.T) {
	tests := []struct {
		serverResponse   []byte
		state            string
		expectedFilename string
	}{
		{[]byte(periodicResponse), periodicState, periodicState + ".json"},
		{[]byte(onboardingResponse), onboardingState, onboardingState + ".json"},
	}

	for _, test := range tests {
		dir := t.TempDir()
		err := createV3MetadataFile(test.state, test.serverResponse, dir)
		if err != nil {
			t.Error(err)
		}
		fileContent, err := ioutil.ReadFile(filepath.Join(dir, test.expectedFilename))
		if err != nil {
			t.Error(err)
		}
		assert.Equal(t, fileContent, test.serverResponse)
	}

}
