package utils

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/formats"
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
	"path"
	"testing"
)

func TestGenerateSarifFileFromScan(t *testing.T) {
	extendedResults := &ExtendedScanResults{
		XrayResults: []services.ScanResponse{
			{
				Vulnerabilities: []services.Vulnerability{
					{
						Cves:     []services.Cve{{Id: "CVE-2022-1234", CvssV3Score: "8.0"}, {Id: "CVE-2023-1234", CvssV3Score: "7.1"}},
						Summary:  "A test vulnerability the harms nothing",
						Severity: "High",
						Components: map[string]services.Component{
							"vulnerability1": {FixedVersions: []string{"1.2.3"}},
						},
						Technology: coreutils.Go.ToString(),
					},
				},
			},
		},
		SecretsScanResults: []IacOrSecretResult{
			{
				Severity:   "Medium",
				File:       "found_secrets.js",
				LineColumn: "1:18",
				Type:       "entropy",
				Text:       "AAA************",
			},
		},
		IacScanResults: []IacOrSecretResult{
			{
				Severity:   "Medium",
				File:       "plan/nonapplicable/req_sw_terraform_azure_compute_no_pass_auth.json",
				LineColumn: "229:38",
				Type:       "entropy",
				Text:       "BBB************",
			},
		},
	}
	testCases := []struct {
		name                string
		extendedResults     *ExtendedScanResults
		isMultipleRoots     bool
		markdownOutput      bool
		expectedSarifOutput string
	}{
		{
			name:                "Scan results with vulnerabilities, secrets and IaC",
			extendedResults:     extendedResults,
			expectedSarifOutput: "{\n  \"version\": \"2.1.0\",\n  \"$schema\": \"https://json.schemastore.org/sarif-2.1.0-rtm.5.json\",\n  \"runs\": [\n    {\n      \"tool\": {\n        \"driver\": {\n          \"informationUri\": \"https://example.com/\",\n          \"name\": \"JFrog Security\",\n          \"rules\": [\n            {\n              \"id\": \"CVE-2022-1234, CVE-2023-1234\",\n              \"shortDescription\": {\n                \"text\": \"A test vulnerability the harms nothing\"\n              },\n              \"help\": {\n                \"markdown\": \"\"\n              },\n              \"properties\": {\n                \"security-severity\": \"8.0\"\n              }\n            },\n            {\n              \"id\": \"found_secrets.js\",\n              \"shortDescription\": {\n                \"text\": \"AAA************\"\n              },\n              \"help\": {\n                \"markdown\": \"\"\n              },\n              \"properties\": {\n                \"security-severity\": \"6.9\"\n              }\n            },\n            {\n              \"id\": \"plan/nonapplicable/req_sw_terraform_azure_compute_no_pass_auth.json\",\n              \"shortDescription\": {\n                \"text\": \"BBB************\"\n              },\n              \"help\": {\n                \"markdown\": \"\"\n              },\n              \"properties\": {\n                \"security-severity\": \"6.9\"\n              }\n            }\n          ]\n        }\n      },\n      \"results\": [\n        {\n          \"ruleId\": \"CVE-2022-1234, CVE-2023-1234\",\n          \"ruleIndex\": 0,\n          \"message\": {\n            \"text\": \"[CVE-2022-1234, CVE-2023-1234] vulnerability1 \"\n          },\n          \"locations\": [\n            {\n              \"physicalLocation\": {\n                \"artifactLocation\": {\n                  \"uri\": \"go.mod\"\n                },\n                \"region\": {\n                  \"startLine\": 0,\n                  \"startColumn\": 0,\n                  \"endLine\": 0\n                }\n              }\n            }\n          ]\n        },\n        {\n          \"ruleId\": \"found_secrets.js\",\n          \"ruleIndex\": 1,\n          \"message\": {\n            \"text\": \"Potential Secret Exposed\"\n          },\n          \"locations\": [\n            {\n              \"physicalLocation\": {\n                \"artifactLocation\": {\n                  \"uri\": \"found_secrets.js\"\n                },\n                \"region\": {\n                  \"startLine\": 1,\n                  \"startColumn\": 18,\n                  \"endLine\": 1\n                }\n              }\n            }\n          ]\n        },\n        {\n          \"ruleId\": \"plan/nonapplicable/req_sw_terraform_azure_compute_no_pass_auth.json\",\n          \"ruleIndex\": 2,\n          \"message\": {\n            \"text\": \"Infrastructure as Code Vulnerability\"\n          },\n          \"locations\": [\n            {\n              \"physicalLocation\": {\n                \"artifactLocation\": {\n                  \"uri\": \"plan/nonapplicable/req_sw_terraform_azure_compute_no_pass_auth.json\"\n                },\n                \"region\": {\n                  \"startLine\": 229,\n                  \"startColumn\": 38,\n                  \"endLine\": 229\n                }\n              }\n            }\n          ]\n        }\n      ]\n    }\n  ]\n}",
		},
		{
			name:                "Scan results with vulnerabilities, secrets and IaC as Markdown",
			extendedResults:     extendedResults,
			markdownOutput:      true,
			expectedSarifOutput: "{\n  \"version\": \"2.1.0\",\n  \"$schema\": \"https://json.schemastore.org/sarif-2.1.0-rtm.5.json\",\n  \"runs\": [\n    {\n      \"tool\": {\n        \"driver\": {\n          \"informationUri\": \"https://example.com/\",\n          \"name\": \"JFrog Security\",\n          \"rules\": [\n            {\n              \"id\": \"CVE-2022-1234, CVE-2023-1234\",\n              \"shortDescription\": {\n                \"text\": \"\"\n              },\n              \"help\": {\n                \"markdown\": \"| Severity Score | Direct Dependencies | Fixed Versions     |\\n| :---:        |    :----:   |          :---: |\\n| 8.0      |        | 1.2.3   |\\n\"\n              },\n              \"properties\": {\n                \"security-severity\": \"8.0\"\n              }\n            },\n            {\n              \"id\": \"found_secrets.js\",\n              \"shortDescription\": {\n                \"text\": \"\"\n              },\n              \"help\": {\n                \"markdown\": \"| Severity | File | Line:Column | Secret |\\n| :---: | :---: | :---: | :---: |\\n| Medium | found_secrets.js | 1:18 | AAA************ |\"\n              },\n              \"properties\": {\n                \"security-severity\": \"6.9\"\n              }\n            },\n            {\n              \"id\": \"plan/nonapplicable/req_sw_terraform_azure_compute_no_pass_auth.json\",\n              \"shortDescription\": {\n                \"text\": \"\"\n              },\n              \"help\": {\n                \"markdown\": \"| Severity | File | Line:Column | Finding |\\n| :---: | :---: | :---: | :---: |\\n| Medium | plan/nonapplicable/req_sw_terraform_azure_compute_no_pass_auth.json | 229:38 | BBB************ |\"\n              },\n              \"properties\": {\n                \"security-severity\": \"6.9\"\n              }\n            }\n          ]\n        }\n      },\n      \"results\": [\n        {\n          \"ruleId\": \"CVE-2022-1234, CVE-2023-1234\",\n          \"ruleIndex\": 0,\n          \"message\": {\n            \"text\": \"[CVE-2022-1234, CVE-2023-1234] vulnerability1 \"\n          },\n          \"locations\": [\n            {\n              \"physicalLocation\": {\n                \"artifactLocation\": {\n                  \"uri\": \"go.mod\"\n                },\n                \"region\": {\n                  \"startLine\": 0,\n                  \"startColumn\": 0,\n                  \"endLine\": 0\n                }\n              }\n            }\n          ]\n        },\n        {\n          \"ruleId\": \"found_secrets.js\",\n          \"ruleIndex\": 1,\n          \"message\": {\n            \"text\": \"Potential Secret Exposed\"\n          },\n          \"locations\": [\n            {\n              \"physicalLocation\": {\n                \"artifactLocation\": {\n                  \"uri\": \"found_secrets.js\"\n                },\n                \"region\": {\n                  \"startLine\": 1,\n                  \"startColumn\": 18,\n                  \"endLine\": 1\n                }\n              }\n            }\n          ]\n        },\n        {\n          \"ruleId\": \"plan/nonapplicable/req_sw_terraform_azure_compute_no_pass_auth.json\",\n          \"ruleIndex\": 2,\n          \"message\": {\n            \"text\": \"Infrastructure as Code Vulnerability\"\n          },\n          \"locations\": [\n            {\n              \"physicalLocation\": {\n                \"artifactLocation\": {\n                  \"uri\": \"plan/nonapplicable/req_sw_terraform_azure_compute_no_pass_auth.json\"\n                },\n                \"region\": {\n                  \"startLine\": 229,\n                  \"startColumn\": 38,\n                  \"endLine\": 229\n                }\n              }\n            }\n          ]\n        }\n      ]\n    }\n  ]\n}",
		},
		{
			name:                "Scan results without vulnerabilities",
			extendedResults:     &ExtendedScanResults{},
			isMultipleRoots:     true,
			markdownOutput:      true,
			expectedSarifOutput: "{\n  \"version\": \"2.1.0\",\n  \"$schema\": \"https://json.schemastore.org/sarif-2.1.0-rtm.5.json\",\n  \"runs\": [\n    {\n      \"tool\": {\n        \"driver\": {\n          \"informationUri\": \"https://example.com/\",\n          \"name\": \"JFrog Security\",\n          \"rules\": []\n        }\n      },\n      \"results\": []\n    }\n  ]\n}",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			sarifOutput, err := GenerateSarifFileFromScan(testCase.extendedResults, testCase.isMultipleRoots, testCase.markdownOutput, "JFrog Security", "https://example.com/")
			assert.NoError(t, err)
			assert.Equal(t, testCase.expectedSarifOutput, sarifOutput)
		})
	}
}

