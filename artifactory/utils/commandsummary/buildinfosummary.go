package commandsummary

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	buildInfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	basicSummaryUpgradeNotice         = "<a href=\"%s\">🐸 Enable the linkage to Artifactory</a>\n\n"
	modulesTitle                      = "📦 Modules published to Artifactory by this workflow"
	minTableColumnLength              = 400
	markdownSpaceFiller               = "&nbsp;"
	NonScannedResult                  = "non-scanned"
	ManifestJsonFile                  = "manifest.json"
	AttestationsModuleIdPrefix string = "attestations"
)

var (
	// Scanned modules are modules which can be scanned via CLI command
	scannableModuleType = map[buildInfo.ModuleType]bool{
		buildInfo.Docker: true,
	}
	// Supported modules are modules that their build info contains the OriginalDeploymentRepo field.
	supportedModuleTypes = map[buildInfo.ModuleType]bool{
		buildInfo.Maven:     true,
		buildInfo.Npm:       true,
		buildInfo.Go:        true,
		buildInfo.Generic:   true,
		buildInfo.Terraform: true,
		buildInfo.Docker:    true,
		buildInfo.Python:    true,
	}
)

type BuildInfoSummary struct {
	CommandSummary
}

func NewBuildInfoSummary() (*CommandSummary, error) {
	return New(&BuildInfoSummary{}, "build-info")
}

func (bis *BuildInfoSummary) GetSummaryTitle() string {
	return "🛠️️ Published JFrog Build Info"
}

func (bis *BuildInfoSummary) GenerateMarkdownFromFiles(dataFilePaths []string) (finalMarkdown string, err error) {
	// Aggregate all the build info files into a slice
	var builds []*buildInfo.BuildInfo
	for _, filePath := range dataFilePaths {
		var publishBuildInfo buildInfo.BuildInfo
		if err = UnmarshalFromFilePath(filePath, &publishBuildInfo); err != nil {
			return
		}
		builds = append(builds, &publishBuildInfo)
	}
	if len(builds) == 0 {
		return
	}

	buildInfoTableMarkdown, err := bis.buildInfoTable(builds)
	if err != nil {
		return "", err
	}
	publishedModulesMarkdown, err := bis.buildInfoModules(builds)
	if err != nil {
		return "", err
	}
	if publishedModulesMarkdown != "" {
		publishedModulesMarkdown = WrapCollapsableMarkdown(modulesTitle, publishedModulesMarkdown, 2)
	}
	finalMarkdown = buildInfoTableMarkdown + publishedModulesMarkdown

	return WrapCollapsableMarkdown(bis.GetSummaryTitle(), finalMarkdown, 3), nil
}

// Create a table with published builds and possible scan results.
func (bis *BuildInfoSummary) buildInfoTable(builds []*buildInfo.BuildInfo) (string, error) {
	var tableBuilder strings.Builder
	tableBuilder.WriteString(getBuildInfoTableHeader())
	for _, build := range builds {
		if err := appendBuildInfoRow(&tableBuilder, build); err != nil {
			return "", err
		}
	}
	tableBuilder.WriteString("\n\n")
	return tableBuilder.String(), nil
}

// Generates a view for published modules within the build.
// Modules are displayed as tables if they are scannable via CLI command,
// otherwise, they are shown as an artifact tree.
func (bis *BuildInfoSummary) buildInfoModules(builds []*buildInfo.BuildInfo) (string, error) {
	var markdownBuilder strings.Builder
	markdownBuilder.WriteString("\n\n<h3>Published Modules</h3>\n\n")
	var shouldGenerate bool
	for _, build := range builds {
		supportedModules := filterModules(build.Modules...)
		modulesMarkdown, err := bis.generateModulesMarkdown(supportedModules...)
		if err != nil {
			return "", err
		}
		if modulesMarkdown != "" {
			markdownBuilder.WriteString(modulesMarkdown)
			shouldGenerate = true
		}
	}
	if !shouldGenerate {
		return "", nil
	}
	return markdownBuilder.String(), nil
}

