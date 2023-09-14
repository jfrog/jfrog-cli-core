package utils

import (
	"errors"
	"fmt"
	"testing"

	"github.com/owenrumney/go-sarif/v2/sarif"
	"github.com/stretchr/testify/assert"
)

func TestGetResultFileName(t *testing.T) {
	fileNameValue := "fileNameValue"
	tests := []struct {
		result         *sarif.Result
		expectedOutput string
	}{
		{result: &sarif.Result{
			Locations: []*sarif.Location{
				{PhysicalLocation: &sarif.PhysicalLocation{ArtifactLocation: &sarif.ArtifactLocation{URI: nil}}},
			}},
			expectedOutput: ""},
		{result: &sarif.Result{
			Locations: []*sarif.Location{
				{PhysicalLocation: &sarif.PhysicalLocation{ArtifactLocation: &sarif.ArtifactLocation{URI: &fileNameValue}}},
			}},
			expectedOutput: fileNameValue},
	}

	for _, test := range tests {
		assert.Equal(t, test.expectedOutput, GetLocationFileName(test.result.Locations[0]))
	}

}

func TestScanTypeErrorMsg(t *testing.T) {
	tests := []struct {
		scanner JasScanType
		err     error
		wantMsg string
	}{
		{
			scanner: Applicability,
			err:     errors.New("an error occurred"),
			wantMsg: fmt.Sprintf(ErrFailedScannerRun, Applicability, "an error occurred"),
		},
		{
			scanner: Applicability,
			err:     nil,
			wantMsg: "",
		},
		{
			scanner: Secrets,
			err:     nil,
			wantMsg: "",
		},
		{
			scanner: Secrets,
			err:     errors.New("an error occurred"),
			wantMsg: fmt.Sprintf(ErrFailedScannerRun, Secrets, "an error occurred"),
		},
		{
			scanner: IaC,
			err:     nil,
			wantMsg: "",
		},
		{
			scanner: IaC,
			err:     errors.New("an error occurred"),
			wantMsg: fmt.Sprintf(ErrFailedScannerRun, IaC, "an error occurred"),
		},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("Scanner: %s", test.scanner), func(t *testing.T) {
			gotMsg := test.scanner.FormattedError(test.err)
			if gotMsg == nil {
				assert.Nil(t, test.err)
				return
			}
			assert.Equal(t, test.wantMsg, gotMsg.Error())
		})
	}
}
