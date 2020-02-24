// Copyright 2013 Unknown
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

// Package zip enables you to transparently read or write ZIP compressed archives and the files inside them.
package zip

import (
	"archive/zip"
	"errors"
	"io"
	"os"
	"path"
	"strings"

	"github.com/unknwon/cae"
)

// A File represents a file or directory entry in archive.
type File struct {
	*zip.FileHeader
	oldName    string // NOTE: unused, for future change name feature.
	oldComment string // NOTE: unused, for future change comment feature.
	absPath    string // Absolute path of local file system.
	tmpPath    string
}

// A ZipArchive represents a file archive, compressed with Zip.
type ZipArchive struct {
	*zip.ReadCloser
	FileName   string
	Comment    string
	NumFiles   int
	Flag       int
	Permission os.FileMode

	files        []*File
	isHasChanged bool

	// For supporting flushing to io.Writer.
	writer      io.Writer
	isHasWriter bool
}

// OpenFile is the generalized open call; most users will use Open
// instead. It opens the named zip file with specified flag
// (O_RDONLY etc.) if applicable. If successful,
// methods on the returned ZipArchive can be used for I/O.
// If there is an error, it will be of type *PathError.
func OpenFile(name string, flag int, perm os.FileMode) (*ZipArchive, error) {
	z := new(ZipArchive)
	err := z.Open(name, flag, perm)
	return z, err
}

// Create creates the named zip file, truncating
// it if it already exists. If successful, methods on the returned
// ZipArchive can be used for I/O; the associated file descriptor has mode
// O_RDWR.
// If there is an error, it will be of type *PathError.
func Create(name string) (*ZipArchive, error) {
	os.MkdirAll(path.Dir(name), os.ModePerm)
	return OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0666)
}

// Open opens the named zip file for reading. If successful, methods on
// the returned ZipArchive can be used for reading; the associated file
// descriptor has mode O_RDONLY.
// If there is an error, it will be of type *PathError.
func Open(name string) (*ZipArchive, error) {
	return OpenFile(name, os.O_RDONLY, 0)
}

// New accepts a variable that implemented interface io.Writer
// for write-only purpose operations.
func New(w io.Writer) *ZipArchive {
	return &ZipArchive{
		writer:      w,
		isHasWriter: true,
	}
}

// List returns a string slice of files' name in ZipArchive.
// Specify prefixes will be used as filters.
func (z *ZipArchive) List(prefixes ...string) []string {
	isHasPrefix := len(prefixes) > 0
	names := make([]string, 0, z.NumFiles)
	for _, f := range z.files {
		if isHasPrefix && !cae.HasPrefix(f.Name, prefixes) {
			continue
		}
		names = append(names, f.Name)
	}
	return names
}

// AddEmptyDir adds a raw directory entry to ZipArchive,
// it returns false if same directory enry already existed.
func (z *ZipArchive) AddEmptyDir(dirPath string) bool {
	dirPath = strings.Replace(dirPath, "\\", "/", -1)

	if !strings.HasSuffix(dirPath, "/") {
		dirPath += "/"
	}

	for _, f := range z.files {
		if dirPath == f.Name {
			return false
		}
	}

	dirPath = strings.TrimSuffix(dirPath, "/")
	if strings.Contains(dirPath, "/") {
		// Auto add all upper level directories.
		z.AddEmptyDir(path.Dir(dirPath))
	}
	z.files = append(z.files, &File{
		FileHeader: &zip.FileHeader{
			Name:             dirPath + "/",
			UncompressedSize: 0,
		},
	})
	z.updateStat()
	return true
}

// AddDir adds a directory and subdirectories entries to ZipArchive.
func (z *ZipArchive) AddDir(dirPath, absPath string) error {
	dir, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer dir.Close()

	// Make sure we have all upper level directories.
	z.AddEmptyDir(dirPath)

	fis, err := dir.Readdir(0)
	if err != nil {
		return err
	}
	for _, fi := range fis {
		curPath := absPath + "/" + fi.Name()
		tmpRecPath := path.Join(dirPath, fi.Name())
		if fi.IsDir() {
			if err = z.AddDir(tmpRecPath, curPath); err != nil {
				return err
			}
		} else {
			if err = z.AddFile(tmpRecPath, curPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// updateStat should be called after every change for rebuilding statistic.
func (z *ZipArchive) updateStat() {
	z.NumFiles = len(z.files)
	z.isHasChanged = true
}

// AddFile adds a file entry to ZipArchive.
func (z *ZipArchive) AddFile(fileName, absPath string) error {
	fileName = strings.Replace(fileName, "\\", "/", -1)
	absPath = strings.Replace(absPath, "\\", "/", -1)

	if cae.IsFilter(absPath) {
		return nil
	}

	f, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return err
	}

	file := new(File)
	file.FileHeader, err = zip.FileInfoHeader(fi)
	if err != nil {
		return err
	}
	file.Name = fileName
	file.absPath = absPath

	z.AddEmptyDir(path.Dir(fileName))

	isExist := false
	for _, f := range z.files {
		if fileName == f.Name {
			f = file
			isExist = true
			break
		}
	}
	if !isExist {
		z.files = append(z.files, file)
	}

	z.updateStat()
	return nil
}

// DeleteIndex deletes an entry in the archive by its index.
func (z *ZipArchive) DeleteIndex(idx int) error {
	if idx >= z.NumFiles {
		return errors.New("index out of range of number of files")
	}

	z.files = append(z.files[:idx], z.files[idx+1:]...)
	return nil
}

// DeleteName deletes an entry in the archive by its name.
func (z *ZipArchive) DeleteName(name string) error {
	for i, f := range z.files {
		if f.Name == name {
			return z.DeleteIndex(i)
		}
	}
	return errors.New("entry with given name not found")
}
