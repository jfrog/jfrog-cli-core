package utils

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/ioutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

const pathErrorSuffixMsg = " please enter a path, in which the new template file will be created"

type TemplateUserCommand interface {
	// Returns the file path.
	TemplatePath() string
	// Returns vars to replace in the template content.
	Vars() string
}

func ConvertTemplateToMap(templateUserCommand TemplateUserCommand) (map[string]interface{}, error) {
	// Read the template file
	content, err := fileutils.ReadFile(templateUserCommand.TemplatePath())
	if errorutils.CheckError(err) != nil {
		return nil, err
	}
	// Replace vars string-by-string if needed
	if len(templateUserCommand.Vars()) > 0 {
		templateVars := coreutils.SpecVarsStringToMap(templateUserCommand.Vars())
		content = coreutils.ReplaceVars(content, templateVars)
	}
	// Unmarshal template to a map
	var configMap map[string]interface{}
	err = json.Unmarshal(content, &configMap)
	return configMap, errorutils.CheckError(err)
}

func ConvertTemplateToMaps(templateUserCommand TemplateUserCommand) (interface{}, error) {
	content, err := fileutils.ReadFile(templateUserCommand.TemplatePath())
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	// Replace vars string-by-string if needed
	if len(templateUserCommand.Vars()) > 0 {
		templateVars := coreutils.SpecVarsStringToMap(templateUserCommand.Vars())
		content = coreutils.ReplaceVars(content, templateVars)
	}

	var multiRepoCreateEntities []map[string]interface{}
	err = json.Unmarshal(content, &multiRepoCreateEntities)
	if err == nil {
		return multiRepoCreateEntities, nil
	}

	if _, ok := err.(*json.SyntaxError); ok {
		return nil, errorutils.CheckError(err)
	}

	var repoCreateEntity map[string]interface{}
	err = json.Unmarshal(content, &repoCreateEntity)
	if err == nil {
		return repoCreateEntity, nil
	}

	return nil, errorutils.CheckError(err)
}

func ValidateMapEntry(key string, value interface{}, writersMap map[string]ioutils.AnswerWriter) error {
	if _, ok := writersMap[key]; !ok {
		return errorutils.CheckErrorf("template syntax error: unknown key: \"" + key + "\".")
	}
	if _, ok := value.(string); !ok {
		return errorutils.CheckErrorf("template syntax error: the value for the  key: \"" + key + "\" is not a string type.")
	}
	return nil
}

func ValidateTemplatePath(templatePath string) error {
	exists, err := fileutils.IsDirExists(templatePath, false)
	if err != nil {
		return errorutils.CheckError(err)
	}
	if exists || strings.HasSuffix(templatePath, string(os.PathSeparator)) {
		return errorutils.CheckErrorf("path cannot be a directory," + pathErrorSuffixMsg)
	}
	exists, err = fileutils.IsFileExists(templatePath, false)
	if err != nil {
		return errorutils.CheckError(err)
	}
	if exists {
		return errorutils.CheckErrorf("file already exists," + pathErrorSuffixMsg)
	}
	return nil
}
