package npm

import (
	"github.com/stretchr/testify/assert"
	"io/ioutil"
	"path/filepath"
	"testing"
)

const testdataDir = "../testdata/npm/"

func TestGetPackageFileNameFromOutput(t *testing.T) {
	tests := []struct {
		testName                string
		outputTestDataFile      string
		isJsonSupported         bool
		expectedPackageFilename string
	}{
		{"Get package filename for npm 6", "npmPackOutputV6", false, "npm-example-0.0.3.tgz"},
		{"Get package filename for npm 7", "npmPackOutputV7", false, "npm-example-ver0.0.3.tgz"},
		{"Get package filename for npm 7 with json output support", "npmPackOutputV7Json", true, "npm-example-ver0.0.3.tgz"},
	}
	for _, test := range tests {
		t.Run(test.testName, func(t *testing.T) {
			output, err := ioutil.ReadFile(filepath.Join(testdataDir, test.outputTestDataFile))
			if err != nil {
				assert.NoError(t, err)
				return
			}
			actualFilename, err := getPackageFileNameFromOutput(string(output), test.isJsonSupported)
			if err != nil {
				assert.NoError(t, err)
				return
			}
			assert.Equal(t, test.expectedPackageFilename, actualFilename)
		})
	}
}
