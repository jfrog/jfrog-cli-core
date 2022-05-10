package spec

import (
	"strconv"
)

type builder struct {
	pattern             string
	exclusions          []string
	target              string
	explode             string
	props               string
	targetProps         string
	excludeProps        string
	sortOrder           string
	sortBy              []string
	offset              int
	limit               int
	build               string
	project             string
	excludeArtifacts    bool
	includeDeps         bool
	bundle              string
	publicGpgKey        string
	recursive           bool
	flat                bool
	regexp              bool
	ant                 bool
	includeDirs         bool
	archiveEntries      string
	validateSymlinks    bool
	symlinks            bool
	archive             string
	transitive          bool
	targetPathInArchive string
}

func NewBuilder() *builder {
	return &builder{}
}

func (b *builder) Pattern(pattern string) *builder {
	b.pattern = pattern
	return b
}

func (b *builder) ArchiveEntries(archiveEntries string) *builder {
	b.archiveEntries = archiveEntries
	return b
}

func (b *builder) Exclusions(exclusions []string) *builder {
	b.exclusions = exclusions
	return b
}

func (b *builder) Target(target string) *builder {
	b.target = target
	return b
}

func (b *builder) Explode(explode string) *builder {
	b.explode = explode
	return b
}

func (b *builder) Props(props string) *builder {
	b.props = props
	return b
}

func (b *builder) TargetProps(targetProps string) *builder {
	b.targetProps = targetProps
	return b
}

func (b *builder) ExcludeProps(excludeProps string) *builder {
	b.excludeProps = excludeProps
	return b
}

func (b *builder) SortOrder(sortOrder string) *builder {
	b.sortOrder = sortOrder
	return b
}

func (b *builder) SortBy(sortBy []string) *builder {
	b.sortBy = sortBy
	return b
}

func (b *builder) Offset(offset int) *builder {
	b.offset = offset
	return b
}

func (b *builder) Limit(limit int) *builder {
	b.limit = limit
	return b
}

func (b *builder) Build(build string) *builder {
	b.build = build
	return b
}

func (b *builder) Project(project string) *builder {
	b.project = project
	return b
}

func (b *builder) ExcludeArtifacts(excludeArtifacts bool) *builder {
	b.excludeArtifacts = excludeArtifacts
	return b
}

func (b *builder) IncludeDeps(includeDeps bool) *builder {
	b.includeDeps = includeDeps
	return b
}

func (b *builder) Bundle(bundle string) *builder {
	b.bundle = bundle
	return b
}

func (b *builder) PublicGpgKey(gpgKey string) *builder {
	b.publicGpgKey = gpgKey
	return b
}

func (b *builder) Archive(archive string) *builder {
	b.archive = archive
	return b
}

func (b *builder) TargetPathInArchive(targetPathInArchive string) *builder {
	b.targetPathInArchive = targetPathInArchive
	return b
}

func (b *builder) Recursive(recursive bool) *builder {
	b.recursive = recursive
	return b
}

func (b *builder) Flat(flat bool) *builder {
	b.flat = flat
	return b
}

func (b *builder) Regexp(regexp bool) *builder {
	b.regexp = regexp
	return b
}

func (b *builder) Ant(ant bool) *builder {
	b.ant = ant
	return b
}

func (b *builder) IncludeDirs(includeDirs bool) *builder {
	b.includeDirs = includeDirs
	return b
}

func (b *builder) ValidateSymlinks(validateSymlinks bool) *builder {
	b.validateSymlinks = validateSymlinks
	return b
}

func (b *builder) Symlinks(symlinks bool) *builder {
	b.symlinks = symlinks
	return b
}

func (b *builder) Transitive(transitive bool) *builder {
	b.transitive = transitive
	return b
}

func (b *builder) BuildSpec() *SpecFiles {
	return &SpecFiles{
		Files: []File{
			{
				Pattern:             b.pattern,
				Exclusions:          b.exclusions,
				Target:              b.target,
				Props:               b.props,
				TargetProps:         b.targetProps,
				ExcludeProps:        b.excludeProps,
				SortOrder:           b.sortOrder,
				SortBy:              b.sortBy,
				Offset:              b.offset,
				Limit:               b.limit,
				Build:               b.build,
				Project:             b.project,
				Bundle:              b.bundle,
				PublicGpgKey:        b.publicGpgKey,
				Explode:             b.explode,
				ArchiveEntries:      b.archiveEntries,
				Archive:             b.archive,
				TargetPathInArchive: b.targetPathInArchive,
				Recursive:           strconv.FormatBool(b.recursive),
				Flat:                strconv.FormatBool(b.flat),
				Regexp:              strconv.FormatBool(b.regexp),
				Ant:                 strconv.FormatBool(b.ant),
				IncludeDirs:         strconv.FormatBool(b.includeDirs),
				ValidateSymlinks:    strconv.FormatBool(b.validateSymlinks),
				ExcludeArtifacts:    strconv.FormatBool(b.excludeArtifacts),
				IncludeDeps:         strconv.FormatBool(b.includeDeps),
				Symlinks:            strconv.FormatBool(b.symlinks),
				Transitive:          strconv.FormatBool(b.transitive),
			},
		},
	}
}
