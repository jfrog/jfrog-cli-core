package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/xray/formats"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGenerateSarifFileFromScan(t *testing.T) {
	currentScan := services.ScanResponse{
		Vulnerabilities: []services.Vulnerability{
			{
				IssueId: "XRAY-1",
				Summary: "summary-1",
				Cves: []services.Cve{
					{
						Id:          "CVE-2022-0000",
						CvssV3Score: "9",
					},
				},
				Components: map[string]services.Component{
					"component-G": {
						FixedVersions: []string{"[2.1.3]"},
						ImpactPaths:   nil,
					},
				},
				Technology: "go",
			},
		},
		ScannedPackageType: "Go",
	}
	var scanResults []services.ScanResponse
	scanResults = append(scanResults, currentScan)
	sarif, err := GenerateSarifFileFromScan(scanResults, false, false)
	assert.NoError(t, err)
	expected := "{\"version\":\"2.1.0\",\"$schema\":\"https://json.schemastore.org/sarif-2.1.0-rtm.5.json\",\"runs\":[{\"tool\":{\"driver\":{\"informationUri\":\"https://jfrog.com/xray/\",\"name\":\"JFrog Xray\",\"rules\":[{\"id\":\"XRAY-1\",\"shortDescription\":null,\"help\":{\"markdown\":\"summary-1\"},\"properties\":{\"security-severity\":\"9.0\"}}]}},\"results\":[{\"ruleId\":\"XRAY-1\",\"ruleIndex\":0,\"message\":{\"text\":\"[CVE-2022-0000] Upgrade component-G: to [2.1.3]\"},\"locations\":[{\"physicalLocation\":{\"artifactLocation\":{\"uri\":\"go.mod\"}}}]}]}]}"
	assert.JSONEq(t, expected, sarif)

	sarif, err = GenerateSarifFileFromScan(scanResults, false, true)
	assert.NoError(t, err)
	expected = "{\n  \"version\": \"2.1.0\",\n  \"$schema\": \"https://json.schemastore.org/sarif-2.1.0-rtm.5.json\",\n  \"runs\": [\n    {\n      \"tool\": {\n        \"driver\": {\n          \"informationUri\": \"https://jfrog.com/xray/\",\n          \"name\": \"JFrog Xray\",\n          \"rules\": [\n            {\n              \"id\": \"XRAY-1\",\n              \"shortDescription\": null,\n              \"help\": {\n                \"markdown\": \"| Severity Score | Direct Dependencies | Fixed Versions     |\\n| :---        |    :----:   |          ---: |\\n| 9.0      |        | [2.1.3]   |\"\n              },\n              \"properties\": {\n                \"security-severity\": \"9.0\"\n              }\n            }\n          ]\n        }\n      },\n      \"results\": [\n        {\n          \"ruleId\": \"XRAY-1\",\n          \"ruleIndex\": 0,\n          \"message\": {\n            \"text\": \"[CVE-2022-0000] Upgrade component-G: to [2.1.3]\"\n          },\n          \"locations\": [\n            {\n              \"physicalLocation\": {\n                \"artifactLocation\": {\n                  \"uri\": \"go.mod\"\n                }\n              }\n            }\n          ]\n        }\n      ]\n    }\n  ]\n}"
	assert.JSONEq(t, expected, sarif)
}

func TestGetHeadline(t *testing.T) {
	assert.Equal(t, "[CVE-2022-1234] Upgrade loadsh:1.4.1 to 2.0.0", getHeadline("loadsh", "1.4.1", "CVE-2022-1234", "2.0.0"))
	assert.NotEqual(t, "[CVE-2022-1234] Upgrade loadsh:1.4.1 to 2.0.0", getHeadline("loadsh", "1.2.1", "CVE-2022-1234", "2.0.0"))
}

func TestGetCves(t *testing.T) {
	issueId := "XRAY-123456"
	cvesRow := []formats.CveRow{{Id: "CVE-2022-1234"}}
	assert.Equal(t, "CVE-2022-1234", getCves(cvesRow, issueId))
	cvesRow = append(cvesRow, formats.CveRow{Id: "CVE-2019-1234"})
	assert.Equal(t, "CVE-2022-1234, CVE-2019-1234", getCves(cvesRow, issueId))
	assert.Equal(t, issueId, getCves(nil, issueId))
}
