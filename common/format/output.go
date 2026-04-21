package format

import (
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

type OutputFormat string

const (
	FlagName = "format"
	// OutputFormat values
	Table      OutputFormat = "table"
	Json       OutputFormat = "json"
	SimpleJson OutputFormat = "simple-json"
	Sarif      OutputFormat = "sarif"
	CycloneDx  OutputFormat = "cyclonedx"
	None       OutputFormat = ""
)

var (
	All           = []OutputFormat{Table, Json, SimpleJson, Sarif, CycloneDx}
	OutputFormats = []string{string(Table), string(Json), string(SimpleJson), string(Sarif), string(CycloneDx)}
)

// GetFormatFlagDescription returns the help text for the --format flag.
// If supportedFormats is empty, all known formats are listed.
func GetFormatFlagDescription(supportedFormats []OutputFormat) string {
	var allFormats []string
	if len(supportedFormats) == 0 {
		allFormats = OutputFormats
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
func GetFormatFlag(supportedFormats []OutputFormat, defaultFormat OutputFormat) components.StringFlag {
	var options []components.StringFlagOption
	if defaultFormat != None {
		options = append(options, components.WithStrDefaultValue(string(defaultFormat)))
	}
	return components.NewStringFlag(
		FlagName,
		GetFormatFlagDescription(supportedFormats),
		options...,
	)
}

func GetOutputFormat(formatFlagVal string) (format OutputFormat, err error) {
	// Default print format is table.
	format = Table
	if formatFlagVal != "" {
		format, err = ParseOutputFormat(formatFlagVal, All)
	}
	return
}

func ParseOutputFormat(formatFlagVal string, supportedFormats []OutputFormat) (format OutputFormat, err error) {
	if formatFlagVal != "" {
		ff := strings.ToLower(formatFlagVal)
		for _, f := range supportedFormats {
			if ff == string(f) {
				format = f
				return
			}
		}
	}
	return None, errorutils.CheckErrorf("only the following output formats are supported: %s", Join(supportedFormats))
}

func Join(formats []OutputFormat) string {
	supportedNames := make([]string, len(formats))
	for i, f := range formats {
		supportedNames[i] = string(f)
	}
	return strings.Join(supportedNames, ", ")
}
