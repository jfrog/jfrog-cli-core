package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileTree(t *testing.T) {
	copyMaxFilesInTree := maxFilesInTree
	defer func() {
		maxFilesInTree = copyMaxFilesInTree
	}()
	maxFilesInTree = 1

	fileTree := NewFileTree()
	// Add a new file and check String()
	fileTree.AddFile("repoName/path/to/first/artifact", "")
	result, excpected := fileTree.String(), "📦 repoName\n└── 📁 path\n    └── 📁 to\n        └── 📁 first\n            └── 📄 artifact\n"
	assert.Equal(t, excpected, result)

	// If maxFileInTree has exceeded, Check String() returns an empty string
	fileTree.AddFile("repoName/path/to/second/artifact", "")
	result, excpected = fileTree.String(), ""
	assert.Equal(t, excpected, result)
}

func TestFileTreeWithUrls(t *testing.T) {
	copyMaxFilesInTree := maxFilesInTree
	defer func() {
		maxFilesInTree = copyMaxFilesInTree
	}()
	maxFilesInTree = 1

	fileTree := NewFileTree()
	// Add a new file and check String()
	fileTree.AddFile("repoName/path/to/first/artifact", "http://myJFrogPlatform/customLink/first/artifact")
	result, excpected := fileTree.String(), "📦 repoName\n└── 📁 path\n    └── 📁 to\n        └── 📁 first\n            └── 📄 <a href=http://myJFrogPlatform/customLink/first/artifact target=\"_blank\">artifact</a>\n"
	assert.Equal(t, excpected, result)

}
