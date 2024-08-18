package commandssummaries

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
	assert.Equal(t, getTestDataFile(t, "table.md"), buildInfoSummary.buildInfoTable(builds))
}

func TestBuildInfoModules(t *testing.T) {
	buildInfoSummary := &BuildInfoSummary{platformUrl: platformUrl, platformMajorVersion: 7}
	var builds = []*buildinfo.BuildInfo{
		{
			Name:     "buildName",
			Number:   "123",
			Started:  "2024-05-05T12:47:20.803+0300",
			BuildUrl: "http://myJFrogPlatform/builds/buildName/123",
			Modules: []buildinfo.Module{
				{
					Id: "gradle",
					// Validate that ignored types don't show.
					Type: buildinfo.Gradle,
					Artifacts: []buildinfo.Artifact{
						{
							Name:                   "gradleArtifact",
							Path:                   "dir/gradleArtifact",
							OriginalDeploymentRepo: "gradle-local",
						},
					},
				},
				{
					Id:   "maven",
					Type: buildinfo.Maven,
					Artifacts: []buildinfo.Artifact{{
						Name:                   "artifact1",
						Path:                   "path/to/artifact1",
						OriginalDeploymentRepo: "libs-release",
					}},
					// Validate that dependencies don't show.
					Dependencies: []buildinfo.Dependency{{
						Id: "dep1",
					},
					},
				},
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

	result := buildInfoSummary.buildInfoModules(builds)
	// Validate that the markdown contains the expected "generic" repo content as well as the "maven" repo content.
	assert.Contains(t, result, getTestDataFile(t, "generic.md"))
	assert.Contains(t, result, getTestDataFile(t, "maven.md"))
	// The build-info also contains a "gradle" module, but it should not be included in the markdown.
	assert.NotContains(t, result, "gradle")
}

// Validate that if no supported module with artifacts was found, we avoid generating the markdown.
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
					},
					},
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

	assert.Empty(t, buildInfoSummary.buildInfoModules(builds))
}

func TestBuildInfoModulesWithGrouping(t *testing.T) {
	gh := &BuildInfoSummary{platformUrl: platformUrl, platformMajorVersion: 7}
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
					Id:     "linux/arm/multiarch-image:1",
					Artifacts: []buildinfo.Artifact{
						{
							Checksum: buildinfo.Checksum{
								Sha1:   "63d3ac90f9cd322b76543d7bf96eeb92417faf41",
								Sha256: "33b5b5485e88e63d3630e5dcb008f98f102b0f980a9daa31bd976efdec7a8e4c",
								Md5:    "99bbb1e1035aea4d9150e4348f24e107",
							},
							Name:                   "sha256__33b5b5485e88e63d3630e5dcb008f98f102b0f980a9daa31bd976efdec7a8e4c",
							Path:                   "multiarch-image/sha256:686085b9972e0f7a432b934574e3dca27b4fa0a3d10d0ae7099010160db6d338/sha256__33b5b5485e88e63d3630e5dcb008f98f102b0f980a9daa31bd976efdec7a8e4c",
							OriginalDeploymentRepo: "docker-local",
						},
						{
							Checksum: buildinfo.Checksum{
								Sha1:   "9dceac352f990a3149ff97ab605c3c8833409abf",
								Sha256: "5480d2ca1740c20ce17652e01ed2265cdc914458acd41256a2b1ccff28f2762c",
								Md5:    "d6a694604c7e58b2c788dec5656a1add",
							},
							Name:                   "sha256__5480d2ca1740c20ce17652e01ed2265cdc914458acd41256a2b1ccff28f2762c",
							Path:                   "multiarch-image/sha256:686085b9972e0f7a432b934574e3dca27b4fa0a3d10d0ae7099010160db6d338/sha256__5480d2ca1740c20ce17652e01ed2265cdc914458acd41256a2b1ccff28f2762c",
							OriginalDeploymentRepo: "docker-local",
						},
					},
				},
				{
					Type:   "docker",
					Parent: "image:2",
					Id:     "image:2",
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

	result := gh.buildInfoModules(builds)
	assert.Contains(t, result, getTestDataFile(t, "image2.md"))
	assert.Contains(t, result, getTestDataFile(t, "multiarch-image1.md"))
}

func getTestDataFile(t *testing.T, fileName string) string {
	modulesPath := filepath.Join(".", "testdata", fileName)
	content, err := os.ReadFile(modulesPath)
	assert.NoError(t, err)
	contentStr := string(content)
	if coreutils.IsWindows() {
		contentStr = strings.ReplaceAll(contentStr, "\r\n", "\n")
	}
	return contentStr
}

func TestParseBuildTime(t *testing.T) {
	// Test format
	actual := parseBuildTime("2006-01-02T15:04:05.000-0700")
	expected := "Jan 2, 2006 , 15:04:05"
	assert.Equal(t, expected, actual)
	// Test invalid format
	expected = "N/A"
	actual = parseBuildTime("")
	assert.Equal(t, expected, actual)
}
