package commandsummary

import (
	"fmt"
	buildInfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/container"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"path"
	"strings"
)

const (
	basicSummaryUpgradeNotice = "<a href=\"%s\">🐸 Enable the linkage to Artifactory</a>\n\n"
	modulesTitle              = "📦 Modules published to Artifactory by this workflow"
	minTableColumnLength      = 400
	markdownSpaceFiller       = "&nbsp;"
	NonScannedResult          = "non-scanned"
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

	buildInfoTableMarkdown := bis.buildInfoTable(builds)
	publishedModulesMarkdown := bis.buildInfoModules(builds)
	if publishedModulesMarkdown != "" {
		publishedModulesMarkdown = WrapCollapsableMarkdown(modulesTitle, publishedModulesMarkdown, 2)
	}
	finalMarkdown = buildInfoTableMarkdown + publishedModulesMarkdown

	return WrapCollapsableMarkdown(bis.GetSummaryTitle(), finalMarkdown, 3), nil
}

// Create a table with published builds and possible scan results.
func (bis *BuildInfoSummary) buildInfoTable(builds []*buildInfo.BuildInfo) string {
	var tableBuilder strings.Builder
	tableBuilder.WriteString(getBuildInfoTableHeader())
	for _, build := range builds {
		appendBuildInfoRow(&tableBuilder, build)
	}
	tableBuilder.WriteString("\n\n")
	return tableBuilder.String()
}

// Generates a view for published modules within the build.
// Modules are displayed as tables if they are scannable via CLI command,
// otherwise, they are shown as an artifact tree.
func (bis *BuildInfoSummary) buildInfoModules(builds []*buildInfo.BuildInfo) string {
	var markdownBuilder strings.Builder
	markdownBuilder.WriteString("\n\n<h3>Published Modules</h3>\n\n")
	var shouldGenerate bool
	for _, build := range builds {
		supportedModules := filterModules(build.Modules...)
		if modulesMarkdown := bis.generateModulesMarkdown(supportedModules...); modulesMarkdown != "" {
			markdownBuilder.WriteString(modulesMarkdown)
			shouldGenerate = true
		}
	}
	if !shouldGenerate {
		return ""
	}
	return markdownBuilder.String()
}

func (bis *BuildInfoSummary) generateModulesMarkdown(modules ...buildInfo.Module) string {
	var modulesMarkdown strings.Builder
	// Modules could include nested modules inside of them
	// Group the modules by their root module ID
	// If a module has no root, it is considered as a root module itself.
	groupedModuleMap := groupModules(modules)
	if len(groupedModuleMap) == 0 {
		return ""
	}
	for rootModuleID, subModules := range groupedModuleMap {
		if len(subModules) == 0 {
			continue
		}
		if !scannableModuleType[subModules[0].Type] {
			modulesMarkdown.WriteString(bis.generateModuleArtifactTree(rootModuleID, subModules))
		} else {
			modulesMarkdown.WriteString(bis.generateModuleTableView(rootModuleID, subModules))
		}
	}
	return modulesMarkdown.String()
}

func (bis *BuildInfoSummary) generateModuleArtifactTree(rootModuleID string, nestedModules []buildInfo.Module) string {
	if len(nestedModules) == 0 {
		return ""
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
		markdownBuilder.WriteString(fmt.Sprintf("\n\n<pre>%s</pre>\n\n", bis.generateModuleArtifactsTree(&module, isMultiModule)))
	}
	return markdownBuilder.String()
}

func (bis *BuildInfoSummary) generateModuleTableView(rootModuleID string, subModules []buildInfo.Module) string {
	var markdownBuilder strings.Builder
	markdownBuilder.WriteString(generateModuleHeader(rootModuleID))
	markdownBuilder.WriteString(generateModuleTableHeader())
	isMultiModule := len(subModules) > 1
	nestedModuleMarkdownTree := bis.generateTableModuleMarkdown(subModules, rootModuleID, isMultiModule)
	scanResult := getScanResults(extractDockerImageTag(subModules))
	markdownBuilder.WriteString(generateTableRow(nestedModuleMarkdownTree, scanResult))
	return markdownBuilder.String()
}

