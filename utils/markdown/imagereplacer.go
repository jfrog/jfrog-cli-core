package markdown

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jfrog/jfrog-cli-core/v2/utils/image"
	"github.com/jfrog/jfrog-client-go/utils/log"
)

type ImageReference struct {
	FullMatch string
	AltText   string
	ImagePath string
	StartPos  int
	EndPos    int
}

func EmbedMarkdownImages(markdown []byte, baseDir string, widthLimit, heightLimit int) ([]byte, error) {
	content := string(markdown)
	imageRefs := findImageReferences(content)

	if len(imageRefs) == 0 {
		return markdown, nil
	}

	sort.Slice(imageRefs, func(i, j int) bool {
		return imageRefs[i].StartPos < imageRefs[j].StartPos
	})

	var buffer bytes.Buffer
	lastIndex := 0
	hasReplacements := false

	for _, imgRef := range imageRefs {
		buffer.WriteString(content[lastIndex:imgRef.StartPos])

		base64Data, mimeType, err1 := image.ToBase64(imgRef.ImagePath, baseDir, widthLimit, heightLimit)
		if err1 != nil {
			log.Warn(fmt.Sprintf("Could not convert image %s to base64: %v. Keeping original reference.", imgRef.ImagePath, err1))
			buffer.WriteString(imgRef.FullMatch)
		} else {
			newImageRef := fmt.Sprintf("![%s](data:%s;base64,%s)", imgRef.AltText, mimeType, base64Data)
			buffer.WriteString(newImageRef)
			hasReplacements = true
		}
		lastIndex = imgRef.EndPos
	}

	if !hasReplacements {
		return markdown, nil
	}
	buffer.WriteString(content[lastIndex:])
	return buffer.Bytes(), nil
}

var imageStartRegex = regexp.MustCompile(`!\[([^]]*)]\(`)

func findImageReferences(content string) []ImageReference {
	var refs []ImageReference

	matches := imageStartRegex.FindAllStringSubmatchIndex(content, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}

		altText := content[match[2]:match[3]]
		pathStart := match[1]

		pathEnd := findMatchingParen(content, pathStart-1)
		if pathEnd == -1 {
			continue
		}

		imagePath := content[pathStart:pathEnd]

		if strings.HasPrefix(imagePath, "data:") {
			continue
		}

		fullMatch := content[match[0] : pathEnd+1]

		refs = append(refs, ImageReference{
			FullMatch: fullMatch,
			AltText:   altText,
			ImagePath: imagePath,
			StartPos:  match[0],
			EndPos:    pathEnd + 1,
		})
	}

	return refs
}

func findMatchingParen(content string, openPos int) int {
	parenCount := 1
	for i := openPos + 1; i < len(content); i++ {
		switch content[i] {
		case '(':
			parenCount++
		case ')':
			parenCount--
			if parenCount == 0 {
				return i
			}
		}
	}
	return -1
}
