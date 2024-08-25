package commandsummary

import (
	buildinfo "github.com/jfrog/build-info-go/entities"
	"github.com/jfrog/jfrog-cli-core/v2/utils/coreutils"
	"github.com/stretchr/testify/assert"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildInfoTable(t *testing.T) {
	buildInfoSummary := &BuildInfoSummary{}
	var builds = []*buildinfo.BuildInfo{
		{
			Name:     "buildName",
			Number:   "123",
			Started:  "2024-05-05T12:47:20.803+0300",
			BuildUrl: "http://myJFrogPlatform/builds/buildName/123",
		},
	}

	t.Run("Extended Summary", func(t *testing.T) {
		setExtendedSummary(true)
		assert.Equal(t, getTestDataFile(t, "build-info-table.md"), buildInfoSummary.buildInfoTable(builds))
	})

	t.Run("Basic Summary", func(t *testing.T) {
		setExtendedSummary(false)
		assert.Equal(t, getTestDataFile(t, "build-info-table.md"), buildInfoSummary.buildInfoTable(builds))
	})

	cleanCommandSummaryValues()
}

func TestBuildInfoModulesMaven(t *testing.T) {
	buildInfoSummary := &BuildInfoSummary{}
	var builds = []*buildinfo.BuildInfo{
		{
			Name:     "buildName",
			Number:   "123",
			Started:  "2024-05-05T12:47:20.803+0300",
			BuildUrl: "http://myJFrogPlatform/builds/buildName/123",
			Modules: []buildinfo.Module{
				{
					Id:   "maven",
					Type: buildinfo.Maven,
					Artifacts: []buildinfo.Artifact{{
						Name:                   "artifact1",
						Path:                   "path/to/artifact1",
						OriginalDeploymentRepo: "libs-release",
					}},
					Dependencies: []buildinfo.Dependency{{
						Id: "dep1",
					}},
				},
			},
		},
	}
	setPlatformUrl(testPlatformUrl)
	t.Run("Extended Summary", func(t *testing.T) {
		setExtendedSummary(true)
		result, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		assert.Equal(t, getTestDataFile(t, "maven-module.md"), result)
	})
	t.Run("Basic Summary", func(t *testing.T) {
		setExtendedSummary(false)
		result, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		assert.Equal(t, getTestDataFile(t, "maven-module.md"), result)
	})
	cleanCommandSummaryValues()
}

func TestBuildInfoModulesGeneric(t *testing.T) {
	buildInfoSummary := &BuildInfoSummary{}
	var builds = []*buildinfo.BuildInfo{
		{
			Name:     "buildName",
			Number:   "123",
			Started:  "2024-05-05T12:47:20.803+0300",
			BuildUrl: "http://myJFrogPlatform/builds/buildName/123",
			Modules: []buildinfo.Module{
				{
					Id:   "generic",
					Type: buildinfo.Generic,
					Artifacts: []buildinfo.Artifact{{
						Name:                   "artifact2",
						Path:                   "path/to/artifact2",
						OriginalDeploymentRepo: "generic-local",
					}},
				},
			},
		},
	}

	setPlatformUrl(testPlatformUrl)
	t.Run("Extended Summary", func(t *testing.T) {
		setExtendedSummary(true)
		result, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		assert.Equal(t, getTestDataFile(t, "generic-module.md"), result)
	})
	t.Run("Basic Summary", func(t *testing.T) {
		setExtendedSummary(false)
		result, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		assert.Equal(t, getTestDataFile(t, "generic-module.md"), result)
	})
	cleanCommandSummaryValues()
}

func TestBuildInfoModulesEmpty(t *testing.T) {
	buildInfoSummary := &BuildInfoSummary{}
	var builds = []*buildinfo.BuildInfo{
		{
			Name:     "buildName",
			Number:   "123",
			Started:  "2024-05-05T12:47:20.803+0300",
			BuildUrl: "http://myJFrogPlatform/builds/buildName/123",
			Modules: []buildinfo.Module{
				{
					Id:        "maven",
					Type:      buildinfo.Maven,
					Artifacts: []buildinfo.Artifact{},
					Dependencies: []buildinfo.Dependency{{
						Id: "dep1",
					}},
				},
				{
					Id:   "gradle",
					Type: buildinfo.Gradle,
					Artifacts: []buildinfo.Artifact{
						{
							Name:                   "gradleArtifact",
							Path:                   "dir/gradleArtifact",
							OriginalDeploymentRepo: "gradle-local",
						},
					},
				},
			},
		},
	}

	t.Run("ExtendedSummary", func(t *testing.T) {
		setExtendedSummary(true)
		result, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("BasicSummary", func(t *testing.T) {
		setExtendedSummary(true)
		result, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		assert.Empty(t, result)
	})
}

func TestDockerMultiArchView(t *testing.T) {
	buildInfoSummary := &BuildInfoSummary{}
	var builds = []*buildinfo.BuildInfo{
		{
			Name:    "dockerx",
			Number:  "1",
			Started: "2024-08-12T11:11:50.198+0300",
			Modules: []buildinfo.Module{
				{
					Properties: map[string]string{
						"docker.image.tag": "ecosysjfrog.jfrog.io/docker-local/multiarch-image:1",
					},
					Type: "docker",
					Id:   "multiarch-image:1",
					Artifacts: []buildinfo.Artifact{
						{
							Type: "json",
							Checksum: buildinfo.Checksum{
								Sha1:   "faf9824aca9d192e16c2f8a6670b149392465ce7",
								Sha256: "2217c766cddcd2d24994caaf7713db556a0fa8de108a946ebe5b0369f784a59a",
								Md5:    "ba0519ebb6feef0edefa03a7afb05406",
							},
							Name:                   "list.manifest.json",
							Path:                   "multiarch-image/1/list.manifest.json",
							OriginalDeploymentRepo: "docker-local",
						},
					},
				},
				{
					Type:   "docker",
					Parent: "multiarch-image:1",
					Id:     "linux/amd64/multiarch-image:1",
					Artifacts: []buildinfo.Artifact{
						{
							Checksum: buildinfo.Checksum{
								Sha1:   "32c1416f8430fbbabd82cb014c5e09c5fe702404",
								Sha256: "sha256:552ccb2628970ef526f13151a0269258589fc8b5701519a9c255c4dd224b9a21",
								Md5:    "f568bfb1c9576a1f06235ebe0389d2d8",
							},
							Name:                   "manifest.json",
							Path:                   "multiarch-image/sha256__552ccb2628970ef526f13151a0269258589fc8b5701519a9c255c4dd224b9a21",
							OriginalDeploymentRepo: "docker-local",
						},
						{
							Checksum: buildinfo.Checksum{
								Sha1:   "32c1416f8430fbbabd82cb014c5e09c5fe702404",
								Sha256: "aee9d258e62f0666e3286acca21be37d2e39f69f8dde74454b9f3cd8ef437e4e",
								Md5:    "f568bfb1c9576a1f06235ebe0389d2d8",
							},
							Name:                   "sha256__aee9d258e62f0666e3286acca21be37d2e39f69f8dde74454b9f3cd8ef437e4e",
							Path:                   "multiarch-image/sha256:552ccb2628970ef526f13151a0269258589fc8b5701519a9c255c4dd224b9a21/sha256__aee9d258e62f0666e3286acca21be37d2e39f69f8dde74454b9f3cd8ef437e4e",
							OriginalDeploymentRepo: "docker-local",
						},
					},
				},
				{
					Type:   "docker",
					Parent: "multiarch-image:1",
					Id:     "linux/arm64/multiarch-image:1",
					Artifacts: []buildinfo.Artifact{
						{
							Checksum: buildinfo.Checksum{
								Sha1:   "32c1416f8430fbbabd82cb014c5e09c5fe702404",
								Sha256: "sha256:bee6dc0408dfd20c01e12e644d8bc1d60ff100a8c180d6c7e85d374c13ae4f92",
								Md5:    "f568bfb1c9576a1f06235ebe0389d2d8",
							},
							Name:                   "manifest.json",
							Path:                   "multiarch-image/sha256__bee6dc0408dfd20c01e12e644d8bc1d60ff100a8c180d6c7e85d374c13ae4f92",
							OriginalDeploymentRepo: "docker-local",
						},
						{
							Checksum: buildinfo.Checksum{
								Sha1:   "82b6d4ae1f673c609469a0a84170390ecdff5a38",
								Sha256: "1f17f9d95f85ba55773db30ac8e6fae894831be87f5c28f2b58d17f04ef65e93",
								Md5:    "d178dd8c1e1fded51ade114136ebdaf2",
							},
							Name:                   "sha256__1f17f9d95f85ba55773db30ac8e6fae894831be87f5c28f2b58d17f04ef65e93",
							Path:                   "multiarch-image/sha256:bee6dc0408dfd20c01e12e644d8bc1d60ff100a8c180d6c7e85d374c13ae4f92/sha256__1f17f9d95f85ba55773db30ac8e6fae894831be87f5c28f2b58d17f04ef65e93",
							OriginalDeploymentRepo: "docker-local",
						},
					},
				},
				{
					Type:   "docker",
					Parent: "multiarch-image:1",
					Id:     "attestations/multiarch-image:1",
					Checksum: buildinfo.Checksum{
						Sha256: "33b5b5485e88e63d3630e5dcb008f98f102b0f980a9daa31bd976efdec7a8e4c",
					},
					Artifacts: []buildinfo.Artifact{
						{
							Checksum: buildinfo.Checksum{
								Sha1:   "63d3ac90f9cd322b76543d7bf96eeb92417faf41",
								Sha256: "33b5b5485e88e63d3630e5dcb008f98f102b0f980a9daa31bd976efdec7a8e4c",
								Md5:    "99bbb1e1035aea4d9150e4348f24e107",
							},
							Name:                   "sha256:67a5a1efd2df970568a17c1178ec5df786bbf627274f285c6dbce71fae9ebe57",
							Path:                   "multiarch-image/sha256:686085b9972e0f7a432b934574e3dca27b4fa0a3d10d0ae7099010160db6d338/sha256__33b5b5485e88e63d3630e5dcb008f98f102b0f980a9daa31bd976efdec7a8e4c",
							OriginalDeploymentRepo: "docker-local",
						},
					},
				},
			},
		},
	}
	setPlatformUrl(testPlatformUrl)
	t.Run("Extended Summary", func(t *testing.T) {
		setExtendedSummary(true)
		result, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		assert.Equal(t, result, getTestDataFile(t, "multiarch-docker-image.md"))
	})

	t.Run("Basic Summary", func(t *testing.T) {
		setExtendedSummary(false)
		result, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		assert.Equal(t, result, getTestDataFile(t, "multiarch-docker-image.md"))
	})
	cleanCommandSummaryValues()
}

func TestDockerModuleView(t *testing.T) {
	buildInfoSummary := &BuildInfoSummary{}
	var builds = []*buildinfo.BuildInfo{
		{
			Name: "docker-image",
			Modules: []buildinfo.Module{
				{
					Type:   "docker",
					Parent: "image:2",
					Id:     "image:2",
					Checksum: buildinfo.Checksum{
						Sha256: "aee9d258e62f0666e3286acca21be37d2e39f69f8dde74454b9f3cd8ef437e4e",
					},
					Artifacts: []buildinfo.Artifact{
						{
							Checksum: buildinfo.Checksum{
								Sha1:   "32c1416f8430fbbabd82cb014c5e09c5fe702404",
								Sha256: "aee9d258e62f0666e3286acca21be37d2e39f69f8dde74454b9f3cd8ef437e4e",
								Md5:    "f568bfb1c9576a1f06235ebe0389d2d8",
							},
							Name:                   "sha256__aee9d258e62f0666e3286acca21be37d2e39f69f8dde74454b9f3cd8ef437e4e",
							Path:                   "image2/sha256:552ccb2628970ef526f13151a0269258589fc8b5701519a9c255c4dd224b9a21/sha256__aee9d258e62f0666e3286acca21be37d2e39f69f8dde74454b9f3cd8ef437e4e",
							OriginalDeploymentRepo: "docker-local",
						},
					},
				},
			},
		},
	}
	setPlatformUrl(testPlatformUrl)
	t.Run("Extended Summary", func(t *testing.T) {
		setExtendedSummary(true)
		result, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		assert.Equal(t, getTestDataFile(t, "docker-image-module.md"), result)
	})

	t.Run("Basic Summary", func(t *testing.T) {
		setExtendedSummary(false)
		result, err := buildInfoSummary.buildInfoModules(builds)
		assert.NoError(t, err)
		assert.Equal(t, getTestDataFile(t, "docker-image-module.md"), result)
	})
	cleanCommandSummaryValues()
}

// Tests data files are location artifactory/commands/testdata/command_summary
func getTestDataFile(t *testing.T, fileName string) string {
	var modulesPath string
	if isExtendedSummary() {
		modulesPath = filepath.Join("../", "testdata", "command_summaries", "extended", fileName)
	} else {
		modulesPath = filepath.Join("../", "testdata", "command_summaries", "basic", fileName)
	}

	content, err := os.ReadFile(modulesPath)
	assert.NoError(t, err)
	contentStr := string(content)
	if coreutils.IsWindows() {
		contentStr = strings.ReplaceAll(contentStr, "\r\n", "\n")
	}
	return contentStr
}

// Return config values to the default state
func cleanCommandSummaryValues() {
	setExtendedSummary(false)
	setPlatformMajorVersion(0)
}
