package utils

import (
	"net/http"
	"strings"

	outFormat "github.com/jfrog/jfrog-cli-core/v2/common/format"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/build"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/auth"
	"github.com/jfrog/jfrog-client-go/http/httpclient"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const minSupportedArtifactoryVersionForNpmCmds = "5.5.2"

func GetArtifactoryNpmRepoDetails(repo string, authArtDetails *auth.ServiceDetails) (npmAuth, registry string, err error) {
	npmAuth, err = getNpmAuth(authArtDetails)
	if err != nil {
		return "", "", err
	}

	if err = utils.ValidateRepoExists(repo, *authArtDetails); err != nil {
		return "", "", err
	}

	registry = getNpmRepositoryUrl(repo, (*authArtDetails).GetUrl())
	return
}

func getNpmAuth(authArtDetails *auth.ServiceDetails) (npmAuth string, err error) {
	// Check Artifactory version
	err = validateArtifactoryVersionForNpmCmds(authArtDetails)
	if err != nil {
		return
	}

	// Get npm token from Artifactory
	return getNpmAuthFromArtifactory(authArtDetails)
}

func validateArtifactoryVersionForNpmCmds(artDetails *auth.ServiceDetails) error {
	// Get Artifactory version.
	versionStr, err := (*artDetails).GetVersion()
	if err != nil {
		return err
	}

	// Validate version.
	return clientutils.ValidateMinimumVersion(clientutils.Artifactory, versionStr, minSupportedArtifactoryVersionForNpmCmds)
}

func getNpmAuthFromArtifactory(artDetails *auth.ServiceDetails) (npmAuth string, err error) {
	authApiUrl := (*artDetails).GetUrl() + "api/npm/auth"
	log.Debug("Sending npm auth request")

	// Get npm token from Artifactory.
	client, err := httpclient.ClientBuilder().SetRetries(3).Build()
	if err != nil {
		return "", err
	}
	resp, body, _, err := client.SendGet(authApiUrl, true, (*artDetails).CreateHttpClientDetails(), "")
	if err != nil {
		return "", err
	}
	if err = errorutils.CheckResponseStatusWithBody(resp, body, http.StatusOK); err != nil {
		return "", err
	}

	return string(body), nil
}

func getNpmRepositoryUrl(repo, url string) string {
	if !strings.HasSuffix(url, "/") {
		url += "/"
	}
	url += "api/npm/" + repo
	return url
}

// Remove all the none npm CLI flags from args.
func ExtractNpmOptionsFromArgs(args []string) (detailedSummary, xrayScan bool, scanOutputFormat outFormat.OutputFormat, cleanArgs []string, buildConfig *build.BuildConfiguration, err error) {
	cleanArgs = append([]string(nil), args...)
	cleanArgs, detailedSummary, err = coreutils.ExtractDetailedSummaryFromArgs(cleanArgs)
	if err != nil {
		return
	}

	cleanArgs, xrayScan, err = coreutils.ExtractXrayScanFromArgs(cleanArgs)
	if err != nil {
		return
	}

	cleanArgs, format, err := coreutils.ExtractXrayOutputFormatFromArgs(cleanArgs)
	if err != nil {
		return
	}
	scanOutputFormat, err = outFormat.GetOutputFormat(format)
	if err != nil {
		return
	}
	cleanArgs, buildConfig, err = build.ExtractBuildDetailsFromArgs(cleanArgs)
	return
}
