package cliutils

import (
	"flag"
	"testing"

	"github.com/jfrog/jfrog-cli-core/v2/common/format"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli"
)

func newCLIFormatContext(formatVal string) *cli.Context {
	app := cli.NewApp()
	app.Flags = []cli.Flag{cli.StringFlag{Name: "format"}}
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.String("format", "", "")
	if formatVal != "" {
		_ = fs.Set("format", formatVal)
	}
	return cli.NewContext(app, fs, nil)
}

func TestGetCLIOutputFormat_DefaultJson(t *testing.T) {
	c := newCLIFormatContext("")
	f, err := GetOutputFormat(c, format.Json)
	require.NoError(t, err)
	assert.Equal(t, format.Json, f)
}

func TestGetCLIOutputFormat_DefaultTable(t *testing.T) {
	c := newCLIFormatContext("")
	f, err := GetOutputFormat(c, format.Table)
	require.NoError(t, err)
	assert.Equal(t, format.Table, f)
}

func TestGetCLIOutputFormat_ExplicitJson(t *testing.T) {
	c := newCLIFormatContext("json")
	f, err := GetOutputFormat(c, format.Table)
	require.NoError(t, err)
	assert.Equal(t, format.Json, f)
}

func TestGetCLIOutputFormat_ExplicitTable(t *testing.T) {
	c := newCLIFormatContext("table")
	f, err := GetOutputFormat(c, format.Json)
	require.NoError(t, err)
	assert.Equal(t, format.Table, f)
}

func TestGetCLIOutputFormat_Invalid(t *testing.T) {
	c := newCLIFormatContext("xml")
	_, err := GetOutputFormat(c, format.Json)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "only the following output formats are supported")
}
