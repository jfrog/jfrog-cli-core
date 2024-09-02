package commandsummary

import (
	"fmt"
	buildInfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils"
	"github.com/jfrog/jfrog-cli-core/v2/artifactory/utils/container"
	"path"
	"strings"
)

const (
	basicSummaryUpgradeNotice = "<a href=\"%s\">🐸 Enable the linkage to Artifactory</a>\n\n"
	modulesTitle              = "📦 Artifacts published to Artifactory by this workflow"
	minTableColumnLength      = 400
	markdownSpaceFiller       = "&nbsp;"
	NonScannedResult          = "non-scanned"
)

// List of scan-able modules which will be displayed inside a table.
var scannedModuleTypes = map[buildInfo.ModuleType]bool{
	buildInfo.Docker: true,
}

// List of supported module types which provide the needed fields in the build info object.
var supportedModuleTypes = map[buildInfo.ModuleType]bool{
	buildInfo.Maven:     true,
	buildInfo.Npm:       true,
	buildInfo.Go:        true,
	buildInfo.Generic:   true,
	buildInfo.Terraform: true,
	buildInfo.Docker:    true,
}

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
		return "", nil
	}
	// Creates the build info table
	buildInfoTableMarkdown := bis.buildInfoTable(builds)
	// Creates the published modules
	publishedModulesMarkdown := bis.buildInfoModules(builds)
	if publishedModulesMarkdown != "" {
		publishedModulesMarkdown = WrapCollapsableMarkdown(modulesTitle, publishedModulesMarkdown, 2)
	}
	finalMarkdown = buildInfoTableMarkdown + publishedModulesMarkdown

	// Wrap the content under a collapsible section
	finalMarkdown = WrapCollapsableMarkdown(bis.GetSummaryTitle(), finalMarkdown, 3)
	return
}

func (bis *BuildInfoSummary) buildInfoTable(builds []*buildInfo.BuildInfo) string {
	var tableBuilder strings.Builder
	// Write table header
	tableBuilder.WriteString(getBuildInfoTableHeader())
	// Add rows
	for _, build := range builds {
		appendBuildRow(&tableBuilder, build)
	}
	// Add a new line after the table
	tableBuilder.WriteString("\n\n")
	return tableBuilder.String()
}

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
	// If no modules were generated, return an empty string
	if !shouldGenerate {
		return ""
	}
	return markdownBuilder.String()
}

func (bis *BuildInfoSummary) generateModulesMarkdown(modules ...buildInfo.Module) string {
	var modulesMarkdown strings.Builder
	// Group a module with it's subModules
	// If a module has no subModules, it will be grouped with itself as root.
	groupedModuleMap := groupModules(modules)
	if len(groupedModuleMap) == 0 {
		return ""
	}
	for rootModuleID, subModules := range groupedModuleMap {
		// A module is considered scan-enabled if it can be scanned via a CLI command.
		// Scan-enabled modules should be displayed in a table,
		// Non scan-enabled modules should only display the artifacts tree.
		if !scannedModuleTypes[subModules[0].Type] {
			modulesMarkdown.WriteString(bis.generateModuleArtifactTree(subModules))
		} else {
			modulesMarkdown.WriteString(bis.generateModuleTableView(rootModuleID, subModules))
		}
	}
	return modulesMarkdown.String()
}

// Create a markdown tree for the module artifacts.
func (bis *BuildInfoSummary) generateModuleArtifactTree(modules []buildInfo.Module) string {
	var markdownBuilder strings.Builder
	for _, module := range modules {
		markdownBuilder.WriteString(generateModuleHeader(module.Id))
		if !StaticMarkdownConfig.IsExtendedSummary() {
			markdownBuilder.WriteString(fmt.Sprintf(basicSummaryUpgradeNotice, StaticMarkdownConfig.GetExtendedSummaryLangPage()))
		}
		markdownBuilder.WriteString(fmt.Sprintf("\n\n<pre>%s</pre>\n\n", bis.createArtifactsTree(&module)))
	}
	return markdownBuilder.String()
}

// Creates a table view for the module with possible scan results.
func (bis *BuildInfoSummary) generateModuleTableView(rootModuleID string, subModules []buildInfo.Module) string {
	var markdownBuilder strings.Builder
	markdownBuilder.WriteString(generateModuleHeader(rootModuleID))
	markdownBuilder.WriteString(generateModuleTableHeader())
	isMultiModule := len(subModules) > 1
	nestedModuleMarkdownTree := bis.generateNestedModuleMarkdownTree(subModules, rootModuleID, isMultiModule)
	scanResult := getScanResults(extractDockerImageTag(subModules))
	markdownBuilder.WriteString(generateTableRow(nestedModuleMarkdownTree, scanResult))
	return markdownBuilder.String()
}

