package utils

import (
	"github.com/jfrog/jfrog-client-go/xray/services"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"
)

func TestFilterResultIfNeeded(t *testing.T) {
	// Define test cases
	tests := []struct {
		name       string
		scanResult services.ScanResponse
		params     ScanGraphParams
		expected   services.ScanResponse
	}{
		{
			name:       "Should not filter",
			scanResult: services.ScanResponse{},
			params:     ScanGraphParams{},
			expected:   services.ScanResponse{},
		},
		{
			name: "No filter level specified",
			scanResult: services.ScanResponse{
				Violations: []services.Violation{
					{Severity: "Low"},
					{Severity: "Medium"},
					{Severity: "High"},
					{Severity: "Critical"},
				},
				Vulnerabilities: []services.Vulnerability{
					{Severity: "Low"},
					{Severity: "Medium"},
					{Severity: "High"},
					{Severity: "Critical"},
				},
			},
			params: ScanGraphParams{
				severityLevel: 0,
			},
			expected: services.ScanResponse{
				Violations: []services.Violation{
					{Severity: "Low"},
					{Severity: "Medium"},
					{Severity: "High"},
					{Severity: "Critical"},
				},
				Vulnerabilities: []services.Vulnerability{
					{Severity: "Low"},
					{Severity: "Medium"},
					{Severity: "High"},
					{Severity: "Critical"},
				},
			},
		},
		{
			name: "Filter violations and vulnerabilities by high severity",
			scanResult: services.ScanResponse{
				Violations: []services.Violation{
					{Severity: "Low"},
					{Severity: "Medium"},
					{Severity: "High"},
					{Severity: "Critical"},
				},
				Vulnerabilities: []services.Vulnerability{
					{Severity: "Low"},
					{Severity: "Medium"},
					{Severity: "High"},
					{Severity: "Critical"},
				},
			},
			params: ScanGraphParams{
				severityLevel: 8,
			},
			expected: services.ScanResponse{
				Violations: []services.Violation{
					{Severity: "High"},
					{Severity: "Critical"},
				},
				Vulnerabilities: []services.Vulnerability{
					{Severity: "High"},
					{Severity: "Critical"},
				},
			},
		},
	}

	// Run test cases
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function with the input parameters
			actual := filterResultIfNeeded(&tt.scanResult, &tt.params)
			// Check that the function returned the expected result
			assert.True(t, reflect.DeepEqual(*actual, tt.expected))
		})
	}
}

func TestGetFixableComponents(t *testing.T) {
	// create test cases
	testCases := []struct {
		name        string
		components  map[string]services.Component
		expectedMap map[string]services.Component
	}{
		{
			name: "Returns an empty map when all components have no fixed versions",
			components: map[string]services.Component{
				"vuln1": {
					FixedVersions: []string{},
				},
				"vuln2": {
					FixedVersions: []string{},
				},
			},
			expectedMap: map[string]services.Component{},
		},
		{
			name: "Returns a filtered map with only components that have fixed versions",
			components: map[string]services.Component{
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
			expectedMap: map[string]services.Component{
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
