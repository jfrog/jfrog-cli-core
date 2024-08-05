package golang

import (
	"bytes"
	"github.com/jfrog/gofrog/crypto"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"reflect"
	"testing"

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
		expected         map[crypto.Algorithm]string
	}{
		{buff, filepath.Join(pwd, "testdata"), "myproject.com/module/name", "v1.0.0", nil, map[crypto.Algorithm]string{crypto.MD5: "5b3603a7bf637622516673b845249205", crypto.SHA1: "7386685c432c39428c9cb8584a2b970139c5e626", crypto.SHA256: "eefd8aa3f9ac89876c8442d5feebbc837666bf40114d201219e3e6d51c208949"}},
		{buff, filepath.Join(pwd, "testdata"), "myproject.com/module/name", "v1.0.0", []string{"./testdata/dir1/*"}, map[crypto.Algorithm]string{crypto.MD5: "c2eeb4ef958edee91570690bf4111fc7", crypto.SHA1: "d77e10eaa9bd863a9ff3775d3e452041e6f5aa40", crypto.SHA256: "ecf66c1256263b2b4386efc299fa0c389263608efda9d1d91af8a746e6c5709a"}},
		{buff, filepath.Join(pwd, "testdata"), "myproject.com/module/name", "v1.0.0", []string{"./testdata/dir2/*"}, map[crypto.Algorithm]string{crypto.MD5: "bbe78a98ba10c1428f3a364570015e11", crypto.SHA1: "99fd22ea2fe9c2c48124e741881fc3a555458a7e", crypto.SHA256: "e2299f3c4e1f22d36befba191a347783dc2047e8e38cf6b9b96c273090f6e25b"}},
		{buff, filepath.Join(pwd, "testdata"), "myproject.com/module/name", "v1.0.0", []string{"./testdata/dir2/*", "testdata/dir3/*"}, map[crypto.Algorithm]string{crypto.MD5: "28617d6e74fce3dd2bab21b1bd65009b", crypto.SHA1: "410814fbf21afdfb9c5b550151a51c2e986447fa", crypto.SHA256: "e877c07315d6d3ad69139035defc08c04b400b36cd069b35ea3c2960424f2dc6"}},
		{buff, filepath.Join(pwd, "testdata"), "myproject.com/module/name", "v1.0.0", []string{"./testdata/dir2/*", "./testdata/dir3/dir4/*"}, map[crypto.Algorithm]string{crypto.MD5: "46a3ded48ed7998b1b35c80fbe0ffab5", crypto.SHA1: "a26e73e7d29e49dd5d9c87da8f7c93cf929750df", crypto.SHA256: "cf224b12eca12de4a052ef0f444519d64b6cecaf7b06050a02998be190e88847"}},
		{buff, filepath.Join(pwd, "testdata"), "myproject.com/module/name", "v1.0.0", []string{"./testdata/dir3/*"}, map[crypto.Algorithm]string{crypto.MD5: "c2a2dd6a7af84c2d88a48caf0c3aec34", crypto.SHA1: "193d761317a602d18566561678b7bddc4773385c", crypto.SHA256: "3efcd8b0d88081ec64333ff98b43616d283c4d52ed26cd7c8df646d9ea452c31"}},
		{buff, filepath.Join(pwd, "testdata"), "myproject.com/module/name", "v1.0.0", []string{"*.txt"}, map[crypto.Algorithm]string{crypto.MD5: "e93953b4be84d7753e0f33589b7dc4ba", crypto.SHA1: "280c7492f57262b6e0af56b06c9db6a128e32ab9", crypto.SHA256: "e7357986c59bf670af1e2f4868edb1406a87d328b7681b15cf038491cdc7e88c"}},
		{buff, filepath.Join(pwd, "testdata"), "myproject.com/module/name", "v1.0.0", []string{"./*/dir4/*.txt"}, map[crypto.Algorithm]string{crypto.MD5: "785f0c0c7b20dfd716178856edb79834", crypto.SHA1: "d07204277ece1d7bef6a9f289a56afb91d66125f", crypto.SHA256: "6afa0dd70bfa7c6d3aca1a3dfcd6465c542d64136c6391fa611795e6fa5800ce"}},
	}
	for _, testData := range archiveWithExclusion {
		err = archiveProject(testData.buff, testData.filePath, testData.mod, testData.version, testData.excludedPatterns)
		assert.NoError(t, err)
		actual, err := crypto.CalcChecksums(buff)
		assert.NoError(t, err)

		if !reflect.DeepEqual(testData.expected, actual) {
			t.Errorf("Expecting: %v, Got: %v", testData.expected, actual)
		}
	}
	tests.RenamePath(dotGitPath, filepath.Join(baseDir, originalFolder), t)
}

func TestGetAbsolutePaths(t *testing.T) {
	testData := []string{filepath.Join(".", "dir1", "*"), "*.txt", filepath.Join("*", "dir2", "*")}
	result, err := getAbsolutePaths(testData)
	assert.NoError(t, err)
	wd, err := os.Getwd()
	assert.NoError(t, err)
	expectedResults := []string{filepath.Join(wd, "dir1", "*"), filepath.Join(wd, "*.txt"), filepath.Join(wd, "*", "dir2", "*")}
	assert.ElementsMatch(t, result, expectedResults)
}
