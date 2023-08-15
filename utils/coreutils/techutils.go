package coreutils

import (
	"strings"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/jfrog/jfrog-client-go/utils/io/fileutils"
)

type Technology string

const (
	Maven  Technology = "maven"
	Gradle Technology = "gradle"
	Npm    Technology = "npm"
	Yarn   Technology = "yarn"
	Go     Technology = "go"
	Pip    Technology = "pip"
	Pipenv Technology = "pipenv"
	Poetry Technology = "poetry"
	Nuget  Technology = "nuget"
	Dotnet Technology = "dotnet"
	Docker Technology = "docker"
)

const Pypi = "pypi"

type TechData struct {
	// The name of the package type used in this technology.
	packageType string
	// Suffixes of file/directory names that indicate if a project uses this technology.
	// The name of at least one of the files/directories in the project's directory must end with one of these suffixes.
	indicators []string
	// Suffixes of file/directory names that indicate if a project does not use this technology.
	// The names of all the files/directories in the project's directory must NOT end with any of these suffixes.
	exclude []string
	// Whether this technology is supported by the 'jf ci-setup' command.
	ciSetupSupport bool
	// Whether Contextual Analysis supported in this technology.
	applicabilityScannable bool
	// The file that handles the project's dependencies.
	packageDescriptor string
	// Formal name of the technology
	formal string
	// The executable name of the technology
	execCommand string
	// The operator for package versioning
	packageVersionOperator string
	// The package installation command of a package
	packageInstallationCommand string
}

var technologiesData = map[Technology]TechData{
	Maven: {
		indicators:             []string{"pom.xml"},
		ciSetupSupport:         true,
		packageDescriptor:      "pom.xml",
		execCommand:            "mvn",
		applicabilityScannable: true,
	},
	Gradle: {
		indicators:             []string{".gradle", ".gradle.kts"},
		ciSetupSupport:         true,
		packageDescriptor:      "build.gradle, build.gradle.kts",
		applicabilityScannable: true,
	},
	Npm: {
		indicators:                 []string{"package.json", "package-lock.json", "npm-shrinkwrap.json"},
		exclude:                    []string{".yarnrc.yml", "yarn.lock", ".yarn"},
		ciSetupSupport:             true,
		packageDescriptor:          "package.json",
		formal:                     string(Npm),
		packageVersionOperator:     "@",
		packageInstallationCommand: "install",
		applicabilityScannable:     true,
	},
	Yarn: {
		indicators:             []string{".yarnrc.yml", "yarn.lock", ".yarn"},
		packageDescriptor:      "package.json",
		packageVersionOperator: "@",
		applicabilityScannable: true,
	},
	Go: {
		indicators:                 []string{"go.mod"},
		packageDescriptor:          "go.mod",
		packageVersionOperator:     "@v",
		packageInstallationCommand: "get",
	},
	Pip: {
		packageType:            Pypi,
		indicators:             []string{"setup.py", "requirements.txt"},
		exclude:                []string{"Pipfile", "Pipfile.lock", "pyproject.toml", "poetry.lock"},
		applicabilityScannable: true,
	},
	Pipenv: {
		packageType:                Pypi,
		indicators:                 []string{"Pipfile", "Pipfile.lock"},
		packageDescriptor:          "Pipfile",
		packageVersionOperator:     "==",
		packageInstallationCommand: "install",
		applicabilityScannable:     true,
	},
	Poetry: {
		packageType:                Pypi,
		indicators:                 []string{"pyproject.toml", "poetry.lock"},
		packageInstallationCommand: "add",
		packageVersionOperator:     "==",
		applicabilityScannable:     true,
	},
	Nuget: {
		indicators: []string{".sln", ".csproj"},
		formal:     "NuGet",
	},
	Dotnet: {
		indicators: []string{".sln", ".csproj"},
		formal:     ".NET",
	},
}

func (tech Technology) ToFormal() string {
	if technologiesData[tech].formal == "" {
		return cases.Title(language.Und).String(tech.ToString())
	}
	return technologiesData[tech].formal
}

func (tech Technology) ToString() string {
	return string(tech)
}

func (tech Technology) GetExecCommandName() string {
	if technologiesData[tech].execCommand == "" {
		return tech.ToString()
	}
	return technologiesData[tech].execCommand
}

func (tech Technology) GetPackageType() string {
	if technologiesData[tech].packageType == "" {
		return tech.ToString()
	}
	return technologiesData[tech].packageType
}

func (tech Technology) GetPackageDescriptor() string {
	if technologiesData[tech].packageDescriptor == "" {
		return tech.ToFormal() + " Package Descriptor"
	}
	return technologiesData[tech].packageDescriptor
}

func (tech Technology) IsCiSetup() bool {
	return technologiesData[tech].ciSetupSupport
}

func (tech Technology) GetPackageOperator() string {
	return technologiesData[tech].packageVersionOperator
}

func (tech Technology) GetPackageInstallationCommand() string {
	return technologiesData[tech].packageInstallationCommand
}

func (tech Technology) ApplicabilityScannable() bool {
	return technologiesData[tech].applicabilityScannable
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

func GetAllTechnologiesList() (technologies []Technology) {
	for tech := range technologiesData {
		technologies = append(technologies, tech)
	}
	return
}

func ContainsApplicabilityScannableTech(technologies []Technology) bool {
	for _, technology := range technologies {
		if technology.ApplicabilityScannable() {
			return true
		}
	}
	return false
}
