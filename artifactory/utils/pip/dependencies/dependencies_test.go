package dependencies

import (
	"strings"
	"testing"
)

func TestCreateAqlQueries(t *testing.T) {

	fileToPackageMap := map[string]string{"file1.tgz": "file1", "file2.whl": "file2", "file3.tar.gz": "file3", "file4.egg": "file4"}
	expectedQueryParts := []string{
		"{\"$and\":[{\"path\":{\"$match\":\"*\"},\"name\":{\"$match\":\"file1.tgz\"}}]}",
		"{\"$and\":[{\"path\":{\"$match\":\"*\"},\"name\":{\"$match\":\"file2.whl\"}}]}",
		"{\"$and\":[{\"path\":{\"$match\":\"*\"},\"name\":{\"$match\":\"file3.tar.gz\"}}]}",
		"{\"$and\":[{\"path\":{\"$match\":\"*\"},\"name\":{\"$match\":\"file4.egg\"}}]}"}

	tests := []struct {
		name                string
		fileToPackage       map[string]string
		bulkSize            int
		expectedResultsSize int
	}{
		{"test1", fileToPackageMap, 2, 2},
		{"test2", fileToPackageMap, 1, 4},
		{"test3", fileToPackageMap, 4, 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			results := createAqlQueries(test.fileToPackage, "repository", test.bulkSize)
			// Validate results size.
			if len(results) != test.expectedResultsSize {
				t.Errorf("Expected result of: %d queries, got: %d.", test.expectedResultsSize, len(results))
			}
			// Validate results content.
			for _, expectedQueryPart := range expectedQueryParts {
				foundQueryPart := false
				for _, resultQuery := range results {
					if strings.Contains(resultQuery, expectedQueryPart) {
						foundQueryPart = true
						break
					}
				}
				if !foundQueryPart {
					t.Errorf("The query part: '%s' is expected in the results: %v.", expectedQueryPart, results)
				}
			}
		})
	}
}
