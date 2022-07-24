package utils

import (
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestGenerateSarifFileFromScan(t *testing.T) {
	currentScan := services.ScanResponse{
		Vulnerabilities: []services.Vulnerability{
			{
				IssueId:  "XRAY-1",
				Summary:  "summary-1",
				Severity: "9",
				Components: map[string]services.Component{
					"component-G": {FixedVersions: []string{"[2.1.3]"},
						ImpactPaths: nil},
					"component-B": {}},
				Technology: "go",
			},
			{
				IssueId:    "XRAY-2",
				Summary:    "summary-2",
				Severity:   "4.5",
				Components: map[string]services.Component{"component-C": {}, "component-D": {}},
				Technology: "go",
			},
		},
		ScannedPackageType: "Go",
	}
	var scanResults []services.ScanResponse
	scanResults = append(scanResults, currentScan)
	sarif, err := GenerateSarifFileFromScan(scanResults, true, false)
	assert.NoError(t, err)
	expected := "{\n  \"version\": \"2.1.0\",\n  \"$schema\": \"https://json.schemastore.org/sarif-2.1.0-rtm.5.json\",\n  \"runs\": [\n    {\n      \"tool\": {\n        \"driver\": {\n          \"informationUri\": \"https://jfrog.com/xray/\",\n          \"name\": \"JFrog Xray\",\n          \"rules\": [\n            {\n              \"id\": \"XRAY-1\",\n              \"shortDescription\": null,\n              \"fullDescription\": {\n                \"text\": \"summary-1 . Fixed in Versions: [2.1.3]\"\n              },\n              \"properties\": {\n                \"security-severity\": \"9\"\n              }\n            },\n            {\n              \"id\": \"XRAY-2\",\n              \"shortDescription\": null,\n              \"fullDescription\": {\n                \"text\": \"summary-2\"\n              },\n              \"properties\": {\n                \"security-severity\": \"4.5\"\n              }\n            }\n          ]\n        }\n      },\n      \"results\": [\n        {\n          \"ruleId\": \"XRAY-1\",\n          \"ruleIndex\": 0,\n          \"message\": {\n            \"text\": \"component-B:\"\n          },\n          \"locations\": [\n            {\n              \"physicalLocation\": {\n                \"artifactLocation\": {\n                  \"uri\": \"go.mod\"\n                }\n              }\n            }\n          ]\n        },\n        {\n          \"ruleId\": \"XRAY-1\",\n          \"ruleIndex\": 0,\n          \"message\": {\n            \"text\": \"component-G:\"\n          },\n          \"locations\": [\n            {\n              \"physicalLocation\": {\n                \"artifactLocation\": {\n                  \"uri\": \"go.mod\"\n                }\n              }\n            }\n          ]\n        },\n        {\n          \"ruleId\": \"XRAY-2\",\n          \"ruleIndex\": 1,\n          \"message\": {\n            \"text\": \"component-C:\"\n          },\n          \"locations\": [\n            {\n              \"physicalLocation\": {\n                \"artifactLocation\": {\n                  \"uri\": \"go.mod\"\n                }\n              }\n            }\n          ]\n        },\n        {\n          \"ruleId\": \"XRAY-2\",\n          \"ruleIndex\": 1,\n          \"message\": {\n            \"text\": \"component-D:\"\n          },\n          \"locations\": [\n            {\n              \"physicalLocation\": {\n                \"artifactLocation\": {\n                  \"uri\": \"go.mod\"\n                }\n              }\n            }\n          ]\n        }\n      ]\n    }\n  ]\n}"
	assert.Equal(t, expected, sarif)
}
