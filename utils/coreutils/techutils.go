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
	Pypi   = "pypi"
)

type TechnologyIndicator interface {
	GetTechnology() Technology
	Indicates(file string) bool
}

type MavenIndicator struct {
}

func (mi MavenIndicator) GetTechnology() Technology {
	return Maven
}

func (mi MavenIndicator) Indicates(file string) bool {
	return strings.Contains(file, "pom.xml")
}

type GradleIndicator struct {
}

func (gi GradleIndicator) GetTechnology() Technology {
	return Gradle
}

func (gi GradleIndicator) Indicates(file string) bool {
	return strings.Contains(file, ".gradle")
}

type NpmIndicator struct {
}

func (ni NpmIndicator) GetTechnology() Technology {
	return Npm
}

func (ni NpmIndicator) Indicates(file string) bool {
	return strings.Contains(file, "package.json") || strings.Contains(file, "package-lock.json") || strings.Contains(file, "npm-shrinkwrap.json")
}

type GoIndicator struct {
}

func (gi GoIndicator) GetTechnology() Technology {
	return Go
}

func (gi GoIndicator) Indicates(file string) bool {
	return strings.Contains(file, "go.mod")
}

type PypiIndicator struct {
}

func (pi PypiIndicator) GetTechnology() Technology {
	return Pypi
}

func (pi PypiIndicator) Indicates(file string) bool {
	return strings.Contains(file, "setup.py") || strings.Contains(file, "requirements.txt")
}

func GetMinSupportedTechIndicators() []TechnologyIndicator {
	return []TechnologyIndicator{
		MavenIndicator{},
		GradleIndicator{},
		NpmIndicator{},
	}
}

func GetTechIndicators() []TechnologyIndicator {
	return append(GetMinSupportedTechIndicators(), GoIndicator{}, PypiIndicator{})
}

func DetectTechnologies(path string, limitTechnologiesSupport, recursive bool) (map[Technology]bool, error) {
	var indicators []TechnologyIndicator
	if limitTechnologiesSupport {
		indicators = GetMinSupportedTechIndicators()
	} else {
		indicators = GetTechIndicators()
	}
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
		for _, indicator := range indicators {
			if indicator.Indicates(file) {
				detectedTechnologies[indicator.GetTechnology()] = true
				// Same file can't indicate more than one technology.
				break
			}
		}
	}
	return detectedTechnologies, nil
}
