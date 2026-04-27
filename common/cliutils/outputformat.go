package cliutils

import (
	"github.com/jfrog/jfrog-cli-core/v2/common/format"
	"github.com/urfave/cli"
)

// GetOutputFormat returns the output format from the --format flag on a urfave/cli
// context, falling back to defaultFormat when the flag is not set.
// Accepted values are json and table.
func GetOutputFormat(c *cli.Context, defaultFormat format.OutputFormat) (format.OutputFormat, error) {
	if !c.IsSet("format") {
		return defaultFormat, nil
	}
	return format.ParseOutputFormat(c.String("format"), []format.OutputFormat{format.Json, format.Table})
}
