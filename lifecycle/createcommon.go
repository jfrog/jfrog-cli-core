package lifecycle

import (
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/utils/config"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/lifecycle/services"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
)

type ReleaseBundleCreateCommand struct {
	releaseBundleCmd
	signingKeyName string
	spec           *spec.SpecFiles
	// Backward compatibility:
	buildsSpecPath         string
	releaseBundlesSpecPath string
}

func NewReleaseBundleCreateCommand() *ReleaseBundleCreateCommand {
	return &ReleaseBundleCreateCommand{}
}

func (rbc *ReleaseBundleCreateCommand) SetServerDetails(serverDetails *config.ServerDetails) *ReleaseBundleCreateCommand {
	rbc.serverDetails = serverDetails
	return rbc
}

func (rbc *ReleaseBundleCreateCommand) SetReleaseBundleName(releaseBundleName string) *ReleaseBundleCreateCommand {
	rbc.releaseBundleName = releaseBundleName
	return rbc
}

func (rbc *ReleaseBundleCreateCommand) SetReleaseBundleVersion(releaseBundleVersion string) *ReleaseBundleCreateCommand {
	rbc.releaseBundleVersion = releaseBundleVersion
	return rbc
}

func (rbc *ReleaseBundleCreateCommand) SetSigningKeyName(signingKeyName string) *ReleaseBundleCreateCommand {
	rbc.signingKeyName = signingKeyName
	return rbc
}

func (rbc *ReleaseBundleCreateCommand) SetSync(sync bool) *ReleaseBundleCreateCommand {
	rbc.sync = sync
	return rbc
}

func (rbc *ReleaseBundleCreateCommand) SetReleaseBundleProject(rbProjectKey string) *ReleaseBundleCreateCommand {
	rbc.rbProjectKey = rbProjectKey
	return rbc
}

func (rbc *ReleaseBundleCreateCommand) SetSpec(spec *spec.SpecFiles) *ReleaseBundleCreateCommand {
	rbc.spec = spec
	return rbc
}

// Deprecated
func (rbc *ReleaseBundleCreateCommand) SetBuildsSpecPath(buildsSpecPath string) *ReleaseBundleCreateCommand {
	rbc.buildsSpecPath = buildsSpecPath
	return rbc
}

// Deprecated
func (rbc *ReleaseBundleCreateCommand) SetReleaseBundlesSpecPath(releaseBundlesSpecPath string) *ReleaseBundleCreateCommand {
	rbc.releaseBundlesSpecPath = releaseBundlesSpecPath
	return rbc
}

func (rbc *ReleaseBundleCreateCommand) CommandName() string {
	return "rb_create"
}

func (rbc *ReleaseBundleCreateCommand) ServerDetails() (*config.ServerDetails, error) {
	return rbc.serverDetails, nil
}

func (rbc *ReleaseBundleCreateCommand) Run() error {
	if err := validateArtifactoryVersionSupported(rbc.serverDetails); err != nil {
		return err
	}

	servicesManager, rbDetails, queryParams, err := rbc.getPrerequisites()
	if err != nil {
		return err
	}

	sourceType, err := rbc.identifySourceType()
	if err != nil {
		return err
	}

	switch sourceType {
	case services.Aql:
		return rbc.createFromAql(servicesManager, rbDetails, queryParams)
	case services.Artifacts:
		return rbc.createFromArtifacts(servicesManager, rbDetails, queryParams)
	case services.Builds:
		return rbc.createFromBuilds(servicesManager, rbDetails, queryParams)
	case services.ReleaseBundles:
		return rbc.createFromReleaseBundles(servicesManager, rbDetails, queryParams)
	default:
		return errorutils.CheckErrorf("unknown source for release bundle creation was provided")
	}
}

func (rbc *ReleaseBundleCreateCommand) identifySourceType() (services.SourceType, error) {
	switch {
	case rbc.buildsSpecPath != "":
		return services.Builds, nil
	case rbc.releaseBundlesSpecPath != "":
		return services.ReleaseBundles, nil
	case rbc.spec != nil:
		return validateAndIdentifyRbCreationSpec(rbc.spec.Files)
	default:
		return "", errorutils.CheckErrorf("a spec file input is mandatory")
	}
}

func validateAndIdentifyRbCreationSpec(files []spec.File) (services.SourceType, error) {
	if len(files) == 0 {
		return "", errorutils.CheckErrorf("spec must include at least one file group")
	}

	var detectedCreationSources []services.SourceType
	for _, file := range files {
		sourceType, err := validateFile(file)
		if err != nil {
			return "", err
		}
		detectedCreationSources = append(detectedCreationSources, sourceType)
	}

	if err := validateCreationSources(detectedCreationSources); err != nil {
		return "", err
	}
	return detectedCreationSources[0], nil
}

