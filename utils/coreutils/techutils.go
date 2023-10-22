package coreutils

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jfrog/jfrog-client-go/artifactory/services/fspatterns"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"

	"golang.org/x/exp/maps"
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
	// The files that handle the project's dependencies.
	packageDescriptors []string
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
		packageDescriptors:     []string{"pom.xml"},
		execCommand:            "mvn",
		applicabilityScannable: true,
	},
	Gradle: {
		indicators:             []string{".gradle", ".gradle.kts"},
		ciSetupSupport:         true,
		packageDescriptors:     []string{"build.gradle", "build.gradle.kts"},
		applicabilityScannable: true,
	},
	Npm: {
		indicators:                 []string{"package.json", "package-lock.json", "npm-shrinkwrap.json"},
		exclude:                    []string{".yarnrc.yml", "yarn.lock", ".yarn"},
		ciSetupSupport:             true,
		packageDescriptors:         []string{"package.json"},
		formal:                     string(Npm),
		packageVersionOperator:     "@",
		packageInstallationCommand: "install",
		applicabilityScannable:     true,
	},
	Yarn: {
		indicators:             []string{".yarnrc.yml", "yarn.lock", ".yarn"},
		packageDescriptors:     []string{"package.json"},
		packageVersionOperator: "@",
		applicabilityScannable: true,
	},
	Go: {
		indicators:                 []string{"go.mod"},
		packageDescriptors:         []string{"go.mod"},
		packageVersionOperator:     "@v",
		packageInstallationCommand: "get",
	},
	Pip: {
		packageType:            Pypi,
		indicators:             []string{"setup.py", "requirements.txt"},
		packageDescriptors:     []string{"setup.py", "requirements.txt"},
		exclude:                []string{"Pipfile", "Pipfile.lock", "pyproject.toml", "poetry.lock"},
		applicabilityScannable: true,
	},
	Pipenv: {
		packageType:                Pypi,
		indicators:                 []string{"Pipfile", "Pipfile.lock"},
		packageDescriptors:         []string{"Pipfile"},
		packageVersionOperator:     "==",
		packageInstallationCommand: "install",
		applicabilityScannable:     true,
	},
	Poetry: {
		packageType:                Pypi,
		indicators:                 []string{"pyproject.toml", "poetry.lock"},
		packageDescriptors:         []string{"pyproject.toml"},
		packageInstallationCommand: "add",
		packageVersionOperator:     "==",
		applicabilityScannable:     true,
	},
	Nuget: {
		indicators: []string{".sln", ".csproj"},
		formal:     "NuGet",
		// .NET CLI is used for NuGet projects
		execCommand:                "dotnet",
		packageInstallationCommand: "add",
		// packageName -v packageVersion
		packageVersionOperator: " -v ",
	},
	Dotnet: {
		indicators: []string{".sln", ".csproj"},
		formal:     ".NET",
	},
}

func (tech Technology) ToFormal() string {
	if technologiesData[tech].formal == "" {
		return cases.Title(language.Und).String(tech.String())
	}
	return technologiesData[tech].formal
}

func (tech Technology) String() string {
	return string(tech)
}

func (tech Technology) GetExecCommandName() string {
	if technologiesData[tech].execCommand == "" {
		return tech.String()
	}
	return technologiesData[tech].execCommand
}

func (tech Technology) GetPackageType() string {
	if technologiesData[tech].packageType == "" {
		return tech.String()
	}
	return technologiesData[tech].packageType
}

func (tech Technology) GetPackageDescriptor() []string {
	return technologiesData[tech].packageDescriptors
}

func (tech Technology) IsCiSetup() bool {
	return technologiesData[tech].ciSetupSupport
}

func (tech Technology) GetPackageVersionOperator() string {
	return technologiesData[tech].packageVersionOperator
}

func (tech Technology) GetPackageInstallationCommand() string {
	return technologiesData[tech].packageInstallationCommand
}

func (tech Technology) ApplicabilityScannable() bool {
	return technologiesData[tech].applicabilityScannable
}

func DetectedTechnologiesList() (technologies []string) {
	wd, err := os.Getwd()
	if errorutils.CheckError(err) != nil {
		return
	}
	return DetectedTechnologiesListInPath(wd, false)
}

