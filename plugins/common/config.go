package common

import (
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/common/cliutils"
	"github.com/jfrog/jfrog-cli-core/v2/common/spec"
	"github.com/jfrog/jfrog-cli-core/v2/plugins/components"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
)

// Returns build configuration struct using the args (build name/number) and options (project) provided by the user.
// Any empty configuration could be later overridden by environment variables if set.
func CreateBuildConfiguration(c *components.Context) *utils.BuildConfiguration {
	buildConfiguration := new(utils.BuildConfiguration)
	buildNameArg, buildNumberArg := c.Arguments[0], c.Arguments[1]
	if buildNameArg == "" || buildNumberArg == "" {
		buildNameArg = ""
		buildNumberArg = ""
	}
	buildConfiguration.SetBuildName(buildNameArg).SetBuildNumber(buildNumberArg).SetProject(c.GetStringFlagValue("project")).SetModule(c.GetStringFlagValue("module"))
	return buildConfiguration
}

func FixWinPathsForFileSystemSourcedCmds(uploadSpec *spec.SpecFiles, c *components.Context) {
	cliutils.FixWinPathsForFileSystemSourcedCmds(uploadSpec, c.IsFlagSet("spec"), c.IsFlagSet("exclusions"))
}

func GetFileSystemSpec(c *components.Context) (fsSpec *spec.SpecFiles, err error) {
	fsSpec, err = spec.CreateSpecFromFile(c.GetStringFlagValue("spec"), coreutils.SpecVarsStringToMap(c.GetStringFlagValue("spec-vars")))
	if err != nil {
		return
	}
	// Override spec with CLI options
	for i := 0; i < len(fsSpec.Files); i++ {
		fsSpec.Get(i).Target = strings.TrimPrefix(fsSpec.Get(i).Target, "/")
		OverrideSpecFieldsIfSet(fsSpec.Get(i), c)
	}
	return
}

func OverrideSpecFieldsIfSet(spec *spec.File, c *components.Context) {
	OverrideArrayIfSet(&spec.Exclusions, c, "exclusions")
	OverrideArrayIfSet(&spec.SortBy, c, "sort-by")
	OverrideIntIfSet(&spec.Offset, c, "offset")
	OverrideIntIfSet(&spec.Limit, c, "limit")
	OverrideStringIfSet(&spec.SortOrder, c, "sort-order")
	OverrideStringIfSet(&spec.Props, c, "props")
	OverrideStringIfSet(&spec.TargetProps, c, "target-props")
	OverrideStringIfSet(&spec.ExcludeProps, c, "exclude-props")
	OverrideStringIfSet(&spec.Build, c, "build")
	OverrideStringIfSet(&spec.Project, c, "project")
	OverrideStringIfSet(&spec.ExcludeArtifacts, c, "exclude-artifacts")
	OverrideStringIfSet(&spec.IncludeDeps, c, "include-deps")
	OverrideStringIfSet(&spec.Bundle, c, "bundle")
	OverrideStringIfSet(&spec.Recursive, c, "recursive")
	OverrideStringIfSet(&spec.Flat, c, "flat")
	OverrideStringIfSet(&spec.Explode, c, "explode")
	OverrideStringIfSet(&spec.BypassArchiveInspection, c, "bypass-archive-inspection")
	OverrideStringIfSet(&spec.Regexp, c, "regexp")
	OverrideStringIfSet(&spec.IncludeDirs, c, "include-dirs")
	OverrideStringIfSet(&spec.ValidateSymlinks, c, "validate-symlinks")
	OverrideStringIfSet(&spec.Symlinks, c, "symlinks")
	OverrideStringIfSet(&spec.Transitive, c, "transitive")
	OverrideStringIfSet(&spec.PublicGpgKey, c, "gpg-key")
}