func (bis *BuildInfoSummary) generateModulesMarkdown(modules ...buildInfo.Module) (string, error) {
	var modulesMarkdown strings.Builder
	// Modules could include nested modules inside of them
	// Group the modules by their root module ID
	// If a module has no root, it is considered as a root module itself.
	groupedModuleMap := groupModules(modules)
	if len(groupedModuleMap) == 0 {
		return "", nil
	}
	for rootModuleID, subModules := range groupedModuleMap {
		if len(subModules) == 0 {
			continue
		}
		if !scannableModuleType[subModules[0].Type] {
			tree, err := bis.generateModuleArtifactTree(rootModuleID, subModules)
			if err != nil {
				return "", err
			}
			modulesMarkdown.WriteString(tree)
		} else {
			view, err := bis.generateModuleTableView(rootModuleID, subModules)
			if err != nil {
				return "", err
			}
			modulesMarkdown.WriteString(view)
		}
	}
	return modulesMarkdown.String(), nil
}

func (bis *BuildInfoSummary) generateModuleArtifactTree(rootModuleID string, nestedModules []buildInfo.Module) (string, error) {
	if len(nestedModules) == 0 {
		return "", nil
	}
	var markdownBuilder strings.Builder
	isMultiModule := len(nestedModules) > 1

	markdownBuilder.WriteString(generateModuleHeader(rootModuleID))
	if !StaticMarkdownConfig.IsExtendedSummary() {
		markdownBuilder.WriteString(fmt.Sprintf(basicSummaryUpgradeNotice, StaticMarkdownConfig.GetExtendedSummaryLangPage()))
	}
	for _, module := range nestedModules {
		if isMultiModule && rootModuleID == module.Id {
			continue
		}
		tree, err := bis.generateModuleArtifactsTree(&module, isMultiModule)
		if err != nil {
			return "", err
		}
		markdownBuilder.WriteString(fmt.Sprintf("\n\n<pre>%s</pre>\n\n", tree))
	}
	return markdownBuilder.String(), nil
}

func (bis *BuildInfoSummary) generateModuleTableView(rootModuleID string, subModules []buildInfo.Module) (string, error) {
	var markdownBuilder strings.Builder
	markdownBuilder.WriteString(generateModuleHeader(rootModuleID))
	markdownBuilder.WriteString(generateModuleTableHeader())
	isMultiModule := len(subModules) > 1
	nestedModuleMarkdownTree, err := bis.generateTableModuleMarkdown(subModules, rootModuleID, isMultiModule)
	if err != nil {
		return "", err
	}
	scanResult := getScanResults(extractDockerImageTag(subModules))
	markdownBuilder.WriteString(generateTableRow(nestedModuleMarkdownTree, scanResult))
	return markdownBuilder.String(), nil
}

func (bis *BuildInfoSummary) generateTableModuleMarkdown(nestedModules []buildInfo.Module, parentModuleID string, isMultiModule bool) (string, error) {
	var nestedModuleMarkdownTree strings.Builder
	if len(nestedModules) == 0 {
		return "", nil
	}

	if !StaticMarkdownConfig.IsExtendedSummary() {
		nestedModuleMarkdownTree.WriteString("|")
		nestedModuleMarkdownTree.WriteString(fmt.Sprintf(basicSummaryUpgradeNotice, StaticMarkdownConfig.GetExtendedSummaryLangPage()))
		nestedModuleMarkdownTree.WriteString("<pre>")
	} else {
		nestedModuleMarkdownTree.WriteString("|<pre>")
	}

	for _, module := range nestedModules {
		if isMultiModule && parentModuleID == module.Id {
			continue
		}
		tree, err := bis.generateModuleArtifactsTree(&module, isMultiModule)
		if err != nil {
			return "", err
		}
		nestedModuleMarkdownTree.WriteString(tree)
	}
	nestedModuleMarkdownTree.WriteString(appendSpacesToTableColumn(""))
	nestedModuleMarkdownTree.WriteString("</pre>")
	return nestedModuleMarkdownTree.String(), nil
}

