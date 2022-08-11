package ioutils

import (
	"bytes"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"io"
	"os"
	"strings"

	"github.com/chzyer/readline"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/manifoldco/promptui"
)

const (
	// Example:
	// JFrog Artifactory URL (http://localhost:8080/artifactory/)
	promptItemTemplate = " {{ .Option | cyan }}{{if .TargetValue}}({{ .TargetValue }}){{end}}"
	// Npm-remote ()
	selectableItemTemplate = " {{ .Option | cyan }}{{if .DefaultValue}} <{{ .DefaultValue }}>{{end}}"
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
func PromptStrings(items []PromptItem, label string, onSelect func(PromptItem)) error {
	items = append([]PromptItem{{Option: "Save and continue"}}, items...)
	prompt := createSelectableList(len(items), label, promptItemTemplate)
	for {
		prompt.Items = items
		i, _, err := prompt.Run()
		if err != nil {
			return errorutils.CheckError(err)
		}
		if i == 0 {
			return nil
		}
		onSelect(items[i])
	}
}

func createSelectableList(numOfItems int, label, itemTemplate string) (prompt *promptui.Select) {
	selectionIcon := "🐸"
	if !log.IsColorsSupported() {
		selectionIcon = ">"
	}
	templates := &promptui.SelectTemplates{
		Label:    "{{ . }}",
		Active:   selectionIcon + itemTemplate,
		Inactive: "  " + itemTemplate,
	}
	return &promptui.Select{
		Label:        label,
		Templates:    templates,
		Stdout:       &bellSkipper{},
		HideSelected: true,
		Size:         numOfItems,
	}
}

func SelectString(items []PromptItem, label string, needSearch bool, onSelect func(PromptItem)) error {
	selectableList := createSelectableList(len(items), label, selectableItemTemplate)
	selectableList.Items = items
	if needSearch {
		selectableList.StartInSearchMode = true
		selectableList.Searcher = func(input string, index int) bool {
			if found := strings.Index(strings.ToLower(items[index].Option), strings.ToLower(input)); found != -1 {
				return true
			}
			return false
		}
	}
	i, _, err := selectableList.Run()
	if err != nil {
		return errorutils.CheckError(err)
	}
	onSelect(items[i])
	return nil
}

// On macOS the terminal's bell is ringing when trying to select items using the up and down arrows.
//  By using bellSkipper the issue is resolved.
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
