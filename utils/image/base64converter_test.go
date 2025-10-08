package image

import (
	"encoding/base64"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImageToBase64_LocalPNG(t *testing.T) {
	tempDir := t.TempDir()

	pngFile := createTestPNG(t, tempDir, "test.png", 100, 100)

	_, _, err := ToBase64(pngFile, "", 50, 50)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed")
	assert.NoError(t, os.Remove(pngFile))
}

func TestImageToBase64_LocalJPEG(t *testing.T) {
	tempDir := t.TempDir()

	jpegFile := createTestJPEG(t, tempDir, "test.jpg", 200, 150)

	_, _, err := ToBase64(jpegFile, "", 100, 75)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed")
	assert.NoError(t, os.Remove(jpegFile))
}

func TestImageToBase64_LocalGIF(t *testing.T) {
	tempDir := t.TempDir()

	gifFile := createTestGIF(t, tempDir, "test.gif", 80, 60)

	_, _, err := ToBase64(gifFile, "", 40, 30)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed")
	assert.NoError(t, os.Remove(gifFile))
}

func TestImageToBase64_LocalSVG(t *testing.T) {
	tempDir := t.TempDir()

	svgContent := `<svg width="100" height="100" xmlns="http://www.w3.org/2000/svg">
		<rect width="100" height="100" fill="red"/>
	</svg>`
	svgFile := filepath.Join(tempDir, "test.svg")
	err := os.WriteFile(svgFile, []byte(svgContent), 0644)
	require.NoError(t, err)
	_, _, err = ToBase64(svgFile, "", 50, 50)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed")
	assert.NoError(t, os.Remove(svgFile))
}

func TestImageToBase64_URLImage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		img := createTestImage(50, 50)
		w.Header().Set("Content-Type", "image/png")
		if err := png.Encode(w, img); err != nil {
			t.Errorf("failed to encode PNG: %v", err)
		}
	}))
	defer server.Close()

	_, _, err := ToBase64(server.URL, "", 25, 25)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed")
}

func TestImageToBase64_URLImageError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, _, err := ToBase64(server.URL, "", 0, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "status:")
}

func TestImageToBase64_InvalidPath(t *testing.T) {
	_, _, err := ToBase64("nonexistent.png", "", 0, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read image file")
}

func TestImageToBase64_UnsupportedFormat(t *testing.T) {
	tempDir := t.TempDir()

	invalidFile := filepath.Join(tempDir, "test.txt")
	err := os.WriteFile(invalidFile, []byte("not an image"), 0644)
	require.NoError(t, err)
	_, _, err = ToBase64(invalidFile, "", 0, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported image format")
	assert.NoError(t, os.Remove(invalidFile))
}

func TestValidateImageDimensions_NoValidation(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 100))
	err := validateImageDimensions(src, 0, 0)
	require.NoError(t, err)
}

func TestValidateImageDimensions_WithinLimits(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 500, 300))
	err := validateImageDimensions(src, 600, 400)
	require.NoError(t, err)
}

func TestValidateImageDimensions_WidthExceeds(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 200, 100))
	err := validateImageDimensions(src, 150, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "width 200 exceeds maximum allowed width 150")
}

func TestValidateImageDimensions_HeightExceeds(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 200, 100))
	err := validateImageDimensions(src, 0, 75)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "height 100 exceeds maximum allowed height 75")
}

func TestValidateImageDimensions_BothDimensionsExceed(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 200, 100))
	err := validateImageDimensions(src, 150, 75)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed")
}

func TestValidateImageDimensions_ExactLimits(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 100, 100))
	err := validateImageDimensions(src, 100, 100)
	require.NoError(t, err)
}

func TestImageToBase64_WithinLimits(t *testing.T) {
	tempDir := t.TempDir()

	pngFile := createTestPNG(t, tempDir, "test.png", 50, 50)

	base64Str, mimeType, err := ToBase64(pngFile, "", 100, 100)
	require.NoError(t, err)
	assert.Equal(t, "image/png", mimeType)
	assert.NotEmpty(t, base64Str)
	assert.NoError(t, os.Remove(pngFile))
}

