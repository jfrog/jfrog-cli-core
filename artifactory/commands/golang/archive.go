package golang

// The code in this file was copied from https://github.com/golang/go
// which is under this license https://github.com/golang/go/blob/master/LICENSE
// Copyright (c) 2009 The Go Authors. All rights reserved.

// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:

//    * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//    * Neither the name of Google Inc. nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.

// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
import (
	"github.com/jfrog/jfrog-client-go/artifactory/services/fspatterns"
	"github.com/jfrog/jfrog-client-go/utils"
	"golang.org/x/mod/module"
	gozip "golang.org/x/mod/zip"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// Package zip provides functions for creating and extracting module zip files.
//
// Module zip files have several restrictions listed below. These are necessary
// to ensure that module zip files can be extracted consistently on supported
// platforms and file systems.
//
// • No two file paths may be equal under Unicode case-folding (see
// strings.EqualFold).
//
// • A go.mod file may or may not appear in the top-level directory. If present,
// it must be named "go.mod", not any other case. Files named "go.mod"
// are not allowed in any other directory.
//
// • The total size in bytes of a module zip file may be at most MaxZipFile
// bytes (500 MiB). The total uncompressed size of the files within the
// zip may also be at most MaxZipFile bytes.
//
// • Each file's uncompressed size must match its declared 64-bit uncompressed
// size in the zip file header.
//
// • If the zip contains files named "<module>@<version>/go.mod" or
// "<module>@<version>/LICENSE", their sizes in bytes may be at most
// MaxGoMod or MaxLICENSE, respectively (both are 16 MiB).
//
// • Empty directories are ignored. File permissions and timestamps are also
// ignored.
//
// • Symbolic links and other irregular files are not allowed.
//
// Note that this package does not provide hashing functionality. See
// golang.org/x/mod/sumdb/dirhash.

// Archive project files according to the go project standard
func archiveProject(writer io.Writer, dir, mod, version string, excludedPatterns []string) error {
	m := module.Version{Version: version, Path: mod}
	excludedPatterns, err := getAbsolutePaths(excludedPatterns)
	if err != nil {
		return err
	}
	excludePatternsStr := fspatterns.PrepareExcludePathPattern(excludedPatterns, utils.GetPatternType(utils.PatternTypes{RegExp: false, Ant: false}), true)
	var files []gozip.File
	err = filepath.Walk(dir, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(dir, filePath)
		if err != nil {
			return err
		}
		slashPath := filepath.ToSlash(relPath)
		if info.IsDir() {
			if filePath == dir {
				// Don't skip the top-level directory.
				return nil
			}

			// Skip VCS directories.
			// fossil repos are regular files with arbitrary names, so we don't try
			// to exclude them.
			switch filepath.Base(filePath) {
			case ".bzr", ".git", ".hg", ".svn":
				return filepath.SkipDir
			}

			// Skip some subdirectories inside vendor, but maintain bug
			// golang.org/issue/31562, described in isVendoredPackage.
			// We would like Create and CreateFromDir to produce the same result
			// for a set of files, whether expressed as a directory tree or zip.

			if isVendoredPackage(slashPath) {
				return filepath.SkipDir
			}

			// Skip submodules (directories containing go.mod files).
			if goModInfo, err := os.Lstat(filepath.Join(filePath, "go.mod")); err == nil && !goModInfo.IsDir() {
				return filepath.SkipDir
			}
		}
		if info.Mode().IsRegular() {
			if !isVendoredPackage(slashPath) {
				excluded, err := isPathExcluded(filePath, excludePatternsStr)
				if err != nil {
					return err
				}
				if !excluded {
					files = append(files, dirFile{
						filePath:  filePath,
						slashPath: slashPath,
						info:      info,
					})
				}
			}
			return nil
		}
		// Not a regular file or a directory. Probably a symbolic link.
		// Irregular files are ignored, so skip it.
		return nil
	})
	if err != nil {
		return err
	}

	return gozip.Create(writer, m, files)
}

func getAbsolutePaths(exclusionPatterns []string) ([]string, error) {
	var absolutedPaths []string
	for _, singleExclusion := range exclusionPatterns {
		singleExclusion, err := filepath.Abs(singleExclusion)
		if err != nil {
			return nil, err
		}
		absolutedPaths = append(absolutedPaths, singleExclusion)
	}
	return absolutedPaths, nil
}

// This function receives a path and a regexp.
// It returns trUe is the path received matches the regexp.
// Before the match, thw path is turned into an absolute.
func isPathExcluded(path string, excludePatternsRegexp string) (excluded bool, err error) {
	var fullPath string
	if len(excludePatternsRegexp) > 0 {
		fullPath, err = filepath.Abs(path)
		if err != nil {
			return
		}
		excluded, err = regexp.MatchString(excludePatternsRegexp, fullPath)
	}
	return
}

func isVendoredPackage(name string) bool {
	var i int
	if strings.HasPrefix(name, "vendor/") {
		i += len("vendor/")
	} else if j := strings.Index(name, "/vendor/"); j >= 0 {
		// This offset looks incorrect; this should probably be
		//
		// 	i = j + len("/vendor/")
		//
		// Unfortunately, we can't fix it without invalidating checksums.
		// Fortunately, the error appears to be strictly conservative: we'll retain
		// vendored packages that we should have pruned, but we won't prune
		// non-vendored packages that we should have retained.
		//
		// Since this defect doesn't seem to break anything, it's not worth fixing
		// for now.
		i += len("/vendor/")
	} else {
		return false
	}
	return strings.Contains(name[i:], "/")
}

type dirFile struct {
	filePath, slashPath string
	info                os.FileInfo
}

func (f dirFile) Path() string                 { return f.slashPath }
func (f dirFile) Lstat() (os.FileInfo, error)  { return f.info, nil }
func (f dirFile) Open() (io.ReadCloser, error) { return os.Open(f.filePath) }

// File provides an abstraction for a file in a directory, zip, or anything
// else that looks like a file.
type File interface {
	// Path returns a clean slash-separated relative path from the module root
	// directory to the file.
	Path() string

	// Lstat returns information about the file. If the file is a symbolic link,
	// Lstat returns information about the link itself, not the file it points to.
	Lstat() (os.FileInfo, error)

	// Open provides access to the data within a regular file. Open may return
	// an error if called on a directory or symbolic link.
	Open() (io.ReadCloser, error)
}
