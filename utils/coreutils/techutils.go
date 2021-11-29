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
	return strings.Contains(file, "package.json")
}

type GoIndicator struct {
}

func (gi GoIndicator) GetTechnology() Technology {
	return Go
}

func (gi GoIndicator) Indicates(file string) bool {
	return strings.Contains(file, "go.mod")
}

func GetTechIndicators() []TechnologyIndicator {
	return []TechnologyIndicator{
		MavenIndicator{},
		GradleIndicator{},
		NpmIndicator{},
		GoIndicator{},
	}
}

func DetectTechnologies(path string) (map[Technology]bool, error) {
	indicators := GetTechIndicators()
	filesList, err := fileutils.ListFilesRecursiveWalkIntoDirSymlink(path, false)
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
