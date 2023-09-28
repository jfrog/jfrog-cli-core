package jas

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	jfrogappsconfig "github.com/jfrog/jfrog-apps-config/go"
	clientTestUtils "github.com/jfrog/jfrog-client-go/utils/tests"

	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/stretchr/testify/assert"
)

var createJFrogAppsConfigCases = []struct {
	workingDirs []string
}{
	{workingDirs: []string{}},
	{workingDirs: []string{"working-dir"}},
	{workingDirs: []string{"working-dir-1", "working-dir-2"}},
}

func TestCreateJFrogAppsConfig(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)

	for _, testCase := range createJFrogAppsConfigCases {
		t.Run(fmt.Sprintf("%v", testCase.workingDirs), func(t *testing.T) {
			jfrogAppsConfig, err := createJFrogAppsConfig(testCase.workingDirs)
			assert.NoError(t, err)
			assert.NotNil(t, jfrogAppsConfig)
			if len(testCase.workingDirs) == 0 {
				assert.Len(t, jfrogAppsConfig.Modules, 1)
				assert.Equal(t, wd, jfrogAppsConfig.Modules[0].SourceRoot)
				return
			}
			assert.Len(t, jfrogAppsConfig.Modules, len(testCase.workingDirs))
			for i, workingDir := range testCase.workingDirs {
				assert.Equal(t, filepath.Join(wd, workingDir), jfrogAppsConfig.Modules[i].SourceRoot)
			}
		})
	}
}

func TestCreateJFrogAppsConfigWithConfig(t *testing.T) {
	wd, err := os.Getwd()
	assert.NoError(t, err)
	chdirCallback := clientTestUtils.ChangeDirWithCallback(t, wd, "testdata")
	defer chdirCallback()

	jfrogAppsConfig, err := createJFrogAppsConfig([]string{})
	assert.NoError(t, err)
	assert.NotNil(t, jfrogAppsConfig)
	assert.Equal(t, "1.0", jfrogAppsConfig.Version)
	assert.Len(t, jfrogAppsConfig.Modules, 1)
}

func TestShouldSkipScanner(t *testing.T) {
	module := jfrogappsconfig.Module{}
	assert.False(t, ShouldSkipScanner(module, utils.IaC))

	module = jfrogappsconfig.Module{ExcludeScanners: []string{"sast"}}
	assert.False(t, ShouldSkipScanner(module, utils.IaC))
	assert.True(t, ShouldSkipScanner(module, utils.Sast))
}

var getSourceRootsCases = []struct {
	scanner *jfrogappsconfig.Scanner
}{
	{scanner: nil},
	{&jfrogappsconfig.Scanner{WorkingDirs: []string{"working-dir"}}},
	{&jfrogappsconfig.Scanner{WorkingDirs: []string{"working-dir-1", "working-dir-2"}}},
}

func TestGetSourceRoots(t *testing.T) {
	testGetSourceRoots(t, "source-root")
}

func TestGetSourceRootsEmptySourceRoot(t *testing.T) {
	testGetSourceRoots(t, "")
}

func testGetSourceRoots(t *testing.T, sourceRoot string) {
	sourceRoot, err := filepath.Abs(sourceRoot)
	assert.NoError(t, err)
	module := jfrogappsconfig.Module{SourceRoot: sourceRoot}
	for _, testCase := range getSourceRootsCases {
		t.Run("", func(t *testing.T) {
			scanner := testCase.scanner
			actualSourceRoots, err := GetSourceRoots(module, scanner)
			assert.NoError(t, err)
			if scanner == nil {
				assert.ElementsMatch(t, []string{module.SourceRoot}, actualSourceRoots)
				return
			}
			expectedWorkingDirs := []string{}
			for _, workingDir := range scanner.WorkingDirs {
				expectedWorkingDirs = append(expectedWorkingDirs, filepath.Join(module.SourceRoot, workingDir))
			}
			assert.ElementsMatch(t, actualSourceRoots, expectedWorkingDirs)
		})
	}
}

var getExcludePatternsCases = []struct {
	scanner *jfrogappsconfig.Scanner
}{
	{scanner: nil},
	{&jfrogappsconfig.Scanner{WorkingDirs: []string{"exclude-dir"}}},
	{&jfrogappsconfig.Scanner{WorkingDirs: []string{"exclude-dir-1", "exclude-dir-2"}}},
}

func TestGetExcludePatterns(t *testing.T) {
	module := jfrogappsconfig.Module{ExcludePatterns: []string{"exclude-root"}}
	for _, testCase := range getExcludePatternsCases {
		t.Run("", func(t *testing.T) {
			scanner := testCase.scanner
			actualExcludePatterns := GetExcludePatterns(module, scanner)
			if scanner == nil {
				assert.ElementsMatch(t, module.ExcludePatterns, actualExcludePatterns)
				return
			}
			expectedExcludePatterns := module.ExcludePatterns
			expectedExcludePatterns = append(expectedExcludePatterns, scanner.ExcludePatterns...)
			assert.ElementsMatch(t, actualExcludePatterns, expectedExcludePatterns)
		})
	}
}