func (bis *BuildInfoSummary) generateTableModuleMarkdown(nestedModules []buildInfo.Module, parentModuleID string, isMultiModule bool) string {
	var nestedModuleMarkdownTree strings.Builder
	if len(nestedModules) == 0 {
		return ""
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
		nestedModuleMarkdownTree.WriteString(bis.generateModuleArtifactsTree(&module, isMultiModule))
	}
	nestedModuleMarkdownTree.WriteString(appendSpacesToTableColumn(""))
	nestedModuleMarkdownTree.WriteString("</pre>")
	return nestedModuleMarkdownTree.String()
}

func (bis *BuildInfoSummary) generateModuleArtifactsTree(module *buildInfo.Module, shouldCollapseArtifactsTree bool) string {
	artifactsTree := bis.createArtifactsTree(module)
	if shouldCollapseArtifactsTree {
		return bis.generateModuleCollapsibleSection(module, artifactsTree)
	}
	return artifactsTree
}

func (bis *BuildInfoSummary) generateModuleCollapsibleSection(module *buildInfo.Module, sectionContent string) string {
	switch module.Type {
	case buildInfo.Docker:
		return createCollapsibleSection(createDockerMultiArchTitle(module), sectionContent)
	default:
		return createCollapsibleSection(module.Id, sectionContent)
	}
}

func (bis *BuildInfoSummary) createArtifactsTree(module *buildInfo.Module) string {
	artifactsTree := utils.NewFileTree()
	for _, artifact := range module.Artifacts {
		var artifactUrlInArtifactory string
		if StaticMarkdownConfig.IsExtendedSummary() {
			artifactUrlInArtifactory = generateArtifactUrl(artifact)
		}
		if artifact.OriginalDeploymentRepo == "" {
			artifact.OriginalDeploymentRepo = " "
		}
		artifactTreePath := path.Join(artifact.OriginalDeploymentRepo, artifact.Path)
		artifactsTree.AddFile(artifactTreePath, artifactUrlInArtifactory)
	}
	return artifactsTree.String()
}

func generateArtifactUrl(artifact buildInfo.Artifact) string {
	if strings.TrimSpace(artifact.OriginalDeploymentRepo) == "" {
		return ""
	}
	return GenerateArtifactUrl(path.Join(artifact.OriginalDeploymentRepo, artifact.Path))
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
		return !strings.HasPrefix(module.Id, container.AttestationsModuleIdPrefix)
	}
	return true
}

func createDockerMultiArchTitle(module *buildInfo.Module) string {
	parentImageName := strings.Split(module.Parent, ":")[0]
	var sha256 string
	for _, artifact := range module.Artifacts {
		if artifact.Name == container.ManifestJsonFile {
			sha256 = artifact.Sha256
			break
		}
	}
	if StaticMarkdownConfig.IsExtendedSummary() {
		dockerModuleLink := fmt.Sprintf(artifactoryDockerPackagesUiFormat, strings.TrimSuffix(StaticMarkdownConfig.GetPlatformUrl(), "/"), "%2F%2F"+parentImageName, sha256)
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

func appendBuildInfoRow(tableBuilder *strings.Builder, build *buildInfo.BuildInfo) {
	buildName := build.Name + " " + build.Number
	buildScanResult := getScanResults(buildName)
	if StaticMarkdownConfig.IsExtendedSummary() {
		tableBuilder.WriteString(fmt.Sprintf("| [%s](%s) %s | %s | %s | \n", buildName, build.BuildUrl, appendSpacesToTableColumn(""), appendSpacesToTableColumn(buildScanResult.GetViolations()), appendSpacesToTableColumn(buildScanResult.GetVulnerabilities())))
	} else {
		upgradeMessage := fmt.Sprintf(basicSummaryUpgradeNotice, StaticMarkdownConfig.GetExtendedSummaryLangPage())
		buildName = fmt.Sprintf(" %s %s", upgradeMessage, buildName)
		tableBuilder.WriteString(fmt.Sprintf("| %s %s | %s | %s |\n", fitInsideMarkdownTable(buildName), appendSpacesToTableColumn(""), appendSpacesToTableColumn(buildScanResult.GetViolations()), appendSpacesToTableColumn(buildScanResult.GetVulnerabilities())))
	}
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