func DetectedTechnologiesListInPath(path string, recursive bool) (technologies []string) {
	detectedTechnologies, err := DetectTechnologies(path, false, recursive)
	if err != nil {
		return
	}
	if len(detectedTechnologies) == 0 {
		return
	}
	techStringsList := DetectedTechnologiesToSlice(detectedTechnologies)
	log.Info(fmt.Sprintf("Detected: %s.", strings.Join(techStringsList, ", ")))
	return techStringsList
}

// If recursive is true, the search will not be limited to files in the root path.
// If requestedTechs is empty, all technologies will be checked.
// If excludePathPattern is not empty, files/directories that match the wildcard pattern will be excluded from the search.
func DetectTechnologiesDescriptors(path string, recursive bool, requestedTechs []string, requestedDescriptors map[Technology][]string, excludePathPattern string) (technologiesDetected map[Technology]map[string][]string) {
	filesList, err := fspatterns.ListFiles(path, recursive, false, true, excludePathPattern)
	if err != nil {
		return
	}
	workingDirectoryToIndicators, excludedTechAtWorkingDir := mapFilesToRelevantWorkingDirectories(filesList, requestedDescriptors)
	technologiesDetected = mapWorkingDirectoriesToTechnologies(workingDirectoryToIndicators, excludedTechAtWorkingDir, ToTechnologies(requestedTechs))
	log.Debug(fmt.Sprintf("Detected %d technologies at %s: %s.", len(technologiesDetected), path, maps.Keys(technologiesDetected)))
	return
}

func mapFilesToRelevantWorkingDirectories(files []string, requestedDescriptors map[Technology][]string) (workingDirectoryToIndicators map[string][]string, excludedTechAtWorkingDir map[string][]Technology) {
	workingDirectoryToIndicators = make(map[string][]string)
	excludedTechAtWorkingDir = make(map[string][]Technology)
	for _, path := range files {
		directory := filepath.Dir(path)
		for tech, techData := range technologiesData {
			// Check if the working directory contains indicators/descriptors for the technology
			if isDescriptor(path, techData) || isRequestedDescriptor(path, requestedDescriptors[tech]) {
				workingDirectoryToIndicators[directory] = append(workingDirectoryToIndicators[directory], path)
			} else if isIndicator(path, techData) {
				workingDirectoryToIndicators[directory] = append(workingDirectoryToIndicators[directory], path)
			}
			// Check if the working directory contains a file/directory with a name that ends with an excluded suffix
			if isExclude(path, techData) {
				excludedTechAtWorkingDir[directory] = append(excludedTechAtWorkingDir[directory], tech)
			}
		}
	}
	strJson, _ := json.MarshalIndent(workingDirectoryToIndicators, "", "  ")
	log.Debug(fmt.Sprintf("mapped %d working directories with indicators/descriptors:\n%s", len(workingDirectoryToIndicators), strJson))
	return
}

func isDescriptor(path string, techData TechData) bool {
	for _, descriptor := range techData.packageDescriptors {
		if strings.HasSuffix(path, descriptor) {
			return true
		}
	}
	return false
}

func isRequestedDescriptor(path string, requestedDescriptors []string) bool {
	for _, requestedDescriptor := range requestedDescriptors {
		if strings.HasSuffix(path, requestedDescriptor) {
			return true
		}
	}
	return false
}

func isIndicator(path string, techData TechData) bool {
	for _, indicator := range techData.indicators {
		if strings.HasSuffix(path, indicator) {
			return true
		}
	}
	return false
}

func isExclude(path string, techData TechData) bool {
	for _, exclude := range techData.exclude {
		if strings.HasSuffix(path, exclude) {
			return true
		}
	}
	return false
}

