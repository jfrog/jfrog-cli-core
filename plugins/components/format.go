package components

import (
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/common/format"
)

// GetFormatFlagDescription returns the help text for the --format flag.
// If supportedFormats is empty, all known formats are listed.
func GetFormatFlagDescription(supportedFormats []format.OutputFormat) string {
	var allFormats []string
	if len(supportedFormats) == 0 {
		allFormats = format.OutputFormats
	} else {
		allFormats = make([]string, len(supportedFormats))
		for i, f := range supportedFormats {
			allFormats[i] = string(f)
		}
	}
	return "Defines the output format of the command. Acceptable values are: " + strings.Join(allFormats, ", ")
}

// GetFormatFlag returns the standard --format StringFlag for use in any command's Flags list.
// If supportedFormats is empty, all known formats are listed in the description.
func GetFormatFlag(supportedFormats []format.OutputFormat, defaultFormat format.OutputFormat) StringFlag {
	var options []StringFlagOption
	if defaultFormat == format.None {
		options = append(options, SetMandatoryFalse())
	} else {
		options = append(options, WithStrDefaultValue(string(defaultFormat)))
	}
	return NewStringFlag(
		format.FlagName,
		GetFormatFlagDescription(supportedFormats),
		options...,
	)
}
