package transferfiles

import (
	"path"
	"regexp"
	"strings"

	"github.com/jfrog/jfrog-client-go/artifactory/services/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	// cocoapodsSpecsDir is the hidden index directory Artifactory writes generated podspec
	// copies to (CocoaPodsConstants.SPECS_PATH in artifactory-service). Real user-uploaded
	// podspecs live under the repo's visible "Specs/" or "pods/" layout instead.
	cocoapodsSpecsDir = ".specs"
	// swiftCacheDir is the hidden cache directory Artifactory writes generated Swift registry
	// metadata to (SwiftConstants.CACHE_DIRECTORY in artifactory-service). Real uploaded source
	// archives and package metadata live outside it.
	swiftCacheDir = ".swift"
)

var (
	yumRepodataPattern = regexp.MustCompile(`(^|.*/)repodata($|/.*)`)
	yumTmpPattern      = regexp.MustCompile(`(^|.*/)_tmp_\d{13,}($|/.*)`)
	condaTmpPattern    = regexp.MustCompile(`(^|.*/)_tmp-repodata-\d{13,}-.*`)
	swiftManifestFile  = regexp.MustCompile(`\APackage(@swift-(\d+)(?:\.(\d+))?(?:\.(\d+))?)?\.swift\z`)
	swiftManifestURL   = regexp.MustCompile(`\APackage\.swift\?swift-version=(\d+)(?:\.(\d+))?(?:\.(\d+))?\z`)
)

// shouldExcludeGeneratedFile mirrors Artifactory isLocalGenerated checkers (and the older
// data-transfer PathPropsFilter table). Used when the target is older than 7.55 and
// POST /api/localgenerated/filter/paths is unavailable.
func shouldExcludeGeneratedFile(packageType, pathInRepo string) bool {
	if pathInRepo == "" || pathInRepo == "." {
		return false
	}
	// InternalMetadataChecker: .jfrog/ is local-generated for every package type.
	if isInternalMetadataPath(pathInRepo) {
		return true
	}
	switch strings.ToLower(packageType) {
	case "p2", "ivy", "sbt", "maven", "gradle":
		return isMavenGeneratedPath(pathInRepo)
	case "npm":
		return pathInRepo == ".npm" || strings.HasPrefix(pathInRepo, ".npm/")
	case "pub":
		return pathInRepo == ".pub" || strings.HasPrefix(pathInRepo, ".pub/")
	case "rpm", "yum":
		return yumRepodataPattern.MatchString(pathInRepo) || yumTmpPattern.MatchString(pathInRepo)
	case "gems":
		return isGemsGeneratedPath(pathInRepo)
	case "helm":
		return strings.HasSuffix(pathInRepo, "index.yaml")
	case "opkg":
		return isOpkgGeneratedPath(pathInRepo)
	case "pypi":
		return pathInRepo == ".pypi/simple.html"
	case "cargo":
		return strings.HasPrefix(pathInRepo, ".git/") ||
			pathInRepo == "config.json" ||
			pathInRepo == "config.original.json" ||
			strings.HasPrefix(pathInRepo, "index/")
	case "conan":
		return strings.HasSuffix(pathInRepo, "index.json") ||
			strings.HasSuffix(pathInRepo, ".conan/packages.ref.json")
	case "conda":
		return strings.HasSuffix(pathInRepo, "current_repodata.json") ||
			strings.HasSuffix(pathInRepo, "repodata.json") ||
			condaTmpPattern.MatchString(pathInRepo)
	case "nuget":
		return strings.HasPrefix(pathInRepo, ".nuget") || strings.HasPrefix(pathInRepo, ".nuGetV3/")
	case "swift":
		return isUnderDir(pathInRepo, swiftCacheDir) &&
			(strings.HasSuffix(pathInRepo, "releases.json") ||
				strings.HasSuffix(pathInRepo, "release_info.json") ||
				isSwiftGeneratedManifest(pathInRepo))
	case "alpine":
		return strings.HasSuffix(pathInRepo, "APKINDEX.tar.gz")
	case "debian":
		return isDebianGeneratedPath(pathInRepo)
	case "docker", "oci", "helmoci":
		return isDockerGeneratedPath(pathInRepo)
	case "puppet":
		return strings.HasPrefix(pathInRepo, ".puppet")
	case "composer":
		return isComposerGeneratedPath(pathInRepo)
	case "cocoapods":
		return isUnderDir(pathInRepo, cocoapodsSpecsDir) &&
			(strings.HasSuffix(pathInRepo, ".podspec.json") || strings.HasSuffix(pathInRepo, ".podspec"))
	case "terraform":
		return strings.HasSuffix(pathInRepo, "module.json")
	case "releasebundles":
		return strings.HasSuffix(pathInRepo, ".json.draft")
	default:
		return false
	}
}