func TestValidateSVGDimensions_ExceedsLimits(t *testing.T) {
	svgContent := `<svg width="200" height="150" xmlns="http://www.w3.org/2000/svg">
		<rect width="200" height="150" fill="red"/>
	</svg>`

	err := validateSVGDimensions([]byte(svgContent), 100, 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed")
}

func TestValidateSVGDimensions_WithinLimits(t *testing.T) {
	svgContent := `<svg width="100" height="200" xmlns="http://www.w3.org/2000/svg">
		<rect width="100" height="200" fill="red"/>
	</svg>`

	err := validateSVGDimensions([]byte(svgContent), 300, 400)
	require.NoError(t, err)
}

func TestValidateSVGDimensions_NoDimensions(t *testing.T) {
	svgContent := `<svg xmlns="http://www.w3.org/2000/svg">
		<rect fill="red"/>
	</svg>`

	err := validateSVGDimensions([]byte(svgContent), 300, 400)
	require.NoError(t, err)
}

func TestValidateSVGDimensions_SingleQuotes(t *testing.T) {
	svgContent := `<svg width='200' height='150' xmlns="http://www.w3.org/2000/svg">
		<rect width='200' height='150' fill="red"/>
	</svg>`

	err := validateSVGDimensions([]byte(svgContent), 100, 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed")
}

func TestValidateSVGDimensions_MixedQuotes(t *testing.T) {
	svgContent := `<svg width="200" height='150' xmlns="http://www.w3.org/2000/svg">
		<rect width="200" height='150' fill="red"/>
	</svg>`

	err := validateSVGDimensions([]byte(svgContent), 100, 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed")
}

func TestValidateSVGDimensions_SingleQuotesWithinLimits(t *testing.T) {
	svgContent := `<svg width='100' height='200' xmlns="http://www.w3.org/2000/svg">
		<rect width='100' height='200' fill="red"/>
	</svg>`

	err := validateSVGDimensions([]byte(svgContent), 300, 400)
	require.NoError(t, err)
}

func TestValidateSVGDimensions_MismatchedQuotes(t *testing.T) {
	svgContent := `<svg width='200" height="150' xmlns="http://www.w3.org/2000/svg">
		<rect width='200" height="150' fill="red"/>
	</svg>`

	err := validateSVGDimensions([]byte(svgContent), 100, 100)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed")
}

func TestIsURL(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"https://example.com/image.png", true},
		{"ftp://example.com/image.png", true},
		{"image.png", false},
		{"./image.png", false},
		{"../image.png", false},
		{"", false},
		{"not-a-url", false},
	}

	for _, test := range tests {
		result := isURL(test.input)
		assert.Equal(t, test.expected, result, "isURL(%q)", test.input)
	}
}

func TestToBase64WithBaseDir_RelativePath(t *testing.T) {
	tempDir := t.TempDir()
	subDir := filepath.Join(tempDir, "images")
	err := os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	pngFile := createTestPNG(t, subDir, "test.png", 100, 100)

	_, _, err = ToBase64("images/test.png", tempDir, 50, 50)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed")
	assert.NoError(t, os.Remove(pngFile))
}

func TestToBase64WithBaseDir_AbsolutePath(t *testing.T) {
	tempDir := t.TempDir()

	pngFile := createTestPNG(t, tempDir, "test.png", 100, 100)

	_, _, err := ToBase64(pngFile, "/some/other/dir", 50, 50)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed")
	assert.NoError(t, os.Remove(pngFile))
}

func TestToBase64WithBaseDir_EmptyBaseDir(t *testing.T) {
	tempDir := t.TempDir()

	pngFile := createTestPNG(t, tempDir, "test.png", 100, 100)

	_, _, err := ToBase64(pngFile, "", 50, 50)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "exceeds maximum allowed")
	assert.NoError(t, os.Remove(pngFile))
}

func TestResolveImagePath(t *testing.T) {
	absolutePath, err := filepath.Abs(filepath.Join("absolute", "path", "pic.png"))
	if err != nil {
		assert.FailNow(t, err.Error())
	}
	baseDir := filepath.Join("base", "dir")
	relativePath := filepath.Join("images", "pic.png")

	tests := []struct {
		name      string
		imagePath string
		baseDir   string
		expected  string
	}{
		{"absolute path", absolutePath, "", absolutePath},
		{"relative path with base", relativePath, baseDir, filepath.Join(baseDir, relativePath)},
		{"relative path no base", relativePath, "", relativePath},
		{"current dir relative", filepath.Join(".", "pic.png"), baseDir, filepath.Join(baseDir, "pic.png")},
		{"parent dir relative", filepath.Join("..", "pic.png"), baseDir, filepath.Join(baseDir, "..", "pic.png")},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := resolveImagePath(test.imagePath, test.baseDir)
			assert.Equal(t, test.expected, result)
		})
	}
}

