package format

import (
	"strings"

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

// Deprecated: Use ParseOutputFormat instead.
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
