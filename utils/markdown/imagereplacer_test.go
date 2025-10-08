package markdown

import (
	"image"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbedMarkdownImages_BasicImage(t *testing.T) {
	content := `![alt text](./image.png)`

	result, err := EmbedMarkdownImages([]byte(content), "", 100, 100)
	require.NoError(t, err)

	// Should keep original since image doesn't exist
	assert.Contains(t, string(result), "![alt text](./image.png)")
}

func TestEmbedMarkdownImages_ImageWithParentheses(t *testing.T) {
	content := `![the big one](./image(3).png)`

	result, err := EmbedMarkdownImages([]byte(content), "", 100, 100)
	require.NoError(t, err)

	// Should keep original since image doesn't exist
	assert.Contains(t, string(result), "![the big one](./image(3).png)")
}

func TestEmbedMarkdownImages_MultipleImages(t *testing.T) {
	content := `![first](./image1.png) and ![second](./image(2).png)`

	result, err := EmbedMarkdownImages([]byte(content), "", 100, 100)
	require.NoError(t, err)

	// Should preserve both images
	assert.Contains(t, string(result), "![first](./image1.png)")
	assert.Contains(t, string(result), "![second](./image(2).png)")
}

func TestEmbedMarkdownImages_ComplexParentheses(t *testing.T) {
	testCases := []string{
		`![test](./path/with(multiple)parentheses.png)`,
		`![test](./file(1)(2).jpg)`,
		`![test](./normal.png)`,
	}

	for _, content := range testCases {
		t.Run(content, func(t *testing.T) {
			result, err := EmbedMarkdownImages([]byte(content), "", 100, 100)
			require.NoError(t, err)
			assert.Equal(t, content, string(result))
		})
	}
}

func TestEmbedMarkdownImages_NoImages(t *testing.T) {
	content := `This is just text with no images.`

	result, err := EmbedMarkdownImages([]byte(content), "", 100, 100)
	require.NoError(t, err)

	assert.Equal(t, content, string(result))
}

func TestEmbedMarkdownImages_EmptyContent(t *testing.T) {
	content := ``

	result, err := EmbedMarkdownImages([]byte(content), "", 100, 100)
	require.NoError(t, err)

	assert.Equal(t, content, string(result))
}

func TestEmbedMarkdownImages_WithBaseDir(t *testing.T) {
	content := `![test](./image.png)`
	baseDir := "/some/base/dir"

	result, err := EmbedMarkdownImages([]byte(content), baseDir, 100, 100)
	require.NoError(t, err)

	// Should preserve original since image doesn't exist
	assert.Contains(t, string(result), "![test](./image.png)")
}

func TestFindImageReferences_Basic(t *testing.T) {
	content := `![alt text](./image.png)`

	refs := findImageReferences(content)

	require.Len(t, refs, 1)

	ref := refs[0]
	assert.Equal(t, "alt text", ref.AltText)
	assert.Equal(t, "./image.png", ref.ImagePath)
	assert.GreaterOrEqual(t, ref.StartPos, 0)
	assert.Less(t, ref.StartPos, len(content))
	assert.Greater(t, ref.EndPos, ref.StartPos)
}

func TestFindImageReferences_WithParentheses(t *testing.T) {
	content := `![the big one](./image(3).png)`

	refs := findImageReferences(content)

	require.Len(t, refs, 1)

	ref := refs[0]
	assert.Equal(t, "the big one", ref.AltText)
	assert.Equal(t, "./image(3).png", ref.ImagePath)
}

func TestFindImageReferences_Multiple(t *testing.T) {
	content := `![first](./image1.png) and ![second](./image(2).png)`

	refs := findImageReferences(content)

	require.Len(t, refs, 2)

	// Check first image
	assert.Equal(t, "first", refs[0].AltText)
	assert.Equal(t, "./image1.png", refs[0].ImagePath)

	// Check second image
	assert.Equal(t, "second", refs[1].AltText)
	assert.Equal(t, "./image(2).png", refs[1].ImagePath)
}

func TestFindImageReferences_NoImages(t *testing.T) {
	content := `This is just text with no images.`

	refs := findImageReferences(content)

	require.Len(t, refs, 0)
}

func TestFindImageReferences_DataURL(t *testing.T) {
	content := `![alt](data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==)`

	refs := findImageReferences(content)

	// Data URLs should be skipped
	require.Len(t, refs, 0)
}

func TestFindImageReferences_EdgeCases(t *testing.T) {
	testCases := []struct {
		name     string
		content  string
		expected int
	}{
		{"empty content", "", 0},
		{"only text", "just text", 0},
		{"malformed image", "![alt](", 0},
		{"incomplete image", "![alt", 0},
		{"complex path", `![test](./path/with(multiple)parentheses.png)`, 1},
		{"multiple parentheses", `![test](./file(1)(2).jpg)`, 1},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			refs := findImageReferences(tc.content)
			assert.Equal(t, tc.expected, len(refs))
		})
	}
}

func TestEmbedMarkdownImages_HappyPath_EmbedsDataURI(t *testing.T) {
	tempDir := t.TempDir()
	imgPath := filepath.Join(tempDir, "img.png")
	f, err := os.Create(imgPath)
	require.NoError(t, err)
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	require.NoError(t, png.Encode(f, img))
	require.NoError(t, f.Close())

	content := `before ![x](img.png) after`
	result, err := EmbedMarkdownImages([]byte(content), tempDir, 0, 0)
	require.NoError(t, err)
	assert.Contains(t, string(result), "data:image/png;base64,")
	assert.NotContains(t, string(result), "](img.png)")
}