func (bis *BuildInfoSummary) generateModuleArtifactsTree(module *buildInfo.Module, shouldCollapseArtifactsTree bool) (string, error) {
	artifactsTree, err := bis.createArtifactsTree(module)
	if err != nil {
		return "", err
	}
	if shouldCollapseArtifactsTree {
		return bis.generateModuleCollapsibleSection(module, artifactsTree), nil
	}
	return artifactsTree, nil
}

func (bis *BuildInfoSummary) generateModuleCollapsibleSection(module *buildInfo.Module, sectionContent string) string {
	switch module.Type {
	case buildInfo.Docker:
		return createCollapsibleSection(createDockerMultiArchTitle(module), sectionContent)
	default:
		return createCollapsibleSection(module.Id, sectionContent)
	}
}

func (bis *BuildInfoSummary) createArtifactsTree(module *buildInfo.Module) (string, error) {
	artifactsTree := utils.NewFileTree()
	for _, artifact := range module.Artifacts {
		var artifactUrlInArtifactory string
		var err error
		if StaticMarkdownConfig.IsExtendedSummary() {
			artifactUrlInArtifactory, err = generateArtifactUrl(artifact, *module)
			if err != nil {
				return "", err
			}
		}
		if artifact.OriginalDeploymentRepo == "" {
			artifact.OriginalDeploymentRepo = " "
		}
		artifactTreePath := path.Join(artifact.OriginalDeploymentRepo, artifact.Path)
		artifactsTree.AddFile(artifactTreePath, artifactUrlInArtifactory)
		if artifactsTree.IsTreeExceedsMax() {
			return "", nil
		}
	}
	return artifactsTree.String(), nil
}

func generateArtifactUrl(artifact buildInfo.Artifact, module buildInfo.Module) (string, error) {
	if strings.TrimSpace(artifact.OriginalDeploymentRepo) == "" {
		return "", nil
	}
	var section summarySection

	if module.Type == buildInfo.Generic {
		section = artifactsSection
	} else {
		section = packagesSection
	}
	return GenerateArtifactUrl(path.Join(artifact.OriginalDeploymentRepo, artifact.Path), section)
}

func groupModules(modules []buildInfo.Module) map[string][]buildInfo.Module {
	parentToModulesMap := make(map[string][]buildInfo.Module, len(modules))
	for _, module := range modules {
		if len(module.Artifacts) == 0 {
			continue
		}
		parentID := module.Parent
		if parentID == "" {
			parentID = module.Id
		}
		parentToModulesMap[parentID] = append(parentToModulesMap[parentID], module)
	}
	return parentToModulesMap
}

func isSupportedModule(module *buildInfo.Module) bool {
	if !supportedModuleTypes[module.Type] {
		return false
	}
	if module.Type == buildInfo.Docker {
		return !strings.HasPrefix(module.Id, AttestationsModuleIdPrefix)
	}
	return true
}

func createDockerMultiArchTitle(module *buildInfo.Module) string {
	if StaticMarkdownConfig.IsExtendedSummary() {
		parentImageName := strings.Split(module.Parent, ":")[0]
		var sha256 string
		for _, artifact := range module.Artifacts {
			if artifact.Name == ManifestJsonFile {
				sha256 = artifact.Sha256
				break
			}
		}
		dockerModuleLink := fmt.Sprintf(artifactoryDockerPackagesUiFormat, strings.TrimSuffix(StaticMarkdownConfig.GetPlatformUrl(), "/"), "%2F%2F"+url.PathEscape(parentImageName), sha256)
		return fmt.Sprintf("%s <a href=%s>(🐸 View)</a>", module.Id, dockerModuleLink)
	}
	return module.Id
}

func createCollapsibleSection(title, content string) string {
	return fmt.Sprintf("<details><summary>%s</summary>\n%s</details>", title, content)
}

