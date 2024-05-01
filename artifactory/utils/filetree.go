package utils

import (
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

func (ft *FileTree) AddFile(path string) {
	if ft.size >= maxFilesInTree {
		ft.exceedsMax = true
		return
	}
	splitPath := strings.Split(path, "/")
	if _, exist := ft.repos[splitPath[0]]; !exist {
		ft.repos[splitPath[0]] = &dirNode{name: splitPath[0], prefix: "📦 ", subDirNodes: map[string]*dirNode{}, fileNames: map[string]bool{}}
	}
	if ft.repos[splitPath[0]].addArtifact(splitPath[1:]) {
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
		treeStr += strings.Join(repo.strings(), "\n") + "\n"
	}
	return treeStr
}

type dirNode struct {
	name        string
	prefix      string
	subDirNodes map[string]*dirNode
	fileNames   map[string]bool
}

func (dn *dirNode) addArtifact(pathInDir []string) bool {
	if len(pathInDir) == 1 {
		if _, exist := dn.fileNames[pathInDir[0]]; exist {
			return false
		}
		dn.fileNames[pathInDir[0]] = true
	} else {
		if _, exist := dn.subDirNodes[pathInDir[0]]; !exist {
			dn.subDirNodes[pathInDir[0]] = &dirNode{name: pathInDir[0], prefix: "📁 ", subDirNodes: map[string]*dirNode{}, fileNames: map[string]bool{}}
		}
		return dn.subDirNodes[pathInDir[0]].addArtifact(pathInDir[1:])
	}
	return true
}

func (dn *dirNode) strings() []string {
	strs := []string{dn.prefix + dn.name}
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
		strs = append(strs, subDirPrefix+subDirStrs[0])
		for subDirStrIndex := 1; subDirStrIndex < len(subDirStrs); subDirStrIndex++ {
			strs = append(strs, innerStrPrefix+subDirStrs[subDirStrIndex])
		}
		subDirIndex++
	}
	fileIndex := 0
	for fileName := range dn.fileNames {
		var filePrefix string
		if fileIndex == len(dn.fileNames)-1 {
			filePrefix = "└── 📄 "
		} else {
			filePrefix = "├── 📄 "
			fileIndex++
		}
		strs = append(strs, filePrefix+fileName)
	}
	return strs
}
