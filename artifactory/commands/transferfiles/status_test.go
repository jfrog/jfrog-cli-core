package transferfiles

import (
	"testing"

	"github.com/magiconair/properties/assert"
)

func TestSizeToString(t *testing.T) {
	testCases := []struct {
		sizeInBytes int64
		expected    string
	}{
		{0, "0.0 KiB"},
		{10, "0.0 KiB"},
		{100, "0.1 KiB"},
		{1000, "1.0 KiB"},
		{1024, "1.0 KiB"},
		{1025, "1.0 KiB"},
		{4000, "3.9 KiB"},
		{4096, "4.0 KiB"},
		{1000000, "976.6 KiB"},
		{1048576, "1.0 MiB"},
		{1073741824, "1.0 GiB"},
		{1073741824, "1.0 GiB"},
		{1099511627776, "1.0 TiB"},
		{1125899906842624, "1.0 PiB"},
		{1125899906842624, "1.0 PiB"},
		{1.152921504606847e18, "1.0 EiB"},
	}
	for _, testCase := range testCases {
		assert.Equal(t, sizeToString(testCase.sizeInBytes), testCase.expected)
	}
}
