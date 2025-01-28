package utils

import (
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
	"github.com/stretchr/testify/assert"
	"path/filepath"
	"testing"
)

// TODO complete
func TestGetPlainGitLogFromLastVcsRevision(t *testing.T) {
	// Create git folder with files
	originalFolder := "git_issues_.git_suffix"
	baseDir, dotGitPath := tests.PrepareDotGitDir(t, originalFolder, filepath.Join("..", "..", "commands", "testdata"))
	gitDetails := GitParsingDetails{DotGitPath: dotGitPath, LogLimit: 3, PrettyFormat: "fuller"}

	// Collect issues
	gitLog, err := getPlainGitLogFromLastVcsRevision(gitDetails, "")
	assert.NoError(t, err)
	assert.NotEmpty(t, gitLog)

	// Clean git path
	tests.RenamePath(dotGitPath, filepath.Join(baseDir, originalFolder), t)
}
