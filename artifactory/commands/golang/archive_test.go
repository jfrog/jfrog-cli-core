package golang

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/jfrog/build-info-go/utils"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/log"
	"github.com/jfrog/jfrog-cli-core/v2/utils/tests"
)

func TestArchiveProject(t *testing.T) {
	log.SetDefaultLogger()
	if coreutils.IsWindows() {
		t.Skip("Skipping archive test...")
	}
	pwd, err := os.Getwd()
	if err != nil {
		t.Error(err)
	}

	buff := &bytes.Buffer{}
	if err != nil {
		t.Error(err)
	}
	originalFolder := "test_.git_suffix"
	baseDir, dotGitPath := tests.PrepareDotGitDir(t, originalFolder, "testdata")
	err = archiveProject(buff, filepath.Join(pwd, "testdata"), "myproject.com/module/name", "v1.0.0", nil)
	if err != nil {
		t.Error(err)
	}
	expected := map[utils.Algorithm]string{utils.MD5: "28617d6e74fce3dd2bab21b1bd65009b", utils.SHA1: "410814fbf21afdfb9c5b550151a51c2e986447fa", utils.SHA256: "e877c07315d6d3ad69139035defc08c04b400b36cd069b35ea3c2960424f2dc6"}
	actual, err := utils.CalcChecksums(buff)
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("Expecting: %v, Got: %v", expected, actual)
	}
	tests.RenamePath(dotGitPath, filepath.Join(baseDir, originalFolder), t)
}
