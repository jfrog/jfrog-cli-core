package golang

import (
	"bytes"
	"github.com/stretchr/testify/assert"
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
	originalFolder := "test_.git_suffix"
	baseDir, dotGitPath := tests.PrepareDotGitDir(t, originalFolder, "testdata")
	var archiveWithExclusion = []struct {
		buff             *bytes.Buffer
		filePath         string
		mod              string
		version          string
		excludedPatterns []string
		expected         map[utils.Algorithm]string
	}{
		{buff, filepath.Join(pwd, "testdata"), "myproject.com/module/name", "v1.0.0", nil, map[utils.Algorithm]string{utils.MD5: "5b3603a7bf637622516673b845249205", utils.SHA1: "7386685c432c39428c9cb8584a2b970139c5e626", utils.SHA256: "eefd8aa3f9ac89876c8442d5feebbc837666bf40114d201219e3e6d51c208949"}},
		{buff, filepath.Join(pwd, "testdata"), "myproject.com/module/name", "v1.0.0", []string{"testdata/dir1/*"}, map[utils.Algorithm]string{utils.MD5: "c2eeb4ef958edee91570690bf4111fc7", utils.SHA1: "d77e10eaa9bd863a9ff3775d3e452041e6f5aa40", utils.SHA256: "ecf66c1256263b2b4386efc299fa0c389263608efda9d1d91af8a746e6c5709a"}},
		{buff, filepath.Join(pwd, "testdata"), "myproject.com/module/name", "v1.0.0", []string{"testdata/dir2/*"}, map[utils.Algorithm]string{utils.MD5: "bbe78a98ba10c1428f3a364570015e11", utils.SHA1: "99fd22ea2fe9c2c48124e741881fc3a555458a7e", utils.SHA256: "e2299f3c4e1f22d36befba191a347783dc2047e8e38cf6b9b96c273090f6e25b"}},
		{buff, filepath.Join(pwd, "testdata"), "myproject.com/module/name", "v1.0.0", []string{"testdata/dir2/*", "testdata/dir3/*"}, map[utils.Algorithm]string{utils.MD5: "28617d6e74fce3dd2bab21b1bd65009b", utils.SHA1: "410814fbf21afdfb9c5b550151a51c2e986447fa", utils.SHA256: "e877c07315d6d3ad69139035defc08c04b400b36cd069b35ea3c2960424f2dc6"}},
		{buff, filepath.Join(pwd, "testdata"), "myproject.com/module/name", "v1.0.0", []string{"testdata/dir2/*", "testdata/dir4/*"}, map[utils.Algorithm]string{utils.MD5: "bbe78a98ba10c1428f3a364570015e11", utils.SHA1: "99fd22ea2fe9c2c48124e741881fc3a555458a7e", utils.SHA256: "e2299f3c4e1f22d36befba191a347783dc2047e8e38cf6b9b96c273090f6e25b"}},
		{buff, filepath.Join(pwd, "testdata"), "myproject.com/module/name", "v1.0.0", []string{"testdata/dir3/*"}, map[utils.Algorithm]string{utils.MD5: "c2a2dd6a7af84c2d88a48caf0c3aec34", utils.SHA1: "193d761317a602d18566561678b7bddc4773385c", utils.SHA256: "3efcd8b0d88081ec64333ff98b43616d283c4d52ed26cd7c8df646d9ea452c31"}},
		{buff, filepath.Join(pwd, "testdata"), "myproject.com/module/name", "v1.0.0", []string{"*.txt"}, map[utils.Algorithm]string{utils.MD5: "e93953b4be84d7753e0f33589b7dc4ba", utils.SHA1: "280c7492f57262b6e0af56b06c9db6a128e32ab9", utils.SHA256: "e7357986c59bf670af1e2f4868edb1406a87d328b7681b15cf038491cdc7e88c"}},
	}
	for _, testData := range archiveWithExclusion {
		err = archiveProject(testData.buff, testData.filePath, testData.mod, testData.version, testData.excludedPatterns)
		assert.NoError(t, err)
		actual, err := utils.CalcChecksums(buff)
		assert.NoError(t, err)

		if !reflect.DeepEqual(testData.expected, actual) {
			t.Errorf("Expecting: %v, Got: %v", testData.expected, actual)
		}
	}
	tests.RenamePath(dotGitPath, filepath.Join(baseDir, originalFolder), t)
}
