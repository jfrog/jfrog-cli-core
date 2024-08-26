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
	basicSummaryUpgradeNotice = "<a href=\"%s\">⏫ Enable the linkage to Artifactory</a>\n\n"
	minTableColumnLength      = 350
	markdownSpaceFiller       = "&nbsp;"
)

// Static mapping of scan results to be used in the summary
var ScanResultsMapping map[string]ScanResult

type BuildInfoSummary struct {
	CommandSummary
}

func NewBuildInfoSummary() (*CommandSummary, error) {
	return New(&BuildInfoSummary{}, "build-info")
}

func (bis *BuildInfoSummary) GetSummaryTitle() string {
	return "🐸 Published JFrog Build Infos"
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

	if len(builds) > 0 {
		tableInfo := bis.buildInfoTable(builds)
		modulesMarkdown, err := bis.buildInfoModules(builds)
		if err != nil {
			return "", err
		}
		finalMarkdown = tableInfo + modulesMarkdown
	}
	return WrapCollapsableMarkdown(bis.GetSummaryTitle(), finalMarkdown, 2)
}

func (bis *BuildInfoSummary) buildInfoTable(builds []*buildInfo.BuildInfo) string {
	// Generate a string that represents a Markdown table
	var tableBuilder strings.Builder
	tableBuilder.WriteString("\n\n|  Build Info |  Security Violations | Security Issues |\n")
	tableBuilder.WriteString("|:---------|:------------|:------------|\n")
	for _, build := range builds {
		appendBuildRow(&tableBuilder, build)
	}
	tableBuilder.WriteString("\n\n")
	return tableBuilder.String()
}

func (bis *BuildInfoSummary) buildInfoModules(builds []*buildInfo.BuildInfo) (string, error) {
	var markdownBuilder strings.Builder
	markdownBuilder.WriteString("\n\n### Published Modules\n\n")
	var shouldGenerate bool
	for _, build := range builds {
		if modulesMarkdown := bis.generateModulesMarkdown(build.Modules...); modulesMarkdown != "" {
			markdownBuilder.WriteString(modulesMarkdown)
			shouldGenerate = true
		}
	}

	// If no supported module with artifacts was found, avoid generating the markdown.
	if !shouldGenerate {
		return "", nil
	}
	finalString := markdownBuilder.String()
	return WrapCollapsableMarkdown("📦 Artifacts Published to Artifactory by this workflow", finalString, 4)
}

func (bis *BuildInfoSummary) generateModulesMarkdown(modules ...buildInfo.Module) string {
	var parentModulesMarkdown strings.Builder
	parentToModulesMap := groupModulesByParent(modules)
	if len(parentToModulesMap) == 0 {
		return ""
	}

	for parentModuleID, parentModules := range parentToModulesMap {
		parentModulesMarkdown.WriteString(fmt.Sprintf("\n\n**%s**\n\n", parentModuleID))
		parentModulesMarkdown.WriteString("\n\n|  Artifacts |  Security Violations | Security Issues |\n")
		parentModulesMarkdown.WriteString("|:------------|:---------------------|:------------------|\n")

		isMultiModule := len(parentModules) > 1

		var nestedModuleMarkdownTree strings.Builder
		if !StaticMarkdownConfig.IsExtendedSummary() {
			// The basic summary includes a notice to enable the linkage to Artifactory
			// Notice the UI link has to be updated.
			nestedModuleMarkdownTree.WriteString("|")
			nestedModuleMarkdownTree.WriteString(fmt.Sprintf(basicSummaryUpgradeNotice, StaticMarkdownConfig.GetPlatformUrl()))
			nestedModuleMarkdownTree.WriteString("<pre>")
		} else {
			nestedModuleMarkdownTree.WriteString("|<pre>")
		}
		for _, module := range parentModules {
			if isMultiModule && parentModuleID == module.Id {
				// Skip the parent module if there are multiple modules, as it will be displayed as a header
				continue
			}
			nestedModuleMarkdownTree.WriteString(bis.generateModuleArtifactsTree(&module, isMultiModule))
		}
		nestedModuleMarkdownTree.WriteString(appendSpacesToTableColumn(""))
		nestedModuleMarkdownTree.WriteString("</pre>")
		scanResult := getScanResults(parentModuleID)
		parentModulesMarkdown.WriteString(fmt.Sprintf(" %s | %s | %s |\n", fitInsideMarkdownTable(nestedModuleMarkdownTree.String()), appendSpacesToTableColumn(scanResult.GetViolations()), appendSpacesToTableColumn(scanResult.GetVulnerabilities())))
	}
	return parentModulesMarkdown.String()
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

// groupModulesByParent groups modules that share the same parent ID into a map where the key is the parent ID and the value is a slice of those modules.
func groupModulesByParent(modules []buildInfo.Module) map[string][]buildInfo.Module {
	parentToModulesMap := make(map[string][]buildInfo.Module, len(modules))
	for _, module := range modules {
		if len(module.Artifacts) == 0 || !isSupportedModule(&module) {
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
	switch module.Type {
	case buildInfo.Maven, buildInfo.Npm, buildInfo.Go, buildInfo.Generic, buildInfo.Terraform:
		return true
	case buildInfo.Docker:
		// Skip attestations that are added as a module for multi-arch docker builds
		return !strings.HasPrefix(module.Id, container.AttestationsModuleIdPrefix)
	default:
		return false
	}
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
		tableBuilder.WriteString(fmt.Sprintf("| [%s](%s) %s | %s | %s | \n", buildName, build.BuildUrl, appendSpacesToTableColumn(""), appendSpacesToTableColumn(buildScanResult.GetViolations()), appendSpacesToTableColumn(buildScanResult.GetViolations())))
	} else {
		tableBuilder.WriteString(fmt.Sprintf("| %s %s | %s | %s |\n", buildName, appendSpacesToTableColumn(""), appendSpacesToTableColumn(buildScanResult.GetViolations()), appendSpacesToTableColumn(buildScanResult.GetVulnerabilities())))
	}
}

// To fit inside the Markdown table, replace new lines with <br>
func fitInsideMarkdownTable(str string) string {
	return strings.ReplaceAll(str, "\n", "<br>")
}

func getScanResults(scannedEntity string) (sc ScanResult) {
	sc = ScanResultsMapping[fileNameToSha1(scannedEntity)]
	if sc != nil {
		return
	}
	return ScanResultsMapping["fallback"]
}
