package ioutils

import (
	"bytes"
	"io"
	"os"

	"github.com/chzyer/readline"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/manifoldco/promptui"
)

const (
	// Example:
	// JFrog Artifactory URL (http://localhost:8080/artifactory/)
	selectableItemTemplate = " {{ .Option | cyan }}{{if .TargetValue}}({{ .TargetValue }}){{end}}"
)

type PromptItem struct {
	// The option string to show, i.e - JFrog Artifactory URL.
	Option string
	// The variable to set.
	TargetValue *string
	// Default value to show. If empty string is entered, use the default value.
	DefaultValue string
}

// Prompt strings by selecting from list until "Save and continue" is selected.
// Usage example:
// 🐸 Save and continue
// JFrog Artifactory URL (http://localhost:8080/artifactory/)
// JFrog Distribution URL ()
// JFrog Xray URL ()
// JFrog Mission Control URL ()
// JFrog Pipelines URL ()
func PromptStrings(items []PromptItem, label string, onSelect func()) error {
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}?",
		Active:   "🐸" + selectableItemTemplate,
		Inactive: "  " + selectableItemTemplate,
	}
	prompt := promptui.Select{
		Label:        label,
		Templates:    templates,
		Stdout:       &bellSkipper{},
		HideSelected: true,
		Size:         len(items) + 1,
	}
	items = append([]PromptItem{{Option: "Save and continue"}}, items...)
	for {
		prompt.Items = items
		i, _, err := prompt.Run()
		if err != nil {
			return errorutils.CheckError(err)
		}
		if i == 0 {
			return nil
		}
		ScanFromConsole(items[i].Option, items[i].TargetValue, items[i].DefaultValue)
		onSelect()
	}
}

// In MacOS, Terminal bell is ringing when trying to select items using up and down arrows.
// Using bellSkipper as Stdout is a workaround for this issue.
type bellSkipper struct{ io.WriteCloser }

var charBell = []byte{readline.CharBell}

func (bs *bellSkipper) Write(b []byte) (int, error) {
	if bytes.Equal(b, charBell) {
		return 0, nil
	}
	return os.Stderr.Write(b)
}

func (bs *bellSkipper) Close() error {
	return os.Stderr.Close()
}