func validateCreationSources(detectedCreationSources []services.SourceType) error {
	if len(detectedCreationSources) == 0 {
		return errorutils.CheckErrorf("unexpected err while validating spec - could not detect any creation sources")
	}

	// Assert single creation source.
	for i := 1; i < len(detectedCreationSources); i++ {
		if detectedCreationSources[i] != detectedCreationSources[0] {
			return generateSingleCreationSourceErr(detectedCreationSources)
		}
	}

	// If aql, assert single file.
	if detectedCreationSources[0] == services.Aql && len(detectedCreationSources) > 1 {
		return errorutils.CheckErrorf("only a single aql query can be provided")
	}
	return nil
}

func generateSingleCreationSourceErr(detectedCreationSources []services.SourceType) error {
	var detectedStr []string
	for _, source := range detectedCreationSources {
		detectedStr = append(detectedStr, string(source))
	}
	return errorutils.CheckErrorf(
		"multiple creation sources were detected in separate spec files. Only a single creation source should be provided. Detected: '%s'",
		coreutils.ListToText(detectedStr))
}

func validateFile(file spec.File) (services.SourceType, error) {
	// Aql creation source:
	isAql := len(file.Aql.ItemsFind) > 0

	// Build creation source:
	isBuild := len(file.Build) > 0
	isIncludeDeps, _ := file.IsIncludeDeps(false)

	// Bundle creation source:
	isBundle := len(file.Bundle) > 0

	// Build & bundle:
	isProject := len(file.Project) > 0

	// Artifacts creation source:
	isPattern := len(file.Pattern) > 0
	isExclusions := len(file.Exclusions) > 0 && len(file.Exclusions[0]) > 0
	isProps := len(file.Props) > 0
	isExcludeProps := len(file.ExcludeProps) > 0
	isRecursive, _ := file.IsRecursive(true)

	// Unsupported:
	isPathMapping := len(file.PathMapping.Input) > 0 && len(file.PathMapping.Output) > 0
	isTarget := len(file.Target) > 0
	isSortOrder := len(file.SortOrder) > 0
	isSortBy := len(file.SortBy) > 0
	isExcludeArtifacts, _ := file.IsExcludeArtifacts(false)
	isGPGKey := len(file.PublicGpgKey) > 0
	isOffset := file.Offset > 0
	isLimit := file.Limit > 0
	isArchive := len(file.Archive) > 0
	isSymlinks, _ := file.IsSymlinks(false)
	isRegexp := file.Regexp == "true"
	isAnt := file.Ant == "true"
	isExplode, _ := file.IsExplode(false)
	isBypassArchiveInspection, _ := file.IsBypassArchiveInspection(false)
	isTransitive, _ := file.IsTransitive(false)

	if isPathMapping || isTarget || isSortOrder || isSortBy || isExcludeArtifacts || isGPGKey || isOffset || isLimit ||
		isSymlinks || isArchive || isAnt || isRegexp || isExplode || isBypassArchiveInspection || isTransitive {
		return "", errorutils.CheckErrorf("unsupported fields were provided in file spec. " +
			"release bundle creation file spec only supports the following fields: " +
			"'aql', 'build', 'includeDeps', 'bundle', 'project', 'pattern', 'exclusions', 'props', 'excludeProps' and 'recursive'")
	}
	if coreutils.SumTrueValues([]bool{isAql, isBuild, isBundle, isPattern}) != 1 {
		return "", errorutils.CheckErrorf("exactly one creation source is supported (aql, builds, release bundles or pattern (artifacts))")
	}

	switch {
	case isAql:
		return services.Aql,
			validateCreationSource([]bool{isIncludeDeps, isProject, isExclusions, isProps, isExcludeProps, !isRecursive},
				"aql creation source supports no other fields")
	case isBuild:
		return services.Builds,
			validateCreationSource([]bool{isExclusions, isProps, isExcludeProps, !isRecursive},
				"builds creation source only supports the 'includeDeps' and 'project' fields")
	case isBundle:
		return services.ReleaseBundles,
			validateCreationSource([]bool{isIncludeDeps, isExclusions, isProps, isExcludeProps, !isRecursive},
				"release bundles creation source only supports the 'project' field")
	case isPattern:
		return services.Artifacts,
			validateCreationSource([]bool{isIncludeDeps, isProject},
				"release bundles creation source only supports the 'exclusions', 'props', 'excludeProps' and 'recursive' fields")
	default:
		return "", errorutils.CheckErrorf("unexpected err in spec validation")
	}
}

func validateCreationSource(unsupportedFields []bool, errMsg string) error {
	if coreutils.SumTrueValues(unsupportedFields) > 0 {
		return errorutils.CheckErrorf(errMsg)
	}
	return nil
}
