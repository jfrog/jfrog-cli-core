package format

import (
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

type OutputFormat string

const (
	// OutputFormat values
	Table      OutputFormat = "table"
	Json       OutputFormat = "json"
	SimpleJson OutputFormat = "simple-json"
	Sarif      OutputFormat = "sarif"
	CycloneDx  OutputFormat = "cyclonedx"
)

var OutputFormats = []string{string(Table), string(Json), string(SimpleJson), string(Sarif), string(CycloneDx)}

func GetOutputFormat(formatFlagVal string) (format OutputFormat, err error) {
	// Default print format is table.
	format = Table
	if formatFlagVal != "" {
		switch strings.ToLower(formatFlagVal) {
		case string(Table):
			format = Table
		case string(Json):
			format = Json
		case string(SimpleJson):
			format = SimpleJson
		case string(Sarif):
			format = Sarif
		case string(CycloneDx):
			format = CycloneDx
		default:
			err = errorutils.CheckErrorf("only the following output formats are supported: %s", coreutils.ListToText(OutputFormats))
		}
	}
	return
}
