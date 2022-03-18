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
	Go     = "go"
	Pip    = "pip"
	Pipenv = "pipenv"
	Nuget  = "nuget"
	Dotnet = "dotnet"
	Docker = "docker"
)

type TechData struct {
	PackageType    string
	indicators     []string
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
		ciSetupSupport: true,
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
	detectedTechnologies := make(map[Technology]bool)
	for _, file := range filesList {
		techNames := detectTechnologiesByFile(strings.ToLower(file), isCiSetup)
		for _, techName := range techNames {
			detectedTechnologies[techName] = true
		}
	}
	return detectedTechnologies, nil
}

func detectTechnologiesByFile(file string, isCiSetup bool) (detected []Technology) {
	detected = []Technology{}
	for techName, techData := range technologiesData {
		if !isCiSetup || (isCiSetup && techData.ciSetupSupport) {
			for _, indicator := range techData.indicators {
				if strings.Contains(file, indicator) {
					detected = append(detected, techName)
				}
			}
		}
	}
	return detected
}

// DetectTechnologiesToString returns a string that includes all the names of the detected technologies separated by a comma.
func DetectedTechnologiesToString(detected map[Technology]bool) string {
	detectedTechnologiesString := ""
	for tech := range detected {
		detectedTechnologiesString += string(tech) + ", "
	}
	if detectedTechnologiesString != "" {
		detectedTechnologiesString = strings.Trim(detectedTechnologiesString, ", ")
		detectedTechnologiesString += "."
	}
	return detectedTechnologiesString
}
