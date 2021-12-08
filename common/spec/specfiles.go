package spec

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	clientutils "github.com/jfrog/jfrog-client-go/utils"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

type SpecFiles struct {
	Files []File
}

func (spec *SpecFiles) Get(index int) *File {
	if index < len(spec.Files) {
		return &spec.Files[index]
	}
	return new(File)
}

func CreateSpecFromFile(specFilePath string, specVars map[string]string) (spec *SpecFiles, err error) {
	spec = new(SpecFiles)
	content, err := fileutils.ReadFile(specFilePath)
	if errorutils.CheckError(err) != nil {
		return
	}

	if len(specVars) > 0 {
		content = coreutils.ReplaceVars(content, specVars)
	}

	err = json.Unmarshal(content, spec)
	if errorutils.CheckError(err) != nil {
		return
	}
	return
}

type File struct {
	Aql              utils.Aql
	Pattern          string
	Exclusions       []string
	Target           string
	Explode          string
	Props            string
	TargetProps      string
	ExcludeProps     string
	SortOrder        string
	SortBy           []string
	Offset           int
	Limit            int
	Build            string
	Project          string
	ExcludeArtifacts string
	IncludeDeps      string
	Bundle           string
	PublicGpgKey     string `json:"gpg-key,omitempty"`
	Recursive        string
	Flat             string
	Regexp           string
	Ant              string
	IncludeDirs      string
	ArchiveEntries   string
	ValidateSymlinks string
	Archive          string
	Symlinks         string
	Transitive       string
}

func (f File) IsFlat(defaultValue bool) (bool, error) {
	return clientutils.StringToBool(f.Flat, defaultValue)
}

func (f File) IsExplode(defaultValue bool) (bool, error) {
	return clientutils.StringToBool(f.Explode, defaultValue)
}

func (f File) IsRegexp(defaultValue bool) (bool, error) {
	return clientutils.StringToBool(f.Regexp, defaultValue)
}

func (f File) IsAnt(defaultValue bool) (bool, error) {
	return clientutils.StringToBool(f.Ant, defaultValue)
}

func (f File) IsRecursive(defaultValue bool) (bool, error) {
	return clientutils.StringToBool(f.Recursive, defaultValue)
}

func (f File) IsIncludeDirs(defaultValue bool) (bool, error) {
	return clientutils.StringToBool(f.IncludeDirs, defaultValue)
}

func (f File) IsVlidateSymlinks(defaultValue bool) (bool, error) {
	return clientutils.StringToBool(f.ValidateSymlinks, defaultValue)
}

func (f File) IsExcludeArtifacts(defaultValue bool) (bool, error) {
	return clientutils.StringToBool(f.ExcludeArtifacts, defaultValue)
}

func (f File) IsIncludeDeps(defaultValue bool) (bool, error) {
	return clientutils.StringToBool(f.IncludeDeps, defaultValue)
}

func (f File) IsSymlinks(defaultValue bool) (bool, error) {
	return clientutils.StringToBool(f.Symlinks, defaultValue)
}

func (f File) IsTransitive(defaultValue bool) (bool, error) {
	return clientutils.StringToBool(f.Transitive, defaultValue)
}

func (f File) GetPatternType() (patternType clientutils.PatternType) {
	if regex, _ := f.IsRegexp(false); regex {
		return clientutils.RegExp
	}
	if ant, _ := f.IsAnt(false); ant {
		return clientutils.AntPattern
	}
	return clientutils.WildCardPattern
}

func (f File) GetPublicGpgKey() string {
	return f.PublicGpgKey
}

func (f *File) ToCommonParams() (*utils.CommonParams, error) {
	var err error
	params := new(utils.CommonParams)
	params.TargetProps, err = utils.ParseProperties(f.TargetProps)
	if err != nil {
		return nil, err
	}

	params.Aql = f.Aql
	params.Pattern = f.Pattern
	params.Exclusions = f.Exclusions
	params.Target = f.Target
	params.Props = f.Props
	params.ExcludeProps = f.ExcludeProps
	params.Build = f.Build
	params.Project = f.Project
	params.Bundle = f.Bundle
	params.SortOrder = f.SortOrder
	params.SortBy = f.SortBy
	params.Offset = f.Offset
	params.Limit = f.Limit
	params.ArchiveEntries = f.ArchiveEntries
	return params, nil
}