func TestGetVulnerabilityOrViolationSarifHeadline(t *testing.T) {
	assert.Equal(t, "[CVE-2022-1234] loadsh 1.4.1", getVulnerabilityOrViolationSarifHeadline("loadsh", "1.4.1", "CVE-2022-1234"))
	assert.NotEqual(t, "[CVE-2022-1234] loadsh 1.4.1", getVulnerabilityOrViolationSarifHeadline("loadsh", "1.2.1", "CVE-2022-1234"))
}

func TestGetCves(t *testing.T) {
	issueId := "XRAY-123456"
	cvesRow := []formats.CveRow{{Id: "CVE-2022-1234"}}
	assert.Equal(t, "CVE-2022-1234", getCves(cvesRow, issueId))
	cvesRow = append(cvesRow, formats.CveRow{Id: "CVE-2019-1234"})
	assert.Equal(t, "CVE-2022-1234, CVE-2019-1234", getCves(cvesRow, issueId))
	assert.Equal(t, issueId, getCves(nil, issueId))
}

func TestGetIacOrSecretsProperties(t *testing.T) {
	testCases := []struct {
		name           string
		secretOrIac    formats.IacSecretsRow
		markdownOutput bool
		isSecret       bool
		expectedOutput sarifProperties
	}{
		{
			name: "Infrastructure as Code vulnerability without markdown output",
			secretOrIac: formats.IacSecretsRow{
				Severity:   "high",
				File:       path.Join("path", "to", "file"),
				LineColumn: "10:5",
				Text:       "Vulnerable code",
				Type:       "Terraform",
			},
			markdownOutput: false,
			isSecret:       false,
			expectedOutput: sarifProperties{
				Applicable:          "",
				Cves:                "",
				Headline:            "Infrastructure as Code Vulnerability",
				Severity:            "8.9",
				Description:         "Vulnerable code",
				MarkdownDescription: "",
				XrayID:              "",
				File:                path.Join("path", "to", "file"),
				LineColumn:          "10:5",
				SecretsOrIacType:    "Terraform",
			},
		},
		{
			name: "Potential secret exposed with markdown output",
			secretOrIac: formats.IacSecretsRow{
				Severity:   "medium",
				File:       path.Join("path", "to", "file"),
				LineColumn: "5:3",
				Text:       "Potential secret",
				Type:       "AWS Secret Manager",
			},
			markdownOutput: true,
			isSecret:       true,
			expectedOutput: sarifProperties{
				Applicable:          "",
				Cves:                "",
				Headline:            "Potential Secret Exposed",
				Severity:            "6.9",
				Description:         "Potential secret",
				MarkdownDescription: fmt.Sprintf("| Severity | File | Line:Column | Secret |\n| :---: | :---: | :---: | :---: |\n| medium | %s | 5:3 | Potential secret |", path.Join("path", "to", "file")),
				XrayID:              "",
				File:                path.Join("path", "to", "file"),
				LineColumn:          "5:3",
				SecretsOrIacType:    "AWS Secret Manager",
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			output := getIacOrSecretsProperties(testCase.secretOrIac, testCase.markdownOutput, testCase.isSecret)
			assert.Equal(t, testCase.expectedOutput.Applicable, output.Applicable)
			assert.Equal(t, testCase.expectedOutput.Cves, output.Cves)
			assert.Equal(t, testCase.expectedOutput.Headline, output.Headline)
			assert.Equal(t, testCase.expectedOutput.Severity, output.Severity)
			assert.Equal(t, testCase.expectedOutput.Description, output.Description)
			assert.Equal(t, testCase.expectedOutput.MarkdownDescription, output.MarkdownDescription)
			assert.Equal(t, testCase.expectedOutput.XrayID, output.XrayID)
			assert.Equal(t, testCase.expectedOutput.File, output.File)
			assert.Equal(t, testCase.expectedOutput.LineColumn, output.LineColumn)
			assert.Equal(t, testCase.expectedOutput.SecretsOrIacType, output.SecretsOrIacType)
		})
	}
}

