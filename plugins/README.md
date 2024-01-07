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

## Utilities

Before implementing generic logic, ensure it hasn't been implemented yet.

* Find plugin and `Context` common utilities at [plugins/](../plugins/)

* Discover CLI common commands, utilities and global constants at [common/](../common/)

* Find general utilities at [utils](../utils/)

## Examples

Explore JFrog CLI Plugin examples in these repositories:

* [Jfrog CLI Plugins](https://github.com/jfrog/jfrog-cli-plugins)
* [JFrog Security](https://github.com/jfrog/jfrog-cli-security)