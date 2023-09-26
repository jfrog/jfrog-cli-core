package utils

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