func TestGetViolatedDepsSarifProps(t *testing.T) {
	testCases := []struct {
		name           string
		vulnerability  formats.VulnerabilityOrViolationRow
		markdownOutput bool
		expectedOutput sarifProperties
	}{
		{
			name: "Vulnerability with markdown output",
			vulnerability: formats.VulnerabilityOrViolationRow{
				Summary:                   "Vulnerable dependency",
				Severity:                  "high",
				Applicable:                "Applicable",
				ImpactedDependencyName:    "example-package",
				ImpactedDependencyVersion: "1.0.0",
				ImpactedDependencyType:    "npm",
				FixedVersions:             []string{"1.0.1", "1.0.2"},
				Components: []formats.ComponentRow{
					{Name: "example-package", Version: "1.0.0"},
				},
				Cves: []formats.CveRow{
					{Id: "CVE-2021-1234", CvssV3: "7.2"},
					{Id: "CVE-2021-5678", CvssV3: "7.2"},
				},
				IssueId: "XRAY-12345",
			},
			markdownOutput: true,
			expectedOutput: sarifProperties{
				Applicable:          "Applicable",
				Cves:                "CVE-2021-1234, CVE-2021-5678",
				Headline:            "[CVE-2021-1234, CVE-2021-5678] example-package 1.0.0",
				Severity:            "7.2",
				Description:         "Vulnerable dependency",
				MarkdownDescription: "| Severity Score | Contextual Analysis | Direct Dependencies | Fixed Versions     |\n|  :---:  |  :---:  |  :---:  |  :---:  |\n| 7.2      | Applicable       | `example-package 1.0.0`       | 1.0.1, 1.0.2   |\n",
			},
		},
		{
			name: "Vulnerability without markdown output",
			vulnerability: formats.VulnerabilityOrViolationRow{
				Summary:                   "Vulnerable dependency",
				Severity:                  "high",
				Applicable:                "Applicable",
				ImpactedDependencyName:    "example-package",
				ImpactedDependencyVersion: "1.0.0",
				ImpactedDependencyType:    "npm",
				FixedVersions:             []string{"1.0.1", "1.0.2"},
				Components: []formats.ComponentRow{
					{Name: "example-package", Version: "1.0.0"},
				},
				Cves: []formats.CveRow{
					{Id: "CVE-2021-1234", CvssV3: "7.2"},
					{Id: "CVE-2021-5678", CvssV3: "7.2"},
				},
				IssueId: "XRAY-12345",
			},
			expectedOutput: sarifProperties{
				Applicable:          "Applicable",
				Cves:                "CVE-2021-1234, CVE-2021-5678",
				Headline:            "[CVE-2021-1234, CVE-2021-5678] example-package 1.0.0",
				Severity:            "7.2",
				Description:         "Vulnerable dependency",
				MarkdownDescription: "",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := getViolatedDepsSarifProps(tc.vulnerability, tc.markdownOutput)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedOutput.Cves, output.Cves)
			assert.Equal(t, tc.expectedOutput.Severity, output.Severity)
			assert.Equal(t, tc.expectedOutput.XrayID, output.XrayID)
			assert.Equal(t, tc.expectedOutput.MarkdownDescription, output.MarkdownDescription)
			assert.Equal(t, tc.expectedOutput.Applicable, output.Applicable)
			assert.Equal(t, tc.expectedOutput.Description, output.Description)
			assert.Equal(t, tc.expectedOutput.Headline, output.Headline)
		})
	}
}