func appendSpacesToTableColumn(str string) string {
	const nbspLength = len(markdownSpaceFiller)
	if len(str) < minTableColumnLength {
		padding := minTableColumnLength - len(str)
		if padding > 0 {
			str += strings.Repeat(markdownSpaceFiller, padding/nbspLength)
		}
	}
	return str
}

func appendBuildInfoRow(tableBuilder *strings.Builder, build *buildInfo.BuildInfo) error {
	buildName := build.Name + " " + build.Number
	buildScanResult := getScanResults(buildName)
	if StaticMarkdownConfig.IsExtendedSummary() {
		buildInfoUrl, err := addGitHubTrackingToUrl(build.BuildUrl, buildInfoSection)
		if err != nil {
			return err
		}
		tableBuilder.WriteString(fmt.Sprintf("| [%s](%s) %s | %s | %s | \n", buildName, buildInfoUrl, appendSpacesToTableColumn(""), appendSpacesToTableColumn(buildScanResult.GetViolations()), appendSpacesToTableColumn(buildScanResult.GetVulnerabilities())))
	} else {
		upgradeMessage := fmt.Sprintf(basicSummaryUpgradeNotice, StaticMarkdownConfig.GetExtendedSummaryLangPage())
		buildName = fmt.Sprintf(" %s %s", upgradeMessage, buildName)
		tableBuilder.WriteString(fmt.Sprintf("| %s %s | %s | %s |\n", fitInsideMarkdownTable(buildName), appendSpacesToTableColumn(""), appendSpacesToTableColumn(buildScanResult.GetViolations()), appendSpacesToTableColumn(buildScanResult.GetVulnerabilities())))
	}
	return nil
}

func getBuildInfoTableHeader() string {
	return "\n\n|  Build Info |  Security Violations | Security Issues |\n|:---------|:------------|:------------|\n"
}

func generateModuleHeader(parentModuleID string) string {
	return fmt.Sprintf("\n\n**%s**\n\n", parentModuleID)
}

func generateModuleTableHeader() string {
	return "\n\n|  Artifacts |  Security Violations | Security Issues |\n|:------------|:---------------------|:------------------|\n"
}

func generateTableRow(nestedModuleMarkdownTree string, scanResult ScanResult) string {
	return fmt.Sprintf(" %s | %s | %s |\n", fitInsideMarkdownTable(nestedModuleMarkdownTree), appendSpacesToTableColumn(scanResult.GetViolations()), appendSpacesToTableColumn(scanResult.GetVulnerabilities()))
}

func fitInsideMarkdownTable(str string) string {
	return strings.ReplaceAll(str, "\n", "<br>")
}

func getScanResults(scannedEntity string) (sc ScanResult) {
	log.Debug("Getting scan results for: ", scannedEntity)
	if sc = StaticMarkdownConfig.scanResultsMapping[fileNameToSha1(scannedEntity)]; sc != nil {
		return sc
	}
	log.Debug("No scan results found for: ", scannedEntity)
	return StaticMarkdownConfig.scanResultsMapping[NonScannedResult]
}

func extractDockerImageTag(modules []buildInfo.Module) string {
	if len(modules) == 0 || modules[0].Type != buildInfo.Docker {
		return ""
	}

	const tagKey = "docker.image.tag"
	properties := modules[0].Properties
	switch props := properties.(type) {
	case map[string]interface{}:
		if tag, found := props[tagKey]; found {
			if tagStr, ok := tag.(string); ok {
				return tagStr
			}
		}
	case map[string]string:
		if tag, found := props[tagKey]; found {
			return tag
		}
	}

	return ""
}

// Filter out unsupported modules, return empty list if no supported modules found.
func filterModules(modules ...buildInfo.Module) []buildInfo.Module {
	supportedModules := make([]buildInfo.Module, 0, len(modules))
	for _, module := range modules {
		if !isSupportedModule(&module) {
			continue
		}
		supportedModules = append(supportedModules, module)
	}
	return supportedModules
}