func mapWorkingDirectoriesToTechnologies(workingDirectoryToIndicators map[string][]string, excludedTechAtWorkingDir map[string][]Technology, requestedTechs []Technology) (technologiesDetected map[Technology]map[string][]string) {
	// Get the relevant technologies to check
	technologies := requestedTechs
	if len(technologies) == 0 {
		technologies = GetAllTechnologiesList()
	}
	technologiesDetected = make(map[Technology]map[string][]string)
	// Map working directories to technologies
	for _, tech := range technologies {
		techWorkingDirs := make(map[string][]string)
		foundIndicator := false
		for wd, indicators := range workingDirectoryToIndicators {
			if excludedTechs, exist := excludedTechAtWorkingDir[wd]; exist {
				for _, excludedTech := range excludedTechs {
					if excludedTech == tech {
						// Exclude this technology from this working directory
						continue
					}
				}
			}
			// Check if the working directory contains indicators/descriptors for the technology
			for _, path := range indicators {
				if isDescriptor(path, technologiesData[tech]) {
					techWorkingDirs[wd] = append(techWorkingDirs[wd], path)
				} else if isIndicator(path, technologiesData[tech]) {
					foundIndicator = true
				}
			}
		}
		// Don't allow working directory if sub directory already exists as key for the same technology
		techWorkingDirs = cleanSubDirectories(techWorkingDirs)
		if foundIndicator || len(techWorkingDirs) > 0 {
			// Found indicators/descriptors for technology, add to detected.
			technologiesDetected[tech] = techWorkingDirs
		}
	}

	for _, tech := range requestedTechs {
		if _, exist := technologiesDetected[tech]; !exist {
			// Requested (forced with flag) technology and not found any indicators/descriptors in detection, add as detected.
			log.Warn(fmt.Sprintf("Requested technology %s but not found any indicators/descriptors in detection.", tech))
			technologiesDetected[tech] = map[string][]string{}
		}
	}
	return
}

func cleanSubDirectories(workingDirectoryToFiles map[string][]string) (result map[string][]string) {
	result = make(map[string][]string)
	for wd, files := range workingDirectoryToFiles {
		root := getExistingRootDir(wd, workingDirectoryToFiles)
		result[root] = append(result[root], files...)
		// if root == wd {
		// 	// Current working directory is the root
		// 	result[wd] = files
		// } else {
		// 	// add descriptors from sub projects to the root
		// 	result[root] = append(result[root], files...)
		// }
	}
	return
}

func getExistingRootDir(path string, workingDirectoryToIndicators map[string][]string) (rootDir string) {
	rootDir = path
	for wd := range workingDirectoryToIndicators {
		if strings.HasPrefix(rootDir, wd) {
			rootDir = wd
		}
	}
	return

	// // TODO: make sure to get the top most root!
	// for wd := range workingDirectoryToIndicators {
	// 	if path != wd && strings.HasPrefix(path, wd) {
	// 		return wd
	// 	}
	// }
	// return ""
}

// func detectTechnologiesDescriptorsByFilePaths(paths []string) (technologiesToDescriptors map[Technology][]string) {
// 	detected := make(map[Technology][]string)
// 	for _, path := range paths {
// 		for techName, techData := range technologiesData {
// 			for _, descriptor := range techData.packageDescriptors {
// 				if strings.HasSuffix(path, descriptor) {
// 					detected[techName] = append(detected[techName], path)
// 				}
// 			}
// 		}
// 	}
// }

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
	log.Info(fmt.Sprintf("Scanning %d file(s):%s", len(filesList), filesList))
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

// func t(path string, techData TechData) (exclude bool, detected bool) {
// 	// If the project contains a file/directory with a name that ends with an excluded suffix, then this technology is excluded.
// 	for _, excludeFile := range techData.exclude {
// 		if strings.HasSuffix(path, excludeFile) {
// 			return true, false
// 		}
// 	}
// 	// If this technology was already excluded, there's no need to look for indicator files/directories.
// 	if _, exist := exclude[techName]; !exist {
// 		// If the project contains a file/directory with a name that ends with the indicator suffix, then the project probably uses this technology.
// 		for _, indicator := range techData.indicators {
// 			if strings.HasSuffix(path, indicator) {
// 				return false, true
// 			}
// 		}
// 	}
// }

// DetectedTechnologiesToSlice returns a string slice that includes all the names of the detected technologies.
func DetectedTechnologiesToSlice(detected map[Technology]bool) []string {
	keys := make([]string, 0, len(detected))
	for tech := range detected {
		keys = append(keys, string(tech))
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
