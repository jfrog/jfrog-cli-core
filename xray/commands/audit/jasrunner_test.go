package audit

import (
	rtutils "github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-cli-core/v2/xray/commands/audit/jas"
	"github.com/jfrog/jfrog-cli-core/v2/xray/utils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"github.com/stretchr/testify/assert"
	"os"
	"testing"
)

func TestGetExtendedScanResults_AnalyzerManagerDoesntExist(t *testing.T) {
	tmpDir, err := fileutils.CreateTempDir()
	defer func() {
		assert.NoError(t, fileutils.RemoveTempDir(tmpDir))
	}()
	assert.NoError(t, err)
	assert.NoError(t, os.Setenv(coreutils.HomeDir, tmpDir))
	defer func() {
		assert.NoError(t, os.Unsetenv(coreutils.HomeDir))
	}()
	scanResults := &utils.ExtendedScanResults{XrayResults: jas.FakeBasicXrayResults, ScannedTechnologies: []coreutils.Technology{coreutils.Yarn}}
	err = runJasScannersAndSetResults(scanResults, []string{"issueId_1_direct_dependency", "issueId_2_direct_dependency"}, &jas.FakeServerDetails, nil, false,nil)
	// Expect error:
	assert.Error(t, err)
}

func TestGetExtendedScanResults_ServerNotValid(t *testing.T) {
	scanResults := &utils.ExtendedScanResults{XrayResults: jas.FakeBasicXrayResults, ScannedTechnologies: []coreutils.Technology{coreutils.Pip}}
	err := runJasScannersAndSetResults(scanResults, []string{"issueId_1_direct_dependency", "issueId_2_direct_dependency"}, nil, nil, false,nil)
	assert.NoError(t, err)
}

func TestGetExtendedScanResults_AnalyzerManagerReturnsError(t *testing.T) {
	mockDirectDependencies := []string{"issueId_2_direct_dependency", "issueId_1_direct_dependency"}
	assert.NoError(t, rtutils.DownloadAnalyzerManagerIfNeeded())
	scanResults := &utils.ExtendedScanResults{XrayResults: jas.FakeBasicXrayResults, ScannedTechnologies: []coreutils.Technology{coreutils.Yarn}}
	err := runJasScannersAndSetResults(scanResults, mockDirectDependencies, &jas.FakeServerDetails, nil, false,nil)

	// Expect error:
	assert.ErrorContains(t, err, "failed to run Applicability scan")
}
