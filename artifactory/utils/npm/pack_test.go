package npm

import (
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"testing"
)

const testdataDir = "../testdata/npm/"

func TestGetPackageFileNameFromOutput(t *testing.T) {
	tests := []struct {
		testName                string
		outputTestDataFile      string
		expectedPackageFilename string
	}{
		{"Get package filename for npm 6", "npmPackOutputV6", "npm-example-0.0.3.tgz"},
		{"Get package filename for npm 7", "npmPackOutputV7", "npm-example-ver0.0.3.tgz"},
	}
	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			output, err := os.ReadFile(filepath.Join(testdataDir, test.outputTestDataFile))
			if err != nil {
				assert.NoError(t, err)
				return
			}
			actualFilename := getPackageFileNameFromOutput(string(output))
			assert.Equal(t, test.expectedPackageFilename, actualFilename)
		})
	}
}
