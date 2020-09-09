package plugins

import (
	"github.com/codegangsta/cli"
	"github.com/jfrog/jfrog-cli-core/docs/common"
)

func (jfrogApp *App) convert() *cli.App {
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
		ArgsUsage:    common.CreateEnvVars(cmd.createEnvVarsSummary()...),
		BashComplete: common.CreateBashCompletionFunc(),
		// Passing any other interface than 'cli.ActionFunc' will fail the command.
		Action: actionFuncWrapper(cmd.Action),
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

func (cmd *Command) createEnvVarsSummary() []string {
	var envVarsSummary []string
	for i, env := range cmd.EnvArgs {
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
	return envVarsSummary
}

func (cmd *Command) convertFlags() []cli.Flag {
	var converted []cli.Flag
	for _, flag := range cmd.Flags {
		converted = append(converted,
			cli.StringFlag{
				Name:  flag.Name,
				Usage: flag.Usage + "` `",
			})
	}
	return converted
}

// Wrap the base's ActionFunc with our own, while retrieving needed information from the Context.
func actionFuncWrapper(action ActionFunc) cli.ActionFunc {
	return func(c *cli.Context) error {
		context := &Context{
			Arguments: c.Args(),
			Flags:     createFlagsMap(c),
		}
		return action(context)
	}
}

func createFlagsMap(c *cli.Context) map[string]string {
	flagsMap := map[string]string{}
	// Loop over all known flags.
	for _, flag := range c.Command.Flags {
		if stringFlag, ok := flag.(cli.StringFlag); ok {
			if c.String(stringFlag.Name) != "" {
				flagsMap[stringFlag.Name] = c.String(stringFlag.Name)
			}
		}
	}
	return flagsMap
}
