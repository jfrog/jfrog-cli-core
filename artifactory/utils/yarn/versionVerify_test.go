package yarn

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestIsVersionSupported(t *testing.T) {
	tests := []struct {
		versionStr string
		expectErr  bool
	}{
		{"3.9.0", false},
		{"4.0.0", true},
		{"4.1.0", true},
	}

	for _, test := range tests {
		err := IsVersionSupported(test.versionStr)
		if test.expectErr {
			assert.Error(t, err, "Expected an error for version: %s", test.versionStr)
		} else {
			assert.NoError(t, err, "Did not expect an error for version: %s", test.versionStr)
		}
	}
}
