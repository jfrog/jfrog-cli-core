package coreutils

import (
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"strings"
)

type Technology string

const (
	Maven  = "Maven"
	Gradle = "Gradle"
	Npm    = "npm"
	Go     = "go"
	Pip    = "pip"
	Pipenv = "pipenv"
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
}

func GetTechnologyPackageType(techName Technology) string {
	techData, ok := technologiesData[techName]
	if ok {
		return techData.PackageType
	} else {
		return ""
	}
}

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
		techName := detectTechnologyByFile(strings.ToLower(file), isCiSetup)
		if techName != "" {
			detectedTechnologies[techName] = true
		}
	}
	return detectedTechnologies, nil
}

func detectTechnologyByFile(file string, isCiSetup bool) Technology {
	for techName, techData := range technologiesData {
		if isCiSetup == false || (isCiSetup && techData.ciSetupSupport) {
			for _, indicator := range techData.indicators {
				if strings.Contains(file, indicator) {
					return techName
				}
			}
		}
	}
	return ""
}
