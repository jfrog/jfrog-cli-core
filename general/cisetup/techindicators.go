package cisetup

import "strings"

type Technology string

const (
	Maven  = "maven"
	Gradle = "gradle"
	Npm    = "npm"
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

func GetTechIndicators() []TechnologyIndicator {
	return []TechnologyIndicator{
		MavenIndicator{},
		GradleIndicator{},
		NpmIndicator{},
	}
}
