package progressbar

import (
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/utils/progressbar"
	"github.com/stretchr/testify/assert"
)

const terminalWidth = 100

func TestBuildProgressDescription(t *testing.T) {
	// Set an arbitrary terminal width
	for _, test := range getTestCases() {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.expectedDesc, buildProgressDescription(test.prefix, test.path, terminalWidth, test.extraCharsLen))
		})
	}
}

func getTestCases() []testCase {
	prefix := "  downloading"
	path := "/a/path/to/a/file"
	separator := " | "

	fullDesc := " " + prefix + separator + path + separator
	emptyPathDesc := " " + prefix + separator + "..." + separator
	shortenedDesc := " " + prefix + separator + "...ggggg/path/to/a/file" + separator

	widthMinusProgress := terminalWidth - progressbar.ProgressBarWidth*2
	return []testCase{
		{"commonUseCase", prefix, path, 17, fullDesc},
		{"zeroExtraChars", prefix, path, 0, fullDesc},
		{"minDescLength", prefix, path, widthMinusProgress - len(emptyPathDesc), emptyPathDesc},
		{"longPath", prefix, "/a/longggggggggggggggggggggg/path/to/a/file", 17, shortenedDesc},
		{"longPrefix", "longggggggggggggggggggggggggg prefix", path, 17, ""},
		{"manyExtraChars", prefix, path, widthMinusProgress - len(emptyPathDesc) + 1, ""},
	}
}

type testCase struct {
	name          string
	prefix        string
	path          string
	extraCharsLen int
	expectedDesc  string
}
