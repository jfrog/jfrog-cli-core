package utils

import "testing"

func TestGetRegistry(t *testing.T) {
	var getRegistryTest = []struct {
		repo     string
		url      string
		expected string
	}{
		{"repo", "http://url/art", "http://url/art/api/npm/repo"},
		{"repo", "http://url/art/", "http://url/art/api/npm/repo"},
		{"repo", "", "/api/npm/repo"},
		{"", "http://url/art", "http://url/art/api/npm/"},
	}

	for _, testCase := range getRegistryTest {
		if getNpmRepositoryUrl(testCase.repo, testCase.url) != testCase.expected {
			t.Errorf("The expected output of getRegistry(\"%s\", \"%s\") is %s. But the actual result is:%s", testCase.repo, testCase.url, testCase.expected, getNpmRepositoryUrl(testCase.repo, testCase.url))
		}
	}
}
