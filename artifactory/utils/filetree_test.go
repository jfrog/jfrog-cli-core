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

func TestFileTreeSort(t *testing.T) {
	testCases := []struct {
		name     string
		files    []string
		expected string
	}{
		{
			name: "Test Case 1",
			files: []string{
				"repoName/path/to/fileC",
				"repoName/path/to/fileA",
				"repoName/path/to/fileB",
			},
			expected: "📦 repoName\n└── 📁 path\n    └── 📁 to\n        ├── 📄 fileA\n        ├── 📄 fileB\n        └── 📄 fileC\n",
		},
		{
			name: "Test Case 2",
			files: []string{
				"repoName/path/to/file3",
				"repoName/path/to/file1",
				"repoName/path/to/file2",
			},
			expected: "📦 repoName\n└── 📁 path\n    └── 📁 to\n        ├── 📄 file1\n        ├── 📄 file2\n        └── 📄 file3\n",
		},
		{
			name: "Test Case 3",
			files: []string{
				"repoName/path/to/fileZ",
				"repoName/path/to/fileX",
				"repoName/path/to/fileY",
			},
			expected: "📦 repoName\n└── 📁 path\n    └── 📁 to\n        ├── 📄 fileX\n        ├── 📄 fileY\n        └── 📄 fileZ\n",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fileTree := NewFileTree()

			// Add files
			for _, file := range tc.files {
				fileTree.AddFile(file, "")
			}

			// Get the string representation of the FileTree
			result := fileTree.String()

			assert.Equal(t, tc.expected, result)
		})
	}
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
