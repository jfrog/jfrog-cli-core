package common

import (
	"fmt"
	"github.com/codegangsta/cli"
	"strings"
)

func CreateUsage(command string, name string, commands []string) string {
	return "\nName:\n\t" + "jfrog " + command + " - " + name + "\n\nUsage:\n\t" + strings.Join(commands[:], "\n\t") + "\n"
}

func CreateBashCompletionFunc(extraCommands ...string) cli.BashCompleteFunc {
	return func(ctx *cli.Context) {
		for _, command := range extraCommands {
			fmt.Println(command)
		}
		flagNames := append(ctx.FlagNames(), "help")
		for _, flagName := range flagNames {
			fmt.Println("--" + flagName)
		}
	}
}