func TestEmbedMarkdownImages_AvoidsCopyWhenNoChanges(t *testing.T) {
	content := `This has no images at all.`
	input := []byte(content)
	result, err := EmbedMarkdownImages(input, "", 100, 100)
	require.NoError(t, err)

	// Verify it returns the same slice when no changes are made
	assert.Equal(t, &input[0], &result[0], "Should return the same slice when no replacements are made")
}

func TestEmbedMarkdownImages_AvoidsCopyWhenAllImagesFail(t *testing.T) {
	content := `before ![x](nonexistent.png) after`
	input := []byte(content)
	result, err := EmbedMarkdownImages(input, "", 100, 100)
	require.NoError(t, err)

	// Verify it returns the same slice when all images fail to convert
	assert.Equal(t, &input[0], &result[0], "Should return the same slice when all conversions fail")
}

func TestEmbedMarkdownImages_URLImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		img := createTestImage(50, 50)
		w.Header().Set("Content-Type", "image/png")
		if err := png.Encode(w, img); err != nil {
			t.Errorf("failed to encode PNG: %v", err)
		}
	}))
	defer server.Close()

	content := `![test image](` + server.URL + `)`
	result, err := EmbedMarkdownImages([]byte(content), "", 0, 0)
	require.NoError(t, err)

	assert.Contains(t, string(result), "data:image/png;base64,")
	assert.NotContains(t, string(result), "](`+server.URL+`)")
}

func TestEmbedMarkdownImages_URLImageWithLimits(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		img := createTestImage(200, 150)
		w.Header().Set("Content-Type", "image/png")
		if err := png.Encode(w, img); err != nil {
			t.Errorf("failed to encode PNG: %v", err)
		}
	}))
	defer server.Close()

	content := `![large image](` + server.URL + `)`
	result, err := EmbedMarkdownImages([]byte(content), "", 100, 100)
	require.NoError(t, err)

	// Should keep original since image exceeds limits
	assert.Contains(t, string(result), "![large image]("+server.URL+")")
	assert.NotContains(t, string(result), "data:image/png;base64,")
}

func TestEmbedMarkdownImages_URLImageError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	content := `![missing image](` + server.URL + `)`
	result, err := EmbedMarkdownImages([]byte(content), "", 0, 0)
	require.NoError(t, err)

	// Should keep original since URL returns error
	assert.Contains(t, string(result), "![missing image]("+server.URL+")")
	assert.NotContains(t, string(result), "data:image/png;base64,")
}

func TestEmbedMarkdownImages_MixedLocalAndURL(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		img := createTestImage(50, 50)
		w.Header().Set("Content-Type", "image/png")
		if err := png.Encode(w, img); err != nil {
			t.Errorf("failed to encode PNG: %v", err)
		}
	}))
	defer server.Close()

	tempDir := t.TempDir()
	imgPath := filepath.Join(tempDir, "local.png")
	f, err := os.Create(imgPath)
	require.NoError(t, err)
	img := createTestImage(30, 30)
	require.NoError(t, png.Encode(f, img))
	require.NoError(t, f.Close())

	content := `![local](local.png) and ![remote](` + server.URL + `)`
	result, err := EmbedMarkdownImages([]byte(content), tempDir, 0, 0)
	require.NoError(t, err)

	// Both should be converted to data URIs
	assert.Contains(t, string(result), "data:image/png;base64,")
	assert.NotContains(t, string(result), "](local.png)")
	assert.NotContains(t, string(result), "]("+server.URL+")")
}

func TestEmbedMarkdownImages_URLWithDifferentFormats(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		img := createTestImage(50, 50)
		w.Header().Set("Content-Type", "image/png")
		if err := png.Encode(w, img); err != nil {
			t.Errorf("failed to encode image: %v", err)
		}
	}))
	defer server.Close()

	content := `![png image](` + server.URL + `)`
	result, err := EmbedMarkdownImages([]byte(content), "", 0, 0)
	require.NoError(t, err)

	assert.Contains(t, string(result), "data:image/png;base64,")
	assert.NotContains(t, string(result), "]("+server.URL+")")
}

func TestFindImageReferences_URLImages(t *testing.T) {
	content := `![local](./image.png) and ![remote](https://example.com/image.jpg)`

	refs := findImageReferences(content)

	require.Len(t, refs, 2)

	// Check local image
	assert.Equal(t, "local", refs[0].AltText)
	assert.Equal(t, "./image.png", refs[0].ImagePath)

	// Check URL image
	assert.Equal(t, "remote", refs[1].AltText)
	assert.Equal(t, "https://example.com/image.jpg", refs[1].ImagePath)
}

func TestFindImageReferences_DataURLSkipped(t *testing.T) {
	content := `![data](data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==) and ![url](https://example.com/image.png)`

	refs := findImageReferences(content)

	// Only URL should be found, data URL should be skipped
	require.Len(t, refs, 1)
	assert.Equal(t, "url", refs[0].AltText)
	assert.Equal(t, "https://example.com/image.png", refs[0].ImagePath)
}

func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	return img
}
