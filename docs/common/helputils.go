package common

import (
	"fmt"
	"github.com/codegangsta/cli"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
)

func CreateUsage(command string, name string, commands []string) string {
	var usage string
	for _, cmd := range commands {
		usage += coreutils.GetCliExecutableName() + " " + cmd + "\n\t"
	}
	return "\nName:\n\t" + coreutils.GetCliExecutableName() + " " + command + " - " + name + "\n\nUsage:\n\t" + usage
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