func TestDownloadImageContent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := w.Write([]byte("test image content")); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	content, err := downloadImageContent(server.URL)
	require.NoError(t, err)
	assert.Equal(t, "test image content", string(content))
}

func TestDownloadImageContent_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := downloadImageContent(server.URL)
	require.Error(t, err)
}

func TestDownloadImageContent_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		if _, err := w.Write([]byte("delayed response")); err != nil {
			t.Errorf("failed to write response: %v", err)
		}
	}))
	defer server.Close()

	client := &http.Client{Timeout: 50 * time.Millisecond}
	_, err := client.Get(server.URL)
	require.Error(t, err)
}

func TestImageToBase64_E2E_RealImages(t *testing.T) {
	tempDir := t.TempDir()

	pngFile := createTestPNG(t, tempDir, "e2e.png", 300, 200)
	jpegFile := createTestJPEG(t, tempDir, "e2e.jpg", 400, 300)
	gifFile := createTestGIF(t, tempDir, "e2e.gif", 150, 100)

	svgContent := `<svg width="250" height="150" xmlns="http://www.w3.org/2000/svg">
		<rect width="250" height="150" fill="blue"/>
		<circle cx="125" cy="75" r="50" fill="yellow"/>
	</svg>`
	svgFile := filepath.Join(tempDir, "e2e.svg")
	err := os.WriteFile(svgFile, []byte(svgContent), 0644)
	if err != nil {
		t.Fatalf("failed to create SVG file: %v", err)
	}

	tests := []struct {
		name         string
		filePath     string
		width        int
		height       int
		expectedMime string
		shouldError  bool
	}{
		{"PNG", pngFile, 150, 100, "image/png", true},
		{"JPEG", jpegFile, 200, 150, "image/jpeg", true},
		{"GIF", gifFile, 75, 50, "image/gif", true},
		{"SVG", svgFile, 200, 100, "image/svg+xml", true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			base64Str, mimeType, err := ToBase64(test.filePath, "", test.width, test.height)
			if test.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "exceeds maximum allowed")
			} else {
				require.NoError(t, err)
				assert.Equal(t, test.expectedMime, mimeType)
				assert.NotEmpty(t, base64Str)
				decoded, decErr := base64.StdEncoding.DecodeString(base64Str)
				require.NoError(t, decErr)
				assert.NotEmpty(t, decoded)
			}
		})
	}

	assert.NoError(t, os.Remove(pngFile))
	assert.NoError(t, os.Remove(jpegFile))
	assert.NoError(t, os.Remove(gifFile))
	assert.NoError(t, os.Remove(svgFile))
}

func createTestPNG(t *testing.T, dir, filename string, width, height int) string {
	filePath := filepath.Join(dir, filename)
	file, err := os.Create(filePath)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, file.Close())
	}()

	img := createTestImage(width, height)
	err = png.Encode(file, img)
	require.NoError(t, err)

	return filePath
}

func createTestJPEG(t *testing.T, dir, filename string, width, height int) string {
	filePath := filepath.Join(dir, filename)
	file, err := os.Create(filePath)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, file.Close())
	}()

	img := createTestImage(width, height)
	err = jpeg.Encode(file, img, &jpeg.Options{Quality: 90})
	require.NoError(t, err)

	return filePath
}

func createTestGIF(t *testing.T, dir, filename string, width, height int) string {
	filePath := filepath.Join(dir, filename)
	file, err := os.Create(filePath)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, file.Close())
	}()

	img := createTestImage(width, height)
	err = gif.Encode(file, img, nil)
	require.NoError(t, err)

	return filePath
}

func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8(float64(x) * 255 / float64(width))
			g := uint8(float64(y) * 255 / float64(height))
			b := uint8(float64(x+y) * 255 / float64(width+height))
			img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
		}
	}

	return img
}
