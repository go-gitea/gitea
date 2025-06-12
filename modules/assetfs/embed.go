// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package assetfs

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/util"
)

type EmbeddedFile interface {
	io.ReadSeeker
	fs.ReadDirFile
	ReadDir(n int) ([]fs.DirEntry, error)
}

type EmbeddedFileInfo interface {
	fs.FileInfo
	fs.DirEntry
	GetGzipContent() ([]byte, bool)
}

type decompressor interface {
	io.Reader
	Close() error
	Reset(io.Reader) error
}

type embeddedFileInfo struct {
	fs       *embeddedFS
	fullName string
	data     []byte

	BaseName   string              `json:"n"`
	OriginSize int64               `json:"s,omitempty"`
	DataBegin  int64               `json:"b,omitempty"`
	DataLen    int64               `json:"l,omitempty"`
	Children   []*embeddedFileInfo `json:"c,omitempty"`
}

func (fi *embeddedFileInfo) GetGzipContent() ([]byte, bool) {
	// when generating the bindata, if the compressed data equals or is larger than the original data, we store the original data
	if fi.DataLen == fi.OriginSize {
		return nil, false
	}
	return fi.data, true
}

type EmbeddedFileBase struct {
	info       *embeddedFileInfo
	dataReader io.ReadSeeker
	seekPos    int64
}

func (f *EmbeddedFileBase) ReadDir(n int) ([]fs.DirEntry, error) {
	// this method is used to satisfy the "func (f ioFile) ReadDir(...)" in httpfs
	l, err := f.info.fs.ReadDir(f.info.fullName)
	if err != nil {
		return nil, err
	}
	if n < 0 || n > len(l) {
		return l, nil
	}
	return l[:n], nil
}

type EmbeddedOriginFile struct {
	EmbeddedFileBase
}

type EmbeddedCompressedFile struct {
	EmbeddedFileBase
	decompressor    decompressor
	decompressorPos int64
}

type embeddedFS struct {
	meta func() *EmbeddedMeta

	files   map[string]*embeddedFileInfo
	filesMu sync.RWMutex

	data []byte
}

type EmbeddedMeta struct {
	Root *embeddedFileInfo
}

func NewEmbeddedFS(data []byte) fs.ReadDirFS {
	efs := &embeddedFS{data: data, files: make(map[string]*embeddedFileInfo)}
	efs.meta = sync.OnceValue(func() *EmbeddedMeta {
		var meta EmbeddedMeta
		p := bytes.LastIndexByte(data, '\n')
		if p < 0 {
			return &meta
		}
		if err := json.Unmarshal(data[p+1:], &meta); err != nil {
			panic("embedded file is not valid")
		}
		return &meta
	})
	return efs
}

var _ fs.ReadDirFS = (*embeddedFS)(nil)

func (e *embeddedFS) ReadDir(name string) (l []fs.DirEntry, err error) {
	fi, err := e.getFileInfo(name)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		return nil, fs.ErrNotExist
	}
	l = make([]fs.DirEntry, len(fi.Children))
	for i, child := range fi.Children {
		l[i], err = e.getFileInfo(name + "/" + child.BaseName)
		if err != nil {
			return nil, err
		}
	}
	return l, nil
}

func (e *embeddedFS) getFileInfo(fullName string) (*embeddedFileInfo, error) {
	// no need to do heavy "path.Clean()" because we don't want to support "foo/../bar" or absolute paths
	fullName = strings.TrimPrefix(fullName, "./")
	if fullName == "" {
		fullName = "."
	}

	e.filesMu.RLock()
	fi := e.files[fullName]
	e.filesMu.RUnlock()
	if fi != nil {
		return fi, nil
	}

	fields := strings.Split(fullName, "/")
	fi = e.meta().Root
	if fullName != "." {
		found := true
		for _, field := range fields {
			for _, child := range fi.Children {
				if found = child.BaseName == field; found {
					fi = child
					break
				}
			}
			if !found {
				return nil, fs.ErrNotExist
			}
		}
	}

	e.filesMu.Lock()
	defer e.filesMu.Unlock()
	if fi != nil {
		fi.fs = e
		fi.fullName = fullName
		fi.data = e.data[fi.DataBegin : fi.DataBegin+fi.DataLen]
		e.files[fullName] = fi // do not cache nil, otherwise keeping accessing random non-existing file will cause OOM
		return fi, nil
	}
	return nil, fs.ErrNotExist
}

func (e *embeddedFS) Open(name string) (fs.File, error) {
	info, err := e.getFileInfo(name)
	if err != nil {
		return nil, err
	}
	base := EmbeddedFileBase{info: info}
	base.dataReader = bytes.NewReader(base.info.data)
	if info.DataLen != info.OriginSize {
		decomp, err := gzip.NewReader(base.dataReader)
		if err != nil {
			return nil, err
		}
		return &EmbeddedCompressedFile{EmbeddedFileBase: base, decompressor: decomp}, nil
	}
	return &EmbeddedOriginFile{base}, nil
}

var (
	_ EmbeddedFileInfo = (*embeddedFileInfo)(nil)
	_ EmbeddedFile     = (*EmbeddedOriginFile)(nil)
	_ EmbeddedFile     = (*EmbeddedCompressedFile)(nil)
)

