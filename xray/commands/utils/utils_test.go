package utils

import (
	"github.com/jfrog/jfrog-client-go/xray/scan"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestFilterResultIfNeeded(t *testing.T) {
	// Define test cases
	tests := []struct {
		name       string
		scanResult scan.ScanResponse
		params     ScanGraphParams
		expected   scan.ScanResponse
	}{
		{
			name:       "Should not filter",
			scanResult: scan.ScanResponse{},
			params:     ScanGraphParams{},
			expected:   scan.ScanResponse{},
		},
		{
			name: "No filter level specified",
			scanResult: scan.ScanResponse{
				Violations: []scan.Violation{
					{Severity: "Low"},
					{Severity: "Medium"},
					{Severity: "High"},
					{Severity: "Critical"},
				},
				Vulnerabilities: []scan.Vulnerability{
					{Severity: "Low"},
					{Severity: "Medium"},
					{Severity: "High"},
					{Severity: "Critical"},
				},
			},
			params: ScanGraphParams{
				severityLevel: 0,
			},
			expected: scan.ScanResponse{
				Violations: []scan.Violation{
					{Severity: "Low"},
					{Severity: "Medium"},
					{Severity: "High"},
					{Severity: "Critical"},
				},
				Vulnerabilities: []scan.Vulnerability{
					{Severity: "Low"},
					{Severity: "Medium"},
					{Severity: "High"},
					{Severity: "Critical"},
				},
			},
		},
		{
			name: "Filter violations and vulnerabilities by high severity",
			scanResult: scan.ScanResponse{
				Violations: []scan.Violation{
					{Severity: "Low"},
					{Severity: "Medium"},
					{Severity: "High"},
					{Severity: "Critical"},
				},
				Vulnerabilities: []scan.Vulnerability{
					{Severity: "Low"},
					{Severity: "Medium"},
					{Severity: "High"},
					{Severity: "Critical"},
				},
			},
			params: ScanGraphParams{
				severityLevel: 11,
			},
			expected: scan.ScanResponse{
				Violations: []scan.Violation{
					{Severity: "High"},
					{Severity: "Critical"},
				},
				Vulnerabilities: []scan.Vulnerability{
					{Severity: "High"},
					{Severity: "Critical"},
				},
			},
		},
	}

	// Run test cases
	for i := range tests {
		t.Run(tests[i].name, func(t *testing.T) {
			// Call the function with the input parameters
			actual := filterResultIfNeeded(&tests[i].scanResult, &tests[i].params)
			// Check that the function returned the expected result
			assert.True(t, reflect.DeepEqual(*actual, tests[i].expected))
		})
	}
}

func TestGetFixableComponents(t *testing.T) {
	// create test cases
	testCases := []struct {
		name        string
		components  map[string]scan.Component
		expectedMap map[string]scan.Component
	}{
		{
			name: "Returns an empty map when all components have no fixed versions",
			components: map[string]scan.Component{
				"vuln1": {
					FixedVersions: []string{},
				},
				"vuln2": {
					FixedVersions: []string{},
				},
			},
			expectedMap: map[string]scan.Component{},
		},
		{
			name: "Returns a filtered map with only components that have fixed versions",
			components: map[string]scan.Component{
				"vuln1": {
					FixedVersions: []string{},
				},
				"vuln2": {
					FixedVersions: []string{"1.0.0"},
				},
				"vuln3": {
					FixedVersions: []string{"2.0.0", "3.0.0"},
				},
				"vuln4": {
					FixedVersions: []string{},
				},
			},
			expectedMap: map[string]scan.Component{
				"vuln2": {
					FixedVersions: []string{"1.0.0"},
				},
				"vuln3": {
					FixedVersions: []string{"2.0.0", "3.0.0"},
				},
			},
		},
	}

	// run test cases
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			actualMap := getFixableComponents(tc.components)
			assert.Equal(t, len(tc.expectedMap), len(actualMap))
			for k, v := range tc.expectedMap {
				if v.FixedVersions == nil {
					assert.True(t, actualMap[k].FixedVersions == nil)
				} else {
					assert.Equal(t, len(actualMap[k].FixedVersions), len(v.FixedVersions))
				}
			}
		})
	}
}
