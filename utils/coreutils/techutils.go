package coreutils

import (
	"strings"

	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

type Technology string

const (
	Maven  = "Maven"
	Gradle = "Gradle"
	Npm    = "npm"
	Yarn   = "Yarn"
	Go     = "go"
	Pip    = "pip"
	Pipenv = "pipenv"
	Nuget  = "nuget"
	Dotnet = "dotnet"
	Docker = "docker"
)

type TechData struct {
	// The name of the package type used in this technology.
	PackageType string
	// Suffixes of file/directory names that indicate if a project uses this technology.
	// The name of at least one of the files/directories in the project's directory must end with one of these suffixes.
	indicators []string
	// Suffixes of file/directory names that indicate if a project does not use this technology.
	// The names of all the files/directories in the project's directory must NOT end with any of these suffixes.
	exclude []string
	// Whether this technology is supported by the 'jf ci-setup' command.
	ciSetupSupport bool
}

var technologiesData = map[Technology]TechData{
	Maven: {
		PackageType:    "Maven",
		indicators:     []string{"pom.xml"},
		ciSetupSupport: true,
	},
	Gradle: {
		PackageType:    "Gradle",
		indicators:     []string{".gradle"},
		ciSetupSupport: true,
	},
	Npm: {
		PackageType:    "npm",
		indicators:     []string{"package.json", "package-lock.json", "npm-shrinkwrap.json"},
		exclude:        []string{".yarnrc.yml", "yarn.lock", ".yarn"},
		ciSetupSupport: true,
	},
	Yarn: {
		PackageType: "npm",
		indicators:  []string{".yarnrc.yml", "yarn.lock", ".yarn"},
	},
	Go: {
		PackageType: "go",
		indicators:  []string{"go.mod"},
	},
	Pip: {
		PackageType: "pypi",
		indicators:  []string{"setup.py", "requirements.txt"},
	},
	Pipenv: {
		PackageType: "pypi",
		indicators:  []string{"pipfile", "pipfile.lock"},
	},
	Nuget: {
		PackageType: "nuget",
		indicators:  []string{".sln", ".csproj"},
	},
	Dotnet: {
		PackageType: "nuget",
		indicators:  []string{".sln", ".csproj"},
	},
}

func GetTechnologyPackageType(techName Technology) string {
	techData, ok := technologiesData[techName]
	if ok {
		return techData.PackageType
	} else {
		return string(techName)
	}
}

// DetectTechnologies tries to detect all technologies types according to the files in the given path.
// 'isCiSetup' will limit the search of possible techs to Maven, Gradle, and npm.
// 'recursive' will determine if the search will be limited to files in the root path or not.
func DetectTechnologies(path string, isCiSetup, recursive bool) (map[Technology]bool, error) {
	var filesList []string
	var err error
	if recursive {
		filesList, err = fileutils.ListFilesRecursiveWalkIntoDirSymlink(path, false)
	} else {
		filesList, err = fileutils.ListFiles(path, true)
	}
	if err != nil {
		return nil, err
	}
	detectedTechnologies := detectTechnologiesByFilePaths(filesList, isCiSetup)
	return detectedTechnologies, nil
}

func detectTechnologiesByFilePaths(paths []string, isCiSetup bool) (detected map[Technology]bool) {
	detected = make(map[Technology]bool)
	exclude := make(map[Technology]bool)
	for _, path := range paths {
		for techName, techData := range technologiesData {
			// If the detection is in a 'jf ci-setup' command, then the checked technology must be supported.
			if !isCiSetup || (isCiSetup && techData.ciSetupSupport) {
				// If the project contains a file/directory with a name that ends with an excluded suffix, then this technology is excluded.
				for _, excludeFile := range techData.exclude {
					if strings.HasSuffix(path, excludeFile) {
						exclude[techName] = true
					}
				}
				// If this technology was already excluded, there's no need to look for indicator files/directories.
				if _, exist := exclude[techName]; !exist {
					// If the project contains a file/directory with a name that ends with the indicator suffix, then the project probably uses this technology.
					for _, indicator := range techData.indicators {
						if strings.HasSuffix(path, indicator) {
							detected[techName] = true
						}
					}
				}
			}
		}
	}
	// Remove excluded technologies.
	for excludeTech := range exclude {
		delete(detected, excludeTech)
	}
	return detected
}

// DetectTechnologiesToString returns a string that includes all the names of the detected technologies separated by a comma.
func DetectedTechnologiesToString(detected map[Technology]bool) string {
	keys := DetectedTechnologiesToSlice(detected)
	if len(keys) > 0 {
		detectedTechnologiesString := strings.Join(keys, ", ")
		detectedTechnologiesString += "."
		return detectedTechnologiesString
	}
	return ""
}

// DetectedTechnologiesToSlice returns a string slice that includes all the names of the detected technologies.
func DetectedTechnologiesToSlice(detected map[Technology]bool) []string {
	keys := make([]string, len(detected))
	i := 0
	for tech := range detected {
		keys[i] = string(tech)
		i++
	}
	return keys
}

func ToTechnologies(args []string) (technologies []Technology) {
	for _, argument := range args {
		technologies = append(technologies, Technology(argument))
	}
	return
}
