package components

import (
	"errors"
	"github.com/codegangsta/cli"
	"github.com/jfrog/jfrog-cli-core/docs/common"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"strings"
)

func (jfrogApp *App) Convert() *cli.App {
	app := cli.NewApp()
	app.Name = jfrogApp.Name
	app.Usage = jfrogApp.Description
	app.Version = jfrogApp.Version
	app.Commands = jfrogApp.convertCommands()

	// Defaults:
	app.EnableBashCompletion = true
	return app
}

func (jfrogApp *App) convertCommands() []cli.Command {
	var converted []cli.Command
	for _, cmd := range jfrogApp.Commands {
		converted = append(converted, cmd.convert(jfrogApp.Name))
	}
	return converted
}

func (cmd *Command) convert(appName string) cli.Command {
	return cli.Command{
		Name:         cmd.Name,
		Flags:        cmd.convertFlags(),
		Aliases:      cmd.Aliases,
		Usage:        cmd.Description,
		HelpName:     common.CreateUsage(appName+" "+cmd.Name, cmd.Description, cmd.createCommandUsage(appName)),
		UsageText:    cmd.createArgumentsSummary(),
		ArgsUsage:    cmd.createEnvVarsSummary(),
		BashComplete: common.CreateBashCompletionFunc(),
		// Passing any other interface than 'cli.ActionFunc' will fail the command.
		Action: cmd.getActionFunc(),
	}
}

func (cmd *Command) createCommandUsage(appName string) []string {
	usage := "jfrog " + appName + " " + cmd.Name
	if len(cmd.Flags) > 0 {
		usage += " [command options]"
	}
	for _, argument := range cmd.Arguments {
		usage += " <" + argument.Name + ">"
	}
	return []string{usage}
}

func (cmd *Command) createArgumentsSummary() string {
	summary := ""
	for i, argument := range cmd.Arguments {
		if i > 0 {
			summary += "\n"
		}
		summary += "\t" + argument.Name + "\n\t\t" + argument.Description + "\n"
	}
	return summary
}

func (cmd *Command) createEnvVarsSummary() string {
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

func (cmd *Command) convertFlags() []cli.Flag {
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
		return f.convert()
	}
	if f, ok := flag.(BoolFlag); ok {
		return f.convert()
	}
	log.Warn("Flag '%s' does not match any known flag type.", flag.GetName())
	return nil
}

func (f StringFlag) convert() cli.Flag {
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

func (f BoolFlag) convert() cli.Flag {
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
func (cmd *Command) getActionFunc() cli.ActionFunc {
	return func(c *cli.Context) error {
		context := &Context{}
		context.Arguments = c.Args()
		err := context.fillFlagMaps(c, cmd.Flags)
		if err != nil {
			return err
		}
		return cmd.Action(context)
	}
}

func (c *Context) fillFlagMaps(baseContext *cli.Context, originalFlags []Flag) error {
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
