package python

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"os"
	"os/exec"
	"strings"
)

func runPythonCommand(execPath string, cmdArgs []string, envs string) (data []byte, err error) {
	cmd := exec.Command(execPath, cmdArgs...)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, envs)
	log.Debug(fmt.Sprintf("running command: %v", cmd.Args))
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = errorutils.CheckError(cmd.Run())
	if err != nil {
		return nil, errors.New(fmt.Sprintf("Failed running command: '%s %s' with error: %s - %s", execPath, strings.Join(cmdArgs, " "), err.Error(), stderr.String()))
	}
	if err != nil {
		return nil, err
	}
	return stdout.Bytes(), err
}

// Parse pip-dependency-map raw output to dependencies map (mapping dependency to his child deps) and top level deps list
func parseDependenciesToGraph(packages []pythonDependencyPackage) (map[string][]string, []string, error) {
	// Create packages map.
	packagesMap := map[string][]string{}
	allSubPackages := map[string]bool{}
	for _, pkg := range packages {
		var subPackages []string
		for _, subPkg := range pkg.Dependencies {
			subPkgFullName := subPkg.Key + ":" + subPkg.InstalledVersion
			subPackages = append(subPackages, subPkgFullName)
			allSubPackages[subPkgFullName] = true
		}
		packagesMap[pkg.Package.Key+":"+pkg.Package.InstalledVersion] = subPackages
	}

	var topLevelPackagesList []string
	for pkgName := range packagesMap {
		if allSubPackages[pkgName] == false {
			topLevelPackagesList = append(topLevelPackagesList, pkgName)
		}
	}
	return packagesMap, topLevelPackagesList, nil
}

// Structs for parsing the pip-dependency-map result.
type pythonDependencyPackage struct {
	Package      packageType  `json:"package,omitempty"`
	Dependencies []dependency `json:"dependencies,omitempty"`
}

type packageType struct {
	Key              string `json:"key,omitempty"`
	PackageName      string `json:"package_name,omitempty"`
	InstalledVersion string `json:"installed_version,omitempty"`
}

type dependency struct {
	Key              string `json:"key,omitempty"`
	PackageName      string `json:"package_name,omitempty"`
	InstalledVersion string `json:"installed_version,omitempty"`
}
