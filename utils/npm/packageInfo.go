package npmutils

import (
	"encoding/json"
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/version"
	"io/ioutil"
	"path/filepath"
	"strings"

	"github.com/jfrog/jfrog-client-go/utils/errorutils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type PackageInfo struct {
	Name    string `json:"name,omitempty"`
	Version string `json:"version,omitempty"`
	Scope   string
}

func ReadPackageInfoFromPackageJson(packageJsonDirectory string, npmVersion *version.Version) (*PackageInfo, error) {
	log.Debug("Reading info from package.json file:", filepath.Join(packageJsonDirectory, "package.json"))
	packageJson, err := ioutil.ReadFile(filepath.Join(packageJsonDirectory, "package.json"))
	if err != nil {
		return nil, errorutils.CheckError(err)
	}
	return ReadPackageInfo(packageJson, npmVersion)
}

func ReadPackageInfo(data []byte, npmVersion *version.Version) (*PackageInfo, error) {
	parsedResult := new(PackageInfo)
	if err := json.Unmarshal(data, parsedResult); err != nil {
		return nil, errorutils.CheckError(err)
	}
	// If npm older than v7, remove prefixes.
	if npmVersion == nil || npmVersion.Compare("7.0.0") > 0 {
		removeVersionPrefixes(parsedResult)
	}
	splitScopeFromName(parsedResult)
	return parsedResult, nil
}

func (pi *PackageInfo) BuildInfoModuleId() string {
	nameBase := fmt.Sprintf("%s:%s", pi.Name, pi.Version)
	if pi.Scope == "" {
		return nameBase
	}
	return fmt.Sprintf("%s:%s", strings.TrimPrefix(pi.Scope, "@"), nameBase)
}

func (pi *PackageInfo) GetDeployPath() string {
	fileName := fmt.Sprintf("%s-%s.tgz", pi.Name, pi.Version)
	if pi.Scope == "" {
		return fmt.Sprintf("%s/-/%s", pi.Name, fileName)
	}
	return fmt.Sprintf("%s/%s/-/%s", pi.Scope, pi.Name, fileName)
}

func (pi *PackageInfo) FullName() string {
	if pi.Scope == "" {
		return pi.Name
	}
	return fmt.Sprintf("%s/%s", pi.Scope, pi.Name)
}

func splitScopeFromName(packageInfo *PackageInfo) {
	if strings.HasPrefix(packageInfo.Name, "@") && strings.Contains(packageInfo.Name, "/") {
		splitValues := strings.Split(packageInfo.Name, "/")
		packageInfo.Scope = splitValues[0]
		packageInfo.Name = splitValues[1]
	}
}

// A leading "=" or "v" character is stripped off and ignored by npm.
func removeVersionPrefixes(packageInfo *PackageInfo) {
	packageInfo.Version = strings.TrimPrefix(packageInfo.Version, "v")
	packageInfo.Version = strings.TrimPrefix(packageInfo.Version, "=")
}