func TestGetDirectDependenciesFormatted(t *testing.T) {
	testCases := []struct {
		name           string
		directDeps     []formats.ComponentRow
		expectedOutput string
	}{
		{
			name: "Single direct dependency",
			directDeps: []formats.ComponentRow{
				{Name: "example-package", Version: "1.0.0"},
			},
			expectedOutput: "`example-package 1.0.0`",
		},
		{
			name: "Multiple direct dependencies",
			directDeps: []formats.ComponentRow{
				{Name: "dependency1", Version: "1.0.0"},
				{Name: "dependency2", Version: "2.0.0"},
			},
			expectedOutput: "`dependency1 1.0.0`<br/>`dependency2 2.0.0`",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := getDirectDependenciesFormatted(tc.directDeps)
			assert.NoError(t, err)
			assert.Equal(t, tc.expectedOutput, output)
		})
	}
}

func TestGetSarifTableDescription(t *testing.T) {
	testCases := []struct {
		name                string
		formattedDeps       string
		maxCveScore         string
		applicable          string
		fixedVersions       []string
		expectedDescription string
	}{
		{
			name:                "Applicable vulnerability",
			formattedDeps:       "`example-package 1.0.0`",
			maxCveScore:         "7.5",
			applicable:          "Applicable",
			fixedVersions:       []string{"1.0.1", "1.0.2"},
			expectedDescription: "| Severity Score | Contextual Analysis | Direct Dependencies | Fixed Versions     |\n|  :---:  |  :---:  |  :---:  |  :---:  |\n| 7.5      | Applicable       | `example-package 1.0.0`       | 1.0.1, 1.0.2   |",
		},
		{
			name:                "Non-applicable vulnerability",
			formattedDeps:       "`example-package 2.0.0`",
			maxCveScore:         "6.2",
			applicable:          "",
			fixedVersions:       []string{"2.0.1"},
			expectedDescription: "| Severity Score | Direct Dependencies | Fixed Versions     |\n| :---:        |    :----:   |          :---: |\n| 6.2      | `example-package 2.0.0`       | 2.0.1   |",
		},
		{
			name:                "No fixed versions",
			formattedDeps:       "`example-package 3.0.0`",
			maxCveScore:         "3.0",
			applicable:          "",
			fixedVersions:       []string{},
			expectedDescription: "| Severity Score | Direct Dependencies | Fixed Versions     |\n| :---:        |    :----:   |          :---: |\n| 3.0      | `example-package 3.0.0`       | No fix available   |",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output := getSarifTableDescription(tc.formattedDeps, tc.maxCveScore, tc.applicable, tc.fixedVersions)
			assert.Equal(t, tc.expectedDescription, output)
		})
	}
}

func TestFindMaxCVEScore(t *testing.T) {
	testCases := []struct {
		name           string
		cves           []formats.CveRow
		expectedOutput string
		expectedError  bool
	}{
		{
			name: "CVEScore with valid float values",
			cves: []formats.CveRow{
				{Id: "CVE-2021-1234", CvssV3: "7.5"},
				{Id: "CVE-2021-5678", CvssV3: "9.2"},
			},
			expectedOutput: "9.2",
		},
		{
			name: "CVEScore with invalid float value",
			cves: []formats.CveRow{
				{Id: "CVE-2022-4321", CvssV3: "invalid"},
			},
			expectedOutput: "",
			expectedError:  true,
		},
		{
			name:           "CVEScore without values",
			cves:           []formats.CveRow{},
			expectedOutput: "0.0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			output, err := findMaxCVEScore(tc.cves)
			assert.False(t, tc.expectedError && err == nil)
			assert.Equal(t, tc.expectedOutput, output)
		})
	}
}
