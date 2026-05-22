# Guidelines for Creating JFrog CLI Plugins

This comprehensive guide equips you with essential tools to embark on implementing a JFrog CLI plugin project or a related feature.

## Table of Contents
* [Implementing a JFrog CLI plugin](#implementing-a-jfrog-cli-plugin)
* [Adding a Command](#adding-a-command)
* [Utilities](#utilities)
* [Examples](#examples)

## Implementing a JFrog CLI plugin

Creating a plugin requires implementing a project that adheres to the structures outlined in the plugin [components](../plugins/components/structure.go). Below is a sample illustrating how to utilize these structures within the main package:

```go
package main

import (
	"github.com/jfrog/jfrog-cli-core/v2/plugins"
    "github.com/jfrog/jfrog-cli-core/v2/plugins/components"
)

func main() {
	// Run this plugin CLI as a stand-alone.
	plugins.PluginMain(GetApp())
}

func GetApp() components.App {
	app := components.CreateApp(
        // Plugin namespace prefix (command usage: app <cmd-name>)
		"app",
        // Plugin version vX.X.X
		"v1.0.0",
        // Plugin description for help usage
		"description",
        // Plugin commands
		GetCommands(),
	)
	return app
}

func GetCommands() []components.Command {
    return []components.Command{
		{
			Name:        "greet",
			Description: "Greet the user with log",
			Action:      GreetCmd,
		},
    }
}
```

The plugin needs to specify a list of all commands under their respective namespaces. Each command is defined using the `Command` structure.

### Nested subcommands

Some CLI paths include more than one command level after the namespace, for example `jf ai plugins publish`. Register the namespace as usual, then use `Command.Subcommands` on a parent command for intermediate groups and keep the executable work on the leaf command.

Embedded apps expose namespaces via `App.Subcommands` (`components.Namespace`). Commands listed under a namespace can define their own children through `Command.Subcommands`, which the conversion layer maps to urfave/cli subcommands.

For the `ai` → `plugins` → `publish` layout:

- `ai` is a `Namespace` on the embedded app (for example `components.Namespace{Name: "ai", Commands: aiCLI.GetAiCommands()}`).
- `plugins` is a parent `Command` with `Subcommands` only (a group, not a runnable verb).
- `publish` is the leaf `Command` with `Arguments`, `Flags`, and `Action`.

Parent group:

```go
func GetAiCommands() []components.Command {
	return []components.Command{
		{
			Name:        "plugins",
			Description: "AI agent plugin commands.",
			Subcommands: GetPluginSubCommands(),
		},
	}
}
```

Leaf subcommand:

```go
func GetPluginSubCommands() []components.Command {
	return []components.Command{
		{
			Name:        "publish",
			Description: "Publish a plugin to Artifactory.",
			Arguments: []components.Argument{
				{
					Name:        "path",
					Description: "Path to the plugin folder containing plugin.json.",
				},
			},
			Flags:  publishFlags,
			Action: publish.RunPublish,
		},
	}
}
```

Keep the following on the parent group command:

- Do not set `Action` (users must run a child verb, such as `publish`; otherwise the parent `Action` runs when no child is specified).
- Do not set `Arguments` (the leaf command owns positional args so `jf ai plugins publish --help` shows publish-specific help).
- Do not set `SkipFlagParsing` when `Subcommands` is non-empty. The converter rejects that combination because urfave/cli v1.22+ will not route to child commands when `SkipFlagParsing` is true.
- Prefer not to set `Flags` on the parent; define flags on leaf subcommands unless you intentionally need flags shared across all children of the group.

Use `SkipFlagParsing` only on leaf commands that forward raw arguments to an external tool (for example `jf mvn`), not on command groups.

## Adding a Command

To add a command you need to insert an entry to the commands list. the entry is an instance of the `Command` structure that defines an `Action` to execute when triggered, as mentioned at this example:

```go
cmd := components.Command{
	Name:        "greet",
	Aliases:	 []string{"g"},
	Description: "Greet the user with log",
	Action:      GreetCmd,
},

// Parse the command context and execute the command action.
func GreetCmd(c *components.Context) (err error) {
	log.Info("Hello World")
	return
}
```

### Define Arguments

For defining arguments within a command, follow this approach:

```go
cmd := components.Command{
	Name:        "greet",
	// The expected order of the arguments entered by the user is the same order that they are defined at this list.
	Arguments: []components.Argument{
		{
			Name: "name",
			Description: "Add the given name to the greeting log",
		},
	}
	Action:      GreetCmd,
}

// Parse the command context and execute the command action.
func GreetCmd(c *components.Context) (err error) {
	who := "Frog"
	if len(c.Arguments) == 1 {
		who = c.Arguments[0]
	} else if len(c.Arguments) > 1 {
		return pluginsCommon.WrongNumberOfArgumentsHandler(c)
	}
	log.Info(fmt.Sprintf("Hello %s", who))
	return
}
```

### Define flags

For defining flags within a command, refer to this example:

```go
cmd := components.Command{
	Name:        "greet",
	Flags:	 	 []components.Flag{
		components.NewBoolFlag(
			"pretty", 
			"Set to false to log simple greeting without decorations.", 
			components.WithBoolDefaultValue(true),
		),
	},
	Action:      GreetCmd,
}

// Parse the command context and execute the command action.
func GreetCmd(c *components.Context) (err error) {
	toLog := "Hello World"
	if c.IsFlagSet("pretty") && c.GetBoolFlagValue("pretty") {
		toLog = 🐸 + " " + toLog + "! " + 🌐
	}
	log.Info(toLog)
	return
}
```

### Define output formats

Commands that produce structured output can declare which formats they support via `SupportedFormats`. When set, a `--format` flag is automatically added to the command. The user may then pass `--format=<value>` to select an output format; the flag accepts any of the values listed in `format.OutputFormat` (`table`, `json`, `simple-json`, `sarif`, `cyclonedx`).

```go
cmd := components.Command{
	Name:        "greet",
	Description: "Greet the user",
	// Automatically adds a --format flag that accepts "table" or "json", defaulting to "table".
	// Omit DefaultFormat (or set it to format.None) to have no default.
	SupportedFormats: []format.OutputFormat{format.Table, format.Json},
	DefaultFormat:    format.Table,
	Action:      GreetCmd,
}

func GreetCmd(c *components.Context) error {
	outputFormat, err := c.GetOutputFormat()
	if err != nil {
		return err
	}
	switch outputFormat {
	case format.Json:
		// Print JSON output.
	default:
		// Print table output.
	}
	return nil
}
```

`c.GetOutputFormat()` returns `format.None` when the flag was not set by the user, and an error when the provided value is not in `SupportedFormats`. Available formats are defined in `common/format/output.go`.

## Utilities

Before implementing generic logic, ensure it hasn't been implemented yet.

* Find plugin and `Context` common utilities at [plugins/](../plugins/)

* Discover CLI common commands, utilities and global constants at [common/](../common/)

* Find general utilities at [utils](../utils/)

## Examples

Explore JFrog CLI Plugin examples in these repositories:

* [Jfrog CLI Plugins](https://github.com/jfrog/jfrog-cli-plugins)
* [JFrog Security](https://github.com/jfrog/jfrog-cli-security)