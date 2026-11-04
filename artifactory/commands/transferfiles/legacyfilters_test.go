package transferfiles

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_isGeneratedPropertyKey_matchesPluginExactSets(t *testing.T) {
	cases := []struct {
		name        string
		packageType string
		generated   []string
		eligible    []string
	}{
		{
			name:        "go",
			packageType: "Go",
			generated:   []string{"go.name", "package.lowercase"},
			eligible:    []string{"go.version", "build.name", "npm.name"},
		},
		{
			name:        "npm",
			packageType: "npm",
			generated: []string{"npm.name", "npm.version", "npm.description", "npm.keywords", "npm.deprecated",
				"npm.detachedtags", "artifactory.metadata.exclude"},
			eligible: []string{"npm.custom", "build.name", "go.name"},
		},
		{
			name:        "pub",
			packageType: "Pub",
			generated: []string{"pub.name", "pub.version", "pub.description", "pub.homepage", "pub.repository",
				"pub.dependencies", "pub.devDependencies", "pub.environment", "baseUrl"},
			eligible: []string{"pub.custom", "build.name", "npm.name"},
		},
		{
			name:        "rpm",
			packageType: "rpm",
			generated: []string{"rpm.metadata.name", "rpm.metadata.arch", "rpm.metadata.version", "rpm.metadata.license",
				"rpm.metadata.release", "rpm.metadata.epoch", "rpm.metadata.group", "rpm.metadata.vendor",
				"rpm.metadata.summary"},
			eligible: []string{"rpm.metadata.custom", "build.name", "rpm.custom"},
		},
		{
			name:        "chef",
			packageType: "chef",
			generated: []string{"chef.name", "chef.version", "chef.maintainer", "chef.description", "chef.external",
				"chef.issues", "chef.dependencies", "chef.platforms", "artifactory.licenses"},
			eligible: []string{"chef.custom", "build.name"},
		},
		{
			name:        "gems",
			packageType: "Gems",
			generated:   []string{"gem.name", "gem.version", "gem.platform", "gem.runtime.dependencies", "ruby"},
			eligible:    []string{"gem.custom", "build.name", "gems.custom"},
		},
		{
			name:        "helm",
			packageType: "helm",
			generated: []string{"chart.name", "chart.version", "chart.appVersion", "chart.description", "chart.home",
				"chart.created", "chart.type", "chart.annotations", "chart.apiVersion", "chart.isDeprecated",
				"chart.sources", "chart.maintainers", "chart.dependencies"},
			eligible: []string{"chart.custom", "build.name", "helm.custom"},
		},
		{
			name:        "opkg",
			packageType: "opkg",
			generated: []string{"opkg.architecture", "opkg.name", "opkg.version", "opkg.maintainer", "opkg.priority",
				"opkg.section", "opkg.website"},
			eligible: []string{"opkg.custom", "build.name"},
		},
		{
			name:        "pypi",
			packageType: "pypi",
			generated:   []string{"pypi.name", "pypi.normalized.name", "pypi.version", "pypi.summary", "pypi.requires.python"},
			eligible:    []string{"pypi.custom", "build.name"},
		},
		{
			name:        "bower",
			packageType: "bower",
			generated:   []string{"bower.name", "bower.version", "bower.pkg"},
			eligible:    []string{"bower.custom", "build.name"},
		},
		{
			name:        "cargo",
			packageType: "cargo",
			generated: []string{"crate.name", "crate.version", "crate.description", "crate.keywords", "crate.categories",
				"crate.dependencies", "crate.features"},
			eligible: []string{"crate.custom", "build.name"},
		},
		{
			name:        "conan",
			packageType: "conan",
			generated: []string{"conan.recipe_hash", "conan.requires", "conan.packages.author", "conan.packages.license",
				"conan.packages.url", "conan.settings.os", "conan.settings.compiler.version", "conan.settings.options"},
			eligible: []string{"conan.name", "conan.author", "build.name"},
		},
		{
			name:        "conda",
			packageType: "conda",
			generated:   []string{"conda.name", "conda.version", "conda.arch", "conda.platform", "artifactory.licenses"},
			eligible:    []string{"conda.custom", "build.name"},
		},
		{
			name:        "nuget",
			packageType: "NuGet",
			generated: []string{"nuget.id", "nuget.version", "nuget.title", "nuget.authors", "nuget.summary",
				"nuget.copyright", "nuget.releaseNotes", "nuget.owners", "nuget.description",
				"nuget.requireLicenseAcceptance", "nuget.projectUrl", "nuget.iconUrl", "nuget.licenseUrl",
				"nuget.tags", "nuget.language", "nuget.digest", "nuget.dependency", "nuget.reference",
				"nuget.frameworks"},
			eligible: []string{"nuget.custom", "build.name"},
		},
		{
			name:        "swift",
			packageType: "swift",
			generated:   []string{"swift.name", "swift.version"},
			eligible:    []string{"swift.custom", "build.name"},
		},
		{
			name:        "alpine",
			packageType: "alpine",
			generated:   []string{"alpine.name", "alpine.version", "alpine.branch", "alpine.repository", "alpine.architecture"},
			eligible:    []string{"alpine.origin", "alpine.maintainer", "build.name"},
		},
		{
			name:        "debian",
			packageType: "debian",
			generated: []string{"deb.name", "deb.version", "deb.maintainer", "deb.priority", "deb.section",
				"deb.website", "artifactory.licenses"},
			eligible: []string{"deb.distribution", "deb.component", "deb.architecture", "build.name", "debian.custom"},
		},
		{
			name:        "puppet",
			packageType: "puppet",
			generated: []string{"puppet.name", "puppet.version", "puppet.description", "puppet.module_groups",
				"puppet.supported", "puppet.license", "puppet.author", "puppet.tags", "puppet.dependencies"},
			eligible: []string{"puppet.custom", "build.name"},
		},
		{
			name:        "composer",
			packageType: "composer",
			generated: []string{"composer.name", "composer.version", "composer.description", "composer.author",
				"composer.type", "composer.dependencies", "composer.keywords", "composer.full.reindex",
				"artifactory.licenses"},
			eligible: []string{"composer.custom", "build.name"},
		},
		{
			name:        "cocoapods",
			packageType: "cocoapods",
			generated:   []string{"pods.name", "pods.version", "pods.git.org", "pods.git.repo", "pods.git.version"},
			eligible:    []string{"pods.custom", "build.name"},
		},
		{
			name:        "terraform",
			packageType: "terraform",
			generated: []string{"terraform.id", "terraform.namespace", "terraform.version", "terraform.name",
				"terraform.provider", "terraform.flavor", "terraform.type"},
			eligible: []string{"terraform.custom", "build.name"},
		},
		{
			name:        "maven-no-prefix",
			packageType: "maven",
			eligible: []string{"artifactory.licenses", "artifactory.metadata.exclude", "package.lowercase", "ruby",
				"baseUrl", "conan.settings.os", "build.name", "maven.name"},
		},
		{
			name:        "docker-no-prefix",
			packageType: "docker",
			eligible: []string{"artifactory.licenses", "artifactory.metadata.exclude", "package.lowercase", "ruby",
				"baseUrl", "conan.settings.os", "build.name", "docker.manifest"},
		},
		{
			name:        "generic-no-prefix",
			packageType: "generic",
			eligible: []string{"artifactory.licenses", "artifactory.metadata.exclude", "package.lowercase", "ruby",
				"baseUrl", "conan.settings.os", "build.name", "npm.name", "go.name"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			for _, key := range tc.generated {
				assert.True(t, isGeneratedPropertyKey(key, tc.packageType), "expected generated key %q for %q", key, tc.packageType)
			}
			for _, key := range tc.eligible {
				assert.False(t, isGeneratedPropertyKey(key, tc.packageType), "expected eligible key %q for %q", key, tc.packageType)
			}
		})
	}
}
