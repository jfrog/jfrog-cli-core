package components

import (
	"fmt"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestCreateCommandUsage(t *testing.T) {
	cmd := Command{
		Name: "test-command",
		Flags: []Flag{
			StringFlag{
				Name: "dummyFlag",
			},
		},
		Arguments: []Argument{
			{
				Name:        "first argument",
				Description: "this is the first argument.",
			},
			{
				Name:        "second",
				Description: "this is the second.",
			},
		},
	}
	appName := "test-app"
	expected := fmt.Sprintf("%s %s %s [command options] <%s> <%s>", coreutils.GetCliExecutableName(), appName, cmd.Name, cmd.Arguments[0].Name, cmd.Arguments[1].Name)
	assert.Equal(t, createCommandUsage(cmd, appName), expected)
}

func TestCreateArgumentsSummary(t *testing.T) {
	cmd := Command{
		Arguments: []Argument{
			{
				Name:        "first argument",
				Description: "this is the first argument.",
			},
			{
				Name:        "second",
				Description: "this is the second.",
			},
		},
	}
	expected :=
		`	first argument
		this is the first argument.

	second
		this is the second.
`
	assert.Equal(t, createArgumentsSummary(cmd), expected)
}

func TestCreateEnvVarsSummary(t *testing.T) {
	cmd := Command{
		EnvVars: []EnvVar{
			{
				Name:        "FIRST_ENV",
				Default:     "15",
				Description: "This is the first env.",
			},
			{
				Name:        "NO_DEFAULT",
				Description: "This flag has no default.",
			},
			{
				Name:        "THIRD_ENV",
				Default:     "true",
				Description: "This is the third env.",
			},
		},
	}
	expected :=
		`	FIRST_ENV
		[Default: 15]
		This is the first env.

	NO_DEFAULT
		This flag has no default.

	THIRD_ENV
		[Default: true]
		This is the third env.`
	assert.Equal(t, createEnvVarsSummary(cmd), expected)
}

type invalidFlag struct {
	Name  string
	Usage string
}

func (f invalidFlag) GetName() string {
	return f.Name
}

func (f invalidFlag) GetDescription() string {
	return f.Usage
}

func TestConvertByTypeFailWithInvalidFlag(t *testing.T) {
	invalid := invalidFlag{
		Name:  "invalid",
		Usage: "",
	}
	_, err := convertByType(invalid)
	assert.Error(t, err)
}

func TestConvertStringFlagDefault(t *testing.T) {
	f := StringFlag{
		Name:         "string-flag",
		Description:  "This is how you use it.",
		DefaultValue: "def",
	}
	converted, err := convertByType(f)
	assert.NoError(t, err)
	if err != nil {
		return
	}

	expected := "--string-flag  \t[Default: def] This is how you use it."
	assert.Equal(t, converted.String(), expected)

	// Verify that when both Default and Mandatory are passed, only Default is shown.
	f.Mandatory = true
	converted, err = convertByType(f)
	assert.NoError(t, err)
	if err != nil {
		return
	}

	assert.Equal(t, converted.String(), expected)
}

func TestConvertStringFlagMandatory(t *testing.T) {
	f := StringFlag{
		Name:        "string-flag",
		Description: "This is how you use it.",
		Mandatory:   true,
	}
	converted, err := convertByType(f)
	assert.NoError(t, err)
	if err != nil {
		return
	}

	assert.Equal(t, converted.String(), "--string-flag  \t[Mandatory] This is how you use it.")

	// Test optional.
	f.Mandatory = false
	converted, err = convertByType(f)
	assert.NoError(t, err)
	if err != nil {
		return
	}

	assert.Equal(t, converted.String(), "--string-flag  \t[Optional] This is how you use it.")
}

func TestConvertBoolFlag(t *testing.T) {
	f := BoolFlag{
		Name:         "bool-flag",
		Description:  "This is how you use it.",
		DefaultValue: true,
	}
	converted, err := convertByType(f)
	assert.NoError(t, err)
	if err != nil {
		return
	}

	assert.Equal(t, converted.String(), "--bool-flag  \t[Default: true] This is how you use it.")

	// Test optional.
	f.DefaultValue = false
	converted, err = convertByType(f)
	assert.NoError(t, err)
	if err != nil {
		return
	}

	assert.Equal(t, converted.String(), "--bool-flag  \t[Default: false] This is how you use it.")
}

func TestGetValueForStringFlag(t *testing.T) {
	f := StringFlag{
		Name:        "string-flag",
		Description: "This is how you use it.",
		Mandatory:   false,
	}

	// Not received, no default or mandatory.
	finalValue, err := getValueForStringFlag(f, "")
	assert.NoError(t, err)
	assert.Empty(t, finalValue)

	// Not received, no default but mandatory.
	f.Mandatory = true
	finalValue, err = getValueForStringFlag(f, "")
	assert.Error(t, err)

	// Not received, verify default is taken.
	f.DefaultValue = "default"
	finalValue, err = getValueForStringFlag(f, "")
	assert.NoError(t, err)
	assert.Equal(t, finalValue, f.DefaultValue)

	// Received, verify default is ignored.
	expected := "value"
	finalValue, err = getValueForStringFlag(f, expected)
	assert.NoError(t, err)
	assert.Equal(t, finalValue, expected)
}
