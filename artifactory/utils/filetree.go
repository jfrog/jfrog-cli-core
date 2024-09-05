package utils

import (
	"fmt"
	"github.com/jfrog/jfrog-client-go/utils/log"
	"golang.org/x/exp/maps"
	"sort"
	"strings"
)

var maxFilesInTree = 200

// FileTree is a UI components that displays a file-system tree view in the terminal.
type FileTree struct {
	repos      map[string]*dirNode
	size       int
	exceedsMax bool
}

func NewFileTree() *FileTree {
	return &FileTree{repos: map[string]*dirNode{}, size: 0}
}

// Path - file structure path to artifact
// UploadedFileUrl - URL to the uploaded file in Artifactory,
// if UploadedFileUrl not provided, the file name will be displayed without a link.
func (ft *FileTree) AddFile(path, uploadedFileUrl string) {
	if ft.size >= maxFilesInTree {
		log.Info("Exceeded maximum number of files in tree")
		ft.exceedsMax = true
		return
	}
	splitPath := strings.Split(path, "/")
	if _, exist := ft.repos[splitPath[0]]; !exist {
		ft.repos[splitPath[0]] = &dirNode{name: splitPath[0], prefix: "📦 ", subDirNodes: map[string]*dirNode{}, fileNames: map[string]string{}}
	}
	if ft.repos[splitPath[0]].addArtifact(splitPath[1:], uploadedFileUrl) {
		ft.size++
	}
}

// Returns a string representation of the tree. If the number of files exceeded the maximum, an empty string will be returned.
func (ft *FileTree) String() string {
	if ft.exceedsMax {
		return ""
	}
	treeStr := ""
	for _, repo := range ft.repos {
		treeStr += strings.Join(repo.strings(), "\n") + "\n\n"
	}
	return treeStr
}

type dirNode struct {
	name        string
	prefix      string
	subDirNodes map[string]*dirNode
	fileNames   map[string]string
}

func (dn *dirNode) addArtifact(pathInDir []string, artifactUrl string) bool {
	if len(pathInDir) == 1 {
		if _, exist := dn.fileNames[pathInDir[0]]; exist {
			return false
		}
		dn.fileNames[pathInDir[0]] = artifactUrl
	} else {
		if _, exist := dn.subDirNodes[pathInDir[0]]; !exist {
			dn.subDirNodes[pathInDir[0]] = &dirNode{name: pathInDir[0], prefix: "📁 ", subDirNodes: map[string]*dirNode{}, fileNames: map[string]string{}}
		}
		return dn.subDirNodes[pathInDir[0]].addArtifact(pathInDir[1:], artifactUrl)
	}
	return true
}

func (dn *dirNode) strings() []string {
	repoAsString := []string{dn.prefix + dn.name}
	subDirIndex := 0
	for subDirName := range dn.subDirNodes {
		var subDirPrefix string
		var innerStrPrefix string
		if subDirIndex == len(dn.subDirNodes)-1 && len(dn.fileNames) == 0 {
			subDirPrefix = "└── "
			innerStrPrefix = "    "
		} else {
			subDirPrefix = "├── "
			innerStrPrefix = "│   "
		}
		subDirStrs := dn.subDirNodes[subDirName].strings()
		repoAsString = append(repoAsString, subDirPrefix+subDirStrs[0])
		for subDirStrIndex := 1; subDirStrIndex < len(subDirStrs); subDirStrIndex++ {
			repoAsString = append(repoAsString, innerStrPrefix+subDirStrs[subDirStrIndex])
		}
		subDirIndex++
	}
	fileIndex := 0

	// Sort File names inside each sub dir
	sortedFileNames := maps.Keys(dn.fileNames)
	sort.Slice(sortedFileNames, func(i, j int) bool { return sortedFileNames[i] < sortedFileNames[j] })

	for _, fileNameKey := range sortedFileNames {
		var filePrefix string
		if fileIndex == len(dn.fileNames)-1 {
			filePrefix = "└── "
		} else {
			filePrefix = "├── "
			fileIndex++
		}

		var fullFileName string
		if dn.fileNames[fileNameKey] != "" {
			fullFileName = fmt.Sprintf("%s<a href='%s' target=\"_blank\">%s</a>", filePrefix, dn.fileNames[fileNameKey], fileNameKey)
		} else {
			fullFileName = filePrefix + "📄 " + fileNameKey
		}
		repoAsString = append(repoAsString, fullFileName)
	}
	return repoAsString
}