func (bis *BuildInfoSummary) generateNestedModuleMarkdownTree(nestedModules []buildInfo.Module, parentModuleID string, isMultiModule bool) string {
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
			artifactUrlInArtifactory = bis.generateArtifactUrl(artifact)
		}
		if artifact.OriginalDeploymentRepo == "" {
			// Placeholder needed to build an artifact tree when repo is missing.
			artifact.OriginalDeploymentRepo = " "
		}
		artifactTreePath := path.Join(artifact.OriginalDeploymentRepo, artifact.Path)
		artifactsTree.AddFile(artifactTreePath, artifactUrlInArtifactory)
	}
	return artifactsTree.String()
}

func (bis *BuildInfoSummary) generateArtifactUrl(artifact buildInfo.Artifact) string {
	if strings.TrimSpace(artifact.OriginalDeploymentRepo) == "" {
		return ""
	}
	return GenerateArtifactUrl(path.Join(artifact.OriginalDeploymentRepo, artifact.Path))
}

// groupModules groups modules that share the same parent ID into a map where the key is the parent ID and the value is a slice of those modules.
func groupModules(modules []buildInfo.Module) map[string][]buildInfo.Module {
	parentToModulesMap := make(map[string][]buildInfo.Module, len(modules))
	for _, module := range modules {
		if len(module.Artifacts) == 0 {
			continue
		}
		parentID := module.Parent
		// If the module has no parent, that means it is the parent module itself, so we can use its ID as the parent ID.
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
	// Special case for Docker: Skip attestations that are added as a module for multi-arch docker builds
	if module.Type == buildInfo.Docker {
		return !strings.HasPrefix(module.Id, container.AttestationsModuleIdPrefix)
	}
	return true
}

func createDockerMultiArchTitle(module *buildInfo.Module) string {
	// Extract the parent image name from the module ID (e.g., my-image:1.0 -> my-image)
	parentImageName := strings.Split(module.Parent, ":")[0]

	// Get the relevant SHA256
	var sha256 string
	for _, artifact := range module.Artifacts {
		if artifact.Name == container.ManifestJsonFile {
			sha256 = artifact.Sha256
			break
		}
	}

	if StaticMarkdownConfig.IsExtendedSummary() {
		// Create a link to the Docker package in Artifactory UI
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

func appendBuildRow(tableBuilder *strings.Builder, build *buildInfo.BuildInfo) {
	buildName := build.Name + " " + build.Number
	buildScanResult := getScanResults(buildName)
	if StaticMarkdownConfig.IsExtendedSummary() {
		tableBuilder.WriteString(fmt.Sprintf("| [%s](%s) %s | %s | %s | \n", buildName, build.BuildUrl, appendSpacesToTableColumn(""), appendSpacesToTableColumn(buildScanResult.GetViolations()), appendSpacesToTableColumn(buildScanResult.GetVulnerabilities())))
	} else {
		// Get the URL to the extended summary page
		upgradeMessage := fmt.Sprintf(basicSummaryUpgradeNotice, StaticMarkdownConfig.GetExtendedSummaryLangPage())
		// Append to build name to fit inside the table
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

// To fit inside the Markdown table, replace new lines with <br>
func fitInsideMarkdownTable(str string) string {
	return strings.ReplaceAll(str, "\n", "<br>")
}

func getScanResults(scannedEntity string) (sc ScanResult) {
	if sc = StaticMarkdownConfig.scanResultsMapping[fileNameToSha1(scannedEntity)]; sc != nil {
		return sc
	}
	return StaticMarkdownConfig.scanResultsMapping[NonScannedResult]
}

// Extracts the docker image tag from a docker module
// Docker modules have in their first index metadata, which contains the docker image tag
func extractDockerImageTag(modules []buildInfo.Module) string {
	if len(modules) == 0 || modules[0].Type != buildInfo.Docker {
		return ""
	}

	const tagKey = "docker.image.tag"
	properties := modules[0].Properties
	// Handle both cases where the properties are a map[string]interface{} or map[string]string
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

func filterModules(modules ...buildInfo.Module) (supportedModules []buildInfo.Module) {
	for _, module := range modules {
		if !isSupportedModule(&module) {
			continue
		}
		supportedModules = append(supportedModules, module)
	}
	return
}
