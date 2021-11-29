package techindicators

import (
	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
	"strings"
)

const (
	// supported technologies:
	Maven  = "Maven"
	Gradle = "Gradle"
	Go     = "Go"
	Npm    = "npm"
	Pip    = "pip"
	Pipenv = "pipenv"
)

type Technology string

var technologies = map[Technology]TechnologyData{
	Maven: {
		ExecName:   "mvn",
		Indicators: []string{"pom.xml"},
	},
	Gradle: {
		ExecName:   "gradle",
		Indicators: []string{".gradle"},
	},
	Npm: {
		ExecName:   "npm",
		Indicators: []string{"package.json", "package-lock.json", "npm-shrinkwrap.json"},
	},
	Go: {
		ExecName:   "go",
		Indicators: []string{"go.mod", "go.sum"},
	},
	Pip: {
		ExecName:   "pip",
		Indicators: []string{"setup.py", "requirements.txt"},
	},
	Pipenv: {
		Indicators: []string{"pipfile"},
	},
}

func GetExecName(tech Technology) string {
	return technologies[tech].ExecName
}

func DetectTechnologies(dirPath string, recursive bool) (map[Technology]bool, error) {
	var filesList []string
	var err error
	if recursive == true {
		filesList, err = fileutils.ListFilesRecursiveWalkIntoDirSymlink(dirPath, false)
	} else {
		filesList, err = fileutils.ListFiles(dirPath, true)
	}
	if err != nil {
		return nil, err
	}
	detectedTechnologies := make(map[Technology]bool)
	for _, filePath := range filesList {
		tech := indicateTechByFile(filePath)
		if tech != "" && detectedTechnologies[tech] == false {
			detectedTechnologies[tech] = true
		}
	}
	return detectedTechnologies, nil
}

func indicateTechByFile(filePath string) Technology {
	for tech, techData := range technologies {
		for _, indicator := range techData.Indicators {
			if strings.Contains(filePath, indicator) {
				return tech
			}
		}
	}
	return ""
}

type TechnologyData struct {
	ExecName   string
	Indicators []string
}