func isInternalMetadataPath(pathInRepo string) bool {
	return pathInRepo == ".jfrog" || strings.HasPrefix(pathInRepo, ".jfrog/")
}

// isUnderDir reports whether dir appears as an actual path segment of pathInRepo, rather than
// merely as a substring, so a hidden generated-file directory (e.g. ".specs", ".swift") never
// matches a real file that merely happens to contain that text in its name.
func isUnderDir(pathInRepo, dir string) bool {
	return pathInRepo == dir || strings.HasPrefix(pathInRepo, dir+"/") || strings.Contains(pathInRepo, "/"+dir+"/")
}

func isDockerGeneratedPath(pathInRepo string) bool {
	return strings.HasSuffix(pathInRepo, "repository.catalog") ||
		strings.HasSuffix(pathInRepo, "repository_v2.catalog") ||
		pathInRepo == "_uploads" ||
		strings.HasPrefix(pathInRepo, "_uploads/") ||
		strings.Contains(pathInRepo, "/_uploads/")
}

func isMavenGeneratedPath(pathInRepo string) bool {
	fileName := path.Base(pathInRepo)
	return strings.HasPrefix(fileName, "nexus-maven-repository-index") ||
		fileName == "maven-metadata.xml" ||
		strings.HasSuffix(fileName, ":maven-metadata.xml")
}

func isGemsGeneratedPath(pathInRepo string) bool {
	switch pathInRepo {
	case "specs.4.8.gz", "latest_specs.4.8.gz", "prerelease_specs.4.8.gz", "versions":
		return true
	}
	return strings.HasPrefix(pathInRepo, "info/") || strings.HasSuffix(pathInRepo, ".gemspec.rz")
}

func isOpkgGeneratedPath(pathInRepo string) bool {
	switch strings.ToLower(path.Base(pathInRepo)) {
	case "packages", "packages.gz", "packages.bz2", "packages.xz", "packages.lzma",
		"packages.stamps", "packages.sig", "packages.gz.sig":
		return true
	default:
		return false
	}
}

func isComposerGeneratedPath(pathInRepo string) bool {
	if pathInRepo == ".composer/packages.json" {
		return true
	}
	if strings.HasPrefix(pathInRepo, ".composer/packages/") && strings.HasSuffix(pathInRepo, "list.json") {
		return true
	}
	return (strings.HasPrefix(pathInRepo, ".composer/p/") || strings.HasPrefix(pathInRepo, ".composer/p2/")) &&
		strings.HasSuffix(pathInRepo, ".json")
}

func isDebianGeneratedPath(pathInRepo string) bool {
	if !isDebianDistPath(pathInRepo) {
		return false
	}
	lower := strings.ToLower(pathInRepo)
	return !strings.HasSuffix(lower, ".deb") && !strings.HasSuffix(lower, ".ddeb")
}

func isDebianDistPath(pathInRepo string) bool {
	return pathInRepo == "dists" || strings.HasPrefix(pathInRepo, "dists/") || strings.Contains(pathInRepo, "/dists/")
}

func isSwiftGeneratedManifest(pathInRepo string) bool {
	fileName := path.Base(pathInRepo)
	return swiftManifestFile.MatchString(fileName) || swiftManifestURL.MatchString(fileName)
}

func filterGeneratedPathsByPackageType(aqlResultItems []utils.ResultItem, packageType string) []utils.ResultItem {
	if packageType == "" {
		return aqlResultItems
	}
	kept := make([]utils.ResultItem, 0, len(aqlResultItems))
	for i := range aqlResultItems {
		pathInRepo := getPathInRepo(&aqlResultItems[i])
		if shouldExcludeGeneratedFile(packageType, pathInRepo) {
			log.Debug("Excluding locally generated item from being transferred:", pathInRepo)
			continue
		}
		kept = append(kept, aqlResultItems[i])
	}
	return kept
}
