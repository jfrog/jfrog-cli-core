package components

import (
	"errors"
	"github.com/codegangsta/cli"
	"github.com/jfrog/jfrog-cli-core/docs/common"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"strings"
)

func ConvertApp(jfrogApp App) *cli.App {
	app := cli.NewApp()
	app.Name = jfrogApp.Name
	app.Usage = jfrogApp.Description
	app.Version = jfrogApp.Version
	app.Commands = convertCommands(jfrogApp)

	// Defaults:
	app.EnableBashCompletion = true
	return app
}

func convertCommands(jfrogApp App) []cli.Command {
	var converted []cli.Command
	for _, cmd := range jfrogApp.Commands {
		converted = append(converted, convertCommand(cmd, jfrogApp.Name))
	}
	return converted
}

func convertCommand(cmd Command, appName string) cli.Command {
	return cli.Command{
		Name:         cmd.Name,
		Flags:        convertFlags(cmd),
		Aliases:      cmd.Aliases,
		Usage:        cmd.Description,
		HelpName:     common.CreateUsage(appName+" "+cmd.Name, cmd.Description, createCommandUsage(cmd, appName)),
		UsageText:    createArgumentsSummary(cmd),
		ArgsUsage:    createEnvVarsSummary(cmd),
		BashComplete: common.CreateBashCompletionFunc(),
		// Passing any other interface than 'cli.ActionFunc' will fail the command.
		Action: getActionFunc(cmd),
	}
}

func createCommandUsage(cmd Command, appName string) []string {
	usage := "jfrog " + appName + " " + cmd.Name
	if len(cmd.Flags) > 0 {
		usage += " [command options]"
	}
	for _, argument := range cmd.Arguments {
		usage += " <" + argument.Name + ">"
	}
	return []string{usage}
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

func convertFlags(cmd Command) []cli.Flag {
	var convertedFlags []cli.Flag
	for _, flag := range cmd.Flags {
		converted := convertByType(flag)
		if converted != nil {
			convertedFlags = append(convertedFlags, converted)
		}
	}
	return convertedFlags
}

func convertByType(flag Flag) cli.Flag {
	if f, ok := flag.(StringFlag); ok {
		return convertStringFlag(f)
	}
	if f, ok := flag.(BoolFlag); ok {
		return convertBoolFlag(f)
	}
	log.Warn("Flag '%s' does not match any known flag type.", flag.GetName())
	return nil
}

func convertStringFlag(f StringFlag) cli.Flag {
	stringFlag := cli.StringFlag{
		Name:  f.Name,
		Usage: f.Usage + "` `",
	}
	// If default is set, add it's value and return.
	if f.DefaultValue != "" {
		stringFlag.Usage = "[Default: " + f.DefaultValue + "] " + stringFlag.Usage
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
			Usage: "[Default: true] " + f.Usage + "` `",
		}
	}
	return cli.BoolFlag{
		Name:  f.Name,
		Usage: "[Default: false] " + f.Usage + "` `",
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
			if baseContext.String(stringFlag.Name) != "" {
				c.stringFlags[stringFlag.Name] = baseContext.String(stringFlag.Name)
				continue
			}
			// Empty but has a default value defined.
			if stringFlag.DefaultValue != "" {
				c.stringFlags[stringFlag.Name] = stringFlag.DefaultValue
				continue
			}
			// Empty but mandatory.
			if stringFlag.Mandatory {
				return errors.New("Mandatory flag '" + stringFlag.Name + "' is missing")
			}
		}

		if boolFlag, ok := flag.(BoolFlag); ok {
			if boolFlag.DefaultValue {
				c.boolFlags[boolFlag.Name] = baseContext.BoolT(boolFlag.Name)
			} else {
				c.boolFlags[boolFlag.Name] = baseContext.Bool(boolFlag.Name)
			}
		}
	}
	return nil
}
