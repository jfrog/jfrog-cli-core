package components

import (
	"errors"
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/jfrog/jfrog-cli-core/v2/docs/common"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"strings"
)

func ConvertApp(jfrogApp App) (*cli.App, error) {
	var err error
	app := cli.NewApp()
	app.Name = jfrogApp.Name
	app.Description = jfrogApp.Description
	app.Version = jfrogApp.Version
	app.Commands, err = convertCommands(jfrogApp)
	if err != nil {
		return nil, err
	}
	// Defaults:
	app.EnableBashCompletion = true
	return app, nil
}

func convertCommands(jfrogApp App) ([]cli.Command, error) {
	var converted []cli.Command
	for _, cmd := range jfrogApp.Commands {
		cur, err := convertCommand(cmd, jfrogApp.Name)
		if err != nil {
			return converted, err
		}
		converted = append(converted, cur)
	}
	return converted, nil
}

func convertCommand(cmd Command, appName string) (cli.Command, error) {
	convertedFlags, err := convertFlags(cmd)
	if err != nil {
		return cli.Command{}, err
	}
	return cli.Command{
		Name:            cmd.Name,
		Flags:           convertedFlags,
		Aliases:         cmd.Aliases,
		Description:     cmd.Description,
		HelpName:        common.CreateUsage(appName+" "+cmd.Name, cmd.Description, []string{createCommandUsage(cmd, appName)}),
		UsageText:       createArgumentsSummary(cmd),
		ArgsUsage:       createEnvVarsSummary(cmd),
		BashComplete:    common.CreateBashCompletionFunc(),
		SkipFlagParsing: cmd.SkipFlagParsing,
		// Passing any other interface than 'cli.ActionFunc' will fail the command.
		Action: getActionFunc(cmd),
	}, nil
}

func createCommandUsage(cmd Command, appName string) string {
	usage := fmt.Sprintf(coreutils.GetCliExecutableName()+" %s %s", appName, cmd.Name)
	if len(cmd.Flags) > 0 {
		usage += " [command options]"
	}
	for _, argument := range cmd.Arguments {
		usage += fmt.Sprintf(" <%s>", argument.Name)
	}
	return usage
}

func createArgumentsSummary(cmd Command) string {
	summary := ""
	for i, argument := range cmd.Arguments {
		if i > 0 {
			summary += "\n"
		}
		summary += "\t" + argument.Name + "\n\t\t" + argument.Description + "\n"
	}
	return summary
}

func createEnvVarsSummary(cmd Command) string {
	var envVarsSummary []string
	for i, env := range cmd.EnvVars {
		summary := ""
		if i > 0 {
			summary += "\n"
		}
		summary = "\t" + env.Name + "\n"
		if env.Default != "" {
			summary += "\t\t[Default: " + env.Default + "]\n"
		}
		summary += "\t\t" + env.Description
		envVarsSummary = append(envVarsSummary, summary)
	}
	return strings.Join(envVarsSummary[:], "\n\n")
}

func convertFlags(cmd Command) ([]cli.Flag, error) {
	var convertedFlags []cli.Flag
	for _, flag := range cmd.Flags {
		converted, err := convertByType(flag)
		if err != nil {
			return convertedFlags, err
		}
		if converted != nil {
			convertedFlags = append(convertedFlags, converted)
		}
	}
	return convertedFlags, nil
}

func convertByType(flag Flag) (cli.Flag, error) {
	if f, ok := flag.(StringFlag); ok {
		return convertStringFlag(f), nil
	}
	if f, ok := flag.(BoolFlag); ok {
		return convertBoolFlag(f), nil
	}
	return nil, errors.New(fmt.Sprintf("Flag '%s' does not match any known flag type.", flag.GetName()))
}

func convertStringFlag(f StringFlag) cli.Flag {
	stringFlag := cli.StringFlag{
		Name:  f.Name,
		Usage: f.Description + "` `",
	}
	// If default is set, add its value and return.
	if f.DefaultValue != "" {
		stringFlag.Usage = fmt.Sprintf("[Default: %s] %s", f.DefaultValue, stringFlag.Usage)
		return stringFlag
	}
	// Otherwise, mark as mandatory/optional accordingly.
	if f.Mandatory {
		stringFlag.Usage = "[Mandatory] " + stringFlag.Usage
	} else {
		stringFlag.Usage = "[Optional] " + stringFlag.Usage
	}
	return stringFlag
}

func convertBoolFlag(f BoolFlag) cli.Flag {
	if f.DefaultValue {
		return cli.BoolTFlag{
			Name:  f.Name,
			Usage: "[Default: true] " + f.Description + "` `",
		}
	}
	return cli.BoolFlag{
		Name:  f.Name,
		Usage: "[Default: false] " + f.Description + "` `",
	}
}

// Wrap the base's ActionFunc with our own, while retrieving needed information from the Context.
func getActionFunc(cmd Command) cli.ActionFunc {
	return func(baseContext *cli.Context) error {
		pluginContext := &Context{}
		pluginContext.Arguments = baseContext.Args()
		err := fillFlagMaps(pluginContext, baseContext, cmd.Flags)
		if err != nil {
			return err
		}
		return cmd.Action(pluginContext)
	}
}

func fillFlagMaps(c *Context, baseContext *cli.Context, originalFlags []Flag) error {
	c.stringFlags = make(map[string]string)
	c.boolFlags = make(map[string]bool)

	// Loop over all plugin's known flags.
	for _, flag := range originalFlags {
		if stringFlag, ok := flag.(StringFlag); ok {
			finalValue, err := getValueForStringFlag(stringFlag, baseContext.String(stringFlag.Name))
			if err != nil {
				return err
			}
			c.stringFlags[stringFlag.Name] = finalValue
			continue
		}

		if boolFlag, ok := flag.(BoolFlag); ok {
			c.boolFlags[boolFlag.Name] = getValueForBoolFlag(boolFlag, baseContext)
		}
	}
	return nil
}

func getValueForStringFlag(f StringFlag, receivedValue string) (finalValue string, err error) {
	if receivedValue != "" {
		return receivedValue, nil
	}
	// Empty but has a default value defined.
	if f.DefaultValue != "" {
		return f.DefaultValue, nil
	}
	// Empty but mandatory.
	if f.Mandatory {
		return "", errors.New("Mandatory flag '" + f.Name + "' is missing")
	}
	return "", nil
}

func getValueForBoolFlag(f BoolFlag, baseContext *cli.Context) bool {
	if f.DefaultValue {
		return baseContext.BoolT(f.Name)
	}
	return baseContext.Bool(f.Name)
}