func ValidateSpec(files []File, isTargetMandatory, isSearchBasedSpec, isUpload bool) error {
	if len(files) == 0 {
		return errors.New("Spec must include at least one file group")
	}

	for _, file := range files {
		isAql := len(file.Aql.ItemsFind) > 0
		isPattern := len(file.Pattern) > 0
		isExclusions := len(file.Exclusions) > 0 && len(file.Exclusions[0]) > 0
		isTarget := len(file.Target) > 0
		isSortOrder := len(file.SortOrder) > 0
		isSortBy := len(file.SortBy) > 0
		isBuild := len(file.Build) > 0
		isExcludeArtifacts, _ := file.IsExcludeArtifacts(false)
		isIncludeDeps, _ := file.IsIncludeDeps(false)
		isBundle := len(file.Bundle) > 0
		isGPGKey := len(file.PublicGpgKey) > 0
		isOffset := file.Offset > 0
		isLimit := file.Limit > 0
		isValidSortOrder := file.SortOrder == "asc" || file.SortOrder == "desc"
		isExcludeProps := len(file.ExcludeProps) > 0
		isArchive := len(file.Archive) > 0
		isValidArchive := file.Archive == "zip"
		isSymlinks, _ := file.IsSymlinks(false)
		isRegexp := file.Regexp == "true"
		isAnt := file.Ant == "true"
		isExplode, _ := file.IsExplode(false)

		if isTargetMandatory && !isTarget {
			return errors.New("Spec must include target.")
		}
		if !isSearchBasedSpec && !isPattern {
			return errors.New("Spec must include a pattern.")
		}
		if isBuild && isBundle {
			return fileSpecValidationError("build", "bundle")
		}
		if isSearchBasedSpec {
			if !isAql && !isPattern && !isBuild && !isBundle {
				return errors.New("Spec must include either aql, pattern, build or bundle.")
			}
			if isOffset {
				if isBuild {
					return fileSpecValidationError("build", "offset")
				}
				if isBundle {
					return fileSpecValidationError("bundle", "offset")
				}
			}
			if isLimit {
				if isBuild {
					return fileSpecValidationError("build", "limit")
				}
				if isBundle {
					return fileSpecValidationError("bundle", "limit")
				}
			}
		}
		if isAql && isPattern {
			return fileSpecValidationError("aql", "pattern")
		}
		if isAql && isExclusions {
			return fileSpecValidationError("aql", "exclusions")
		}
		if isAql && isExcludeProps {
			return fileSpecValidationError("aql", "excludeProps")
		}
		if !isSortBy && isSortOrder {
			return errors.New("Spec cannot include 'sort-order' if 'sort-by' is not included")
		}
		if isSortOrder && !isValidSortOrder {
			return errors.New("The value of 'sort-order' can only be 'asc' or 'desc'.")
		}
		if !isBuild && (isExcludeArtifacts || isIncludeDeps) {
			return errors.New("Spec cannot include 'exclude-artifacts' or 'include-deps' if 'build' is not included.")
		}
		if isRegexp && isAnt {
			return errors.New("Can not use the option of regexp and ant together.")
		}
		if isArchive && isSymlinks && isExplode {
			return errors.New("Symlinks cannot be stored in an archive that will be exploded in artifactory.\\nWhen uploading a symlink to Artifactory, the symlink is represented in Artifactory as 0 size filewith properties describing the symlink.\\nThis symlink representation is not yet supported by Artifactory when exploding symlinks from a zip.")
		}
		if isArchive && !isValidArchive {
			return errors.New("The value of 'archive' (if provided) must be 'zip'.")
		}
		if isGPGKey && !isBundle {
			return errors.New("Spec cannot include 'gpg-key' if 'bundle' is not included.")
		}
	}
	return nil
}

func fileSpecValidationError(fieldA, fieldB string) error {
	return errors.New(fmt.Sprintf("Spec cannot include both '%s' and '%s.'", fieldA, fieldB))
}
