package npmutils

import (
	"fmt"
	buildinfo "github.com/jfrog/build-info-go/entities"
	"strings"

	"github.com/buger/jsonparser"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type TypeRestriction int

const (
	DefaultRestriction TypeRestriction = iota
	All
	DevOnly
	ProdOnly
)

type Dependency struct {
	Name       string
	Version    string
	Scopes     []string
	FileType   string
	Checksum   *buildinfo.Checksum
	PathToRoot [][]string
}

func (dep *Dependency) GetPathToRoot() [][]string {
	return dep.PathToRoot
}

func CalculateDependenciesList(typeRestriction TypeRestriction, npmArgs []string, executablePath, buildInfoModuleId string) (dependenciesList map[string]*Dependency, err error) {
	dependenciesList = make(map[string]*Dependency)
	if typeRestriction != ProdOnly {
		if prepareDependencies("dev", executablePath, buildInfoModuleId, npmArgs, &dependenciesList); err != nil {
			return
		}
	}
	if typeRestriction != DevOnly {
		err = prepareDependencies("prod", executablePath, buildInfoModuleId, npmArgs, &dependenciesList)
	}
	return
}

// Run npm list and parse the returned JSON.
// typeRestriction must be one of: 'dev' or 'prod'!
func prepareDependencies(typeRestriction, executablePath, buildInfoModuleId string, npmArgs []string, results *map[string]*Dependency) error {
	// Run npm list
	// Although this command can get --development as a flag (according to npm docs), it's not working on npm 6.
	// Although this command can get --only=development as a flag (according to npm docs), it's not working on npm 7.
	data, errData, err := RunList(strings.Join(append(npmArgs, "--all", "--"+typeRestriction), " "), executablePath)
	if err != nil {
		log.Warn("npm list command failed with error:", err.Error())
	}
	if len(errData) > 0 {
		log.Warn("Some errors occurred while collecting dependencies info:\n" + string(errData))
	}

	// Parse the dependencies json object
	return jsonparser.ObjectEach(data, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) (err error) {
		if string(key) == "dependencies" {
			err = parseDependencies(value, typeRestriction, []string{buildInfoModuleId}, results)
		}
		return err
	})
}

// Parses npm dependencies recursively and adds the collected dependencies to the given dependencies map.
func parseDependencies(data []byte, scope string, pathToRoot []string, dependencies *map[string]*Dependency) error {
	return jsonparser.ObjectEach(data, func(key []byte, value []byte, dataType jsonparser.ValueType, offset int) error {
		depName := string(key)
		ver, _, _, err := jsonparser.Get(data, depName, "version")
		depVersion := string(ver)
		depKey := depName + ":" + depVersion
		if err != nil && err != jsonparser.KeyPathNotFoundError {
			return errorutils.CheckError(err)
		} else if err == jsonparser.KeyPathNotFoundError {
			log.Debug(fmt.Sprintf("%s dependency will not be included in the build-info, because the 'npm ls' command did not return its version.\nThe reason why the version wasn't returned may be because the package is a 'peerdependency', which was not manually installed.\n'npm install' does not download 'peerdependencies' automatically. It is therefore okay to skip this dependency.", depName))
		} else {
			appendDependency(dependencies, depKey, depName, depVersion, scope, pathToRoot)
		}
		transitive, _, _, err := jsonparser.Get(data, depName, "dependencies")
		if err != nil && err.Error() != "Key path not found" {
			return errorutils.CheckError(err)
		}
		if len(transitive) > 0 {
			if err := parseDependencies(transitive, scope, append([]string{depKey}, pathToRoot...), dependencies); err != nil {
				return err
			}
		}
		return nil
	})
}

func appendDependency(dependencies *map[string]*Dependency, depKey, depName, depVersion, scope string, pathToRoot []string) {
	if (*dependencies)[depKey] == nil {
		(*dependencies)[depKey] = &Dependency{Name: depName, Version: depVersion, Scopes: []string{scope}}
	} else if !scopeAlreadyExists(scope, (*dependencies)[depKey].Scopes) {
		(*dependencies)[depKey].Scopes = append((*dependencies)[depKey].Scopes, scope)
	}
	(*dependencies)[depKey].PathToRoot = append((*dependencies)[depKey].PathToRoot, pathToRoot)
}

func scopeAlreadyExists(scope string, existingScopes []string) bool {
	for _, existingScope := range existingScopes {
		if existingScope == scope {
			return true
		}
	}
	return false
}