func (f *EmbeddedOriginFile) Read(p []byte) (n int, err error) {
	return f.dataReader.Read(p)
}

func (f *EmbeddedCompressedFile) Read(p []byte) (n int, err error) {
	if f.decompressorPos > f.seekPos {
		if err = f.decompressor.Reset(bytes.NewReader(f.info.data)); err != nil {
			return 0, err
		}
		f.decompressorPos = 0
	}
	if f.decompressorPos < f.seekPos {
		if _, err = io.CopyN(io.Discard, f.decompressor, f.seekPos-f.decompressorPos); err != nil {
			return 0, err
		}
		f.decompressorPos = f.seekPos
	}
	n, err = f.decompressor.Read(p)
	f.decompressorPos += int64(n)
	f.seekPos = f.decompressorPos
	return n, err
}

func (f *EmbeddedFileBase) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		f.seekPos = offset
	case io.SeekCurrent:
		f.seekPos += offset
	case io.SeekEnd:
		f.seekPos = f.info.OriginSize + offset
	}
	return f.seekPos, nil
}

func (f *EmbeddedFileBase) Stat() (fs.FileInfo, error) {
	return f.info, nil
}

func (f *EmbeddedOriginFile) Close() error {
	return nil
}

func (f *EmbeddedCompressedFile) Close() error {
	return f.decompressor.Close()
}

func (fi *embeddedFileInfo) Name() string {
	return fi.BaseName
}

func (fi *embeddedFileInfo) Size() int64 {
	return fi.OriginSize
}

func (fi *embeddedFileInfo) Mode() fs.FileMode {
	return util.Iif(fi.IsDir(), fs.ModeDir|0o555, 0o444)
}

func (fi *embeddedFileInfo) ModTime() time.Time {
	return getExecutableModTime()
}

func (fi *embeddedFileInfo) IsDir() bool {
	return fi.Children != nil
}

func (fi *embeddedFileInfo) Sys() any {
	return nil
}

func (fi *embeddedFileInfo) Type() fs.FileMode {
	return util.Iif(fi.IsDir(), fs.ModeDir, 0)
}

func (fi *embeddedFileInfo) Info() (fs.FileInfo, error) {
	return fi, nil
}

// getExecutableModTime returns the modification time of the executable file.
// In bindata, we can't use the ModTime of the files because we need to make the build reproducible
var getExecutableModTime = sync.OnceValue(func() (modTime time.Time) {
	exePath, err := os.Executable()
	if err != nil {
		return modTime
	}
	exePath, err = filepath.Abs(exePath)
	if err != nil {
		return modTime
	}
	exePath, err = filepath.EvalSymlinks(exePath)
	if err != nil {
		return modTime
	}
	st, err := os.Stat(exePath)
	if err != nil {
		return modTime
	}
	return st.ModTime()
})

func GenerateEmbedBindata(fsRootPath, outputFile string) error {
	output, err := os.OpenFile(outputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}
	defer output.Close()

	meta := &EmbeddedMeta{}
	meta.Root = &embeddedFileInfo{}
	var outputOffset int64
	var embedFiles func(parent *embeddedFileInfo, fsPath, embedPath string) error
	embedFiles = func(parent *embeddedFileInfo, fsPath, embedPath string) error {
		dirEntries, err := os.ReadDir(fsPath)
		if err != nil {
			return err
		}
		for _, dirEntry := range dirEntries {
			if err != nil {
				return err
			}
			if dirEntry.IsDir() {
				child := &embeddedFileInfo{
					BaseName: dirEntry.Name(),
					Children: []*embeddedFileInfo{}, // non-nil means it's a directory
				}
				parent.Children = append(parent.Children, child)
				if err = embedFiles(child, filepath.Join(fsPath, dirEntry.Name()), path.Join(embedPath, dirEntry.Name())); err != nil {
					return err
				}
			} else {
				data, err := os.ReadFile(filepath.Join(fsPath, dirEntry.Name()))
				if err != nil {
					return err
				}
				var compressed bytes.Buffer
				gz, _ := gzip.NewWriterLevel(&compressed, gzip.BestCompression)
				if _, err = gz.Write(data); err != nil {
					return err
				}
				if err = gz.Close(); err != nil {
					return err
				}

				// only use the compressed data if it is smaller than the original data
				outputBytes := util.Iif(len(compressed.Bytes()) < len(data), compressed.Bytes(), data)
				child := &embeddedFileInfo{
					BaseName:   dirEntry.Name(),
					OriginSize: int64(len(data)),
					DataBegin:  outputOffset,
					DataLen:    int64(len(outputBytes)),
				}
				if _, err = output.Write(outputBytes); err != nil {
					return err
				}
				outputOffset += child.DataLen
				parent.Children = append(parent.Children, child)
			}
		}
		return nil
	}

	if err = embedFiles(meta.Root, fsRootPath, ""); err != nil {
		return err
	}
	jsonBuf, err := json.Marshal(meta) // can't use json.NewEncoder here because it writes extra EOL
	if err != nil {
		return err
	}
	_, _ = output.Write([]byte{'\n'})
	_, err = output.Write(jsonBuf)
	return err
}
