package image

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/jfrog/jfrog-client-go/utils/log"
)

const (
	pngFormat             = "png"
	gifFormat             = "gif"
	svgMimeType           = "image/svg+xml"
	pngMimeType           = "image/png"
	gifMimeType           = "image/gif"
	jpegMimeType          = "image/jpeg"
	widthAttr             = "width"
	heightAttr            = "height"
	defaultHTTPTimeout    = 30 * time.Second
	svgDoubleQuotePattern = `%s="([^"]+)"`
	svgSingleQuotePattern = `%s='([^']+)'`
	svgTagPattern         = `(?i)^\s*(?:<\?xml[^>]*>\s*)?(?:<!DOCTYPE[^>]*>\s*)?<svg\s`
)

var (
	widthDoubleQuoteRegex  = regexp.MustCompile(fmt.Sprintf(svgDoubleQuotePattern, widthAttr))
	widthSingleQuoteRegex  = regexp.MustCompile(fmt.Sprintf(svgSingleQuotePattern, widthAttr))
	heightDoubleQuoteRegex = regexp.MustCompile(fmt.Sprintf(svgDoubleQuotePattern, heightAttr))
	heightSingleQuoteRegex = regexp.MustCompile(fmt.Sprintf(svgSingleQuotePattern, heightAttr))
	svgTagRegex            = regexp.MustCompile(svgTagPattern)
)

func ToBase64(imagePath, baseDir string, width, height int) (string, string, error) {
	var content []byte
	var err error

	if isURL(imagePath) {
		content, err = downloadImageContent(imagePath)
		if err != nil {
			return "", "", fmt.Errorf("failed to download image: %v", err)
		}
	} else {
		resolvedPath := resolveImagePath(imagePath, baseDir)
		content, err = os.ReadFile(resolvedPath)
		if err != nil {
			return "", "", fmt.Errorf("failed to read image file: %v", err)
		}
	}

	if svgTagRegex.Match(content) {
		if err = validateSVGDimensions(content, width, height); err != nil {
			return "", "", err
		}
		return base64.StdEncoding.EncodeToString(content), svgMimeType, nil
	}

	img, format, err := image.Decode(bytes.NewReader(content))
	if err != nil {
		return "", "", fmt.Errorf("unsupported image format: %v", err)
	}

	if err = validateImageDimensions(img, width, height); err != nil {
		return "", "", err
	}

	var mimeType string
	switch format {
	case pngFormat:
		mimeType = pngMimeType
	case gifFormat:
		mimeType = gifMimeType
	default:
		mimeType = jpegMimeType
	}

	return base64.StdEncoding.EncodeToString(content), mimeType, nil
}

func downloadImageContent(imageURL string) ([]byte, error) {
	client := &http.Client{Timeout: defaultHTTPTimeout}
	resp, err := client.Get(imageURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Warn("failed to close response body: ", closeErr)
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status: %s", resp.Status)
	}
	return io.ReadAll(resp.Body)
}

func validateImageDimensions(img image.Image, maxWidth, maxHeight int) error {
	srcWidth := img.Bounds().Dx()
	srcHeight := img.Bounds().Dy()

	if maxWidth <= 0 && maxHeight <= 0 {
		return nil
	}

	if maxWidth > 0 && srcWidth > maxWidth {
		return fmt.Errorf("image width %d exceeds maximum allowed width %d", srcWidth, maxWidth)
	}
	if maxHeight > 0 && srcHeight > maxHeight {
		return fmt.Errorf("image height %d exceeds maximum allowed height %d", srcHeight, maxHeight)
	}

	return nil
}

func validateSVGDimensions(content []byte, maxWidth, maxHeight int) error {
	contentStr := string(content)

	if maxWidth <= 0 && maxHeight <= 0 {
		return nil
	}

	widthMatch := widthDoubleQuoteRegex.FindStringSubmatch(contentStr)
	if len(widthMatch) == 0 {
		widthMatch = widthSingleQuoteRegex.FindStringSubmatch(contentStr)
	}

	heightMatch := heightDoubleQuoteRegex.FindStringSubmatch(contentStr)
	if len(heightMatch) == 0 {
		heightMatch = heightSingleQuoteRegex.FindStringSubmatch(contentStr)
	}

	var currentWidth, currentHeight int
	var hasWidth, hasHeight bool

	if len(widthMatch) > 1 {
		if _, err := fmt.Sscanf(widthMatch[1], "%d", &currentWidth); err == nil {
			hasWidth = true
		}
	}
	if len(heightMatch) > 1 {
		if _, err := fmt.Sscanf(heightMatch[1], "%d", &currentHeight); err == nil {
			hasHeight = true
		}
	}

	if !hasWidth && !hasHeight {
		return nil
	}

	if hasWidth && maxWidth > 0 && currentWidth > maxWidth {
		return fmt.Errorf("SVG width %d exceeds maximum allowed width %d", currentWidth, maxWidth)
	}
	if hasHeight && maxHeight > 0 && currentHeight > maxHeight {
		return fmt.Errorf("SVG height %d exceeds maximum allowed height %d", currentHeight, maxHeight)
	}

	return nil
}

func resolveImagePath(imagePath, baseDir string) string {
	if filepath.IsAbs(imagePath) {
		return imagePath
	}

	if baseDir == "" {
		return imagePath
	}

	return filepath.Join(baseDir, imagePath)
}

func isURL(str string) bool {
	u, err := url.Parse(str)
	return err == nil && u.Scheme != "" && u.Host != ""
}
