// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dump

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/mholt/archives"
)

var SupportedOutputTypes = []string{"zip", "tar", "tar.sz", "tar.gz", "tar.xz", "tar.bz2", "tar.br", "tar.lz4", "tar.zst"}

// PrepareFileNameAndType prepares the output file name and type, if the type is not supported, it returns an empty "outType"
func PrepareFileNameAndType(argFile, argType string) (outFileName, outType string) {
	if argFile == "" && argType == "" {
		outType = SupportedOutputTypes[0]
		outFileName = fmt.Sprintf("gitea-dump-%d.%s", timeutil.TimeStampNow(), outType)
	} else if argFile == "" {
		outType = argType
		outFileName = fmt.Sprintf("gitea-dump-%d.%s", timeutil.TimeStampNow(), outType)
	} else if argType == "" {
		if filepath.Ext(outFileName) == "" {
			outType = SupportedOutputTypes[0]
			outFileName = argFile
		} else {
			for _, t := range SupportedOutputTypes {
				if strings.HasSuffix(argFile, "."+t) {
					outFileName = argFile
					outType = t
				}
			}
		}
	} else {
		outFileName, outType = argFile, argType
	}
	if !slices.Contains(SupportedOutputTypes, outType) {
		return "", ""
	}
	return outFileName, outType
}

func IsSubdir(upper, lower string) (bool, error) {
	if relPath, err := filepath.Rel(upper, lower); err != nil {
		return false, err
	} else if relPath == "." || !strings.HasPrefix(relPath, ".") {
		return true, nil
	}
	return false, nil
}

type Dumper struct {
	format  string
	output  io.Writer
	jobs    chan archives.ArchiveAsyncJob
	done    chan error
	Verbose bool

	globalExcludeAbsPaths []string
}

func NewDumper(format string, output io.Writer) *Dumper {
	d := &Dumper{
		format: format,
		output: output,
		jobs:   make(chan archives.ArchiveAsyncJob, 100),
		done:   make(chan error, 1),
	}
	d.startArchiver()
	return d
}

func (dumper *Dumper) startArchiver() {
	go func() {
		ctx := context.Background()
		var err error

		switch dumper.format {
		case "zip":
			err = archives.Zip{}.ArchiveAsync(ctx, dumper.output, dumper.jobs)
		case "tar":
			err = archives.Tar{}.ArchiveAsync(ctx, dumper.output, dumper.jobs)
		case "tar.gz":
			comp := archives.CompressedArchive{
				Compression: archives.Gz{},
				Archival:    archives.Tar{},
			}
			err = comp.ArchiveAsync(ctx, dumper.output, dumper.jobs)
		case "tar.xz":
			comp := archives.CompressedArchive{
				Compression: archives.Xz{},
				Archival:    archives.Tar{},
			}
			err = comp.ArchiveAsync(ctx, dumper.output, dumper.jobs)
		case "tar.bz2":
			comp := archives.CompressedArchive{
				Compression: archives.Bz2{},
				Archival:    archives.Tar{},
			}
			err = comp.ArchiveAsync(ctx, dumper.output, dumper.jobs)
		case "tar.br":
			comp := archives.CompressedArchive{
				Compression: archives.Brotli{},
				Archival:    archives.Tar{},
			}
			err = comp.ArchiveAsync(ctx, dumper.output, dumper.jobs)
		case "tar.lz4":
			comp := archives.CompressedArchive{
				Compression: archives.Lz4{},
				Archival:    archives.Tar{},
			}
			err = comp.ArchiveAsync(ctx, dumper.output, dumper.jobs)
		case "tar.zst":
			comp := archives.CompressedArchive{
				Compression: archives.Zstd{},
				Archival:    archives.Tar{},
			}
			err = comp.ArchiveAsync(ctx, dumper.output, dumper.jobs)
		default:
			err = fmt.Errorf("unsupported format: %s", dumper.format)
		}

		dumper.done <- err
	}()
}

// AddFilePath adds a file by its filesystem path
func (dumper *Dumper) AddFilePath(filePath, absPath string) error {
	if dumper.Verbose {
		log.Info("Adding file path %s", filePath)
	}

	fileInfo, err := os.Stat(absPath)
	if err != nil {
		return err
	}

	var archiveFileInfo archives.FileInfo
	if fileInfo.IsDir() {
		archiveFileInfo = archives.FileInfo{
			FileInfo:      fileInfo,
			NameInArchive: filePath,
			Open: func() (fs.File, error) {
				return &emptyDirFile{info: fileInfo}, nil
			},
		}
	} else {
		archiveFileInfo = archives.FileInfo{
			FileInfo:      fileInfo,
			NameInArchive: filePath,
			Open: func() (fs.File, error) {
				return os.Open(absPath)
			},
		}
	}

	resultChan := make(chan error, 1)
	job := archives.ArchiveAsyncJob{
		File:   archiveFileInfo,
		Result: resultChan,
	}

	select {
	case dumper.jobs <- job:
		return <-resultChan
	case err := <-dumper.done:
		return err
	}
}

// AddReader adds a file's contents from a Reader, this uses a pipe to stream files from object store to prevent them from filling up disk
func (dumper *Dumper) AddReader(r io.ReadCloser, info os.FileInfo, customName string) error {
	if dumper.Verbose {
		log.Info("Adding file %s", customName)
	}

	pr, pw := io.Pipe()

	fileInfo := archives.FileInfo{
		FileInfo:      info,
		NameInArchive: customName,
		Open: func() (fs.File, error) {
			go func() {
				defer pw.Close()
				_, err := io.Copy(pw, r)
				r.Close()
				if err != nil {
					pw.CloseWithError(err)
				}
			}()

			return &pipeFile{PipeReader: pr, info: info}, nil
		},
	}

	resultChan := make(chan error, 1)
	job := archives.ArchiveAsyncJob{
		File:   fileInfo,
		Result: resultChan,
	}

	select {
	case dumper.jobs <- job:
		return <-resultChan
	case err := <-dumper.done:
		return err
	}
}

// pipeFile makes io.PipeReader compatible with fs.File interface
type pipeFile struct {
	*io.PipeReader
	info os.FileInfo
}

func (f *pipeFile) Stat() (fs.FileInfo, error) { return f.info, nil }

type emptyDirFile struct {
	info os.FileInfo
}

func (f *emptyDirFile) Read([]byte) (int, error)   { return 0, io.EOF }
func (f *emptyDirFile) Close() error               { return nil }
func (f *emptyDirFile) Stat() (fs.FileInfo, error) { return f.info, nil }

func (dumper *Dumper) Close() error {
	close(dumper.jobs)
	return <-dumper.done
}

// AddFile kept for backwards compatibility since streaming is more efficient
func (dumper *Dumper) AddFile(filePath, absPath string) error {
	return dumper.AddFilePath(filePath, absPath)
}

func (dumper *Dumper) normalizeFilePath(absPath string) string {
	absPath = filepath.Clean(absPath)
	if setting.IsWindows {
		absPath = strings.ToLower(absPath)
	}
	return absPath
}

func (dumper *Dumper) GlobalExcludeAbsPath(absPaths ...string) {
	for _, absPath := range absPaths {
		dumper.globalExcludeAbsPaths = append(dumper.globalExcludeAbsPaths, dumper.normalizeFilePath(absPath))
	}
}

func (dumper *Dumper) shouldExclude(absPath string, excludes []string) bool {
	norm := dumper.normalizeFilePath(absPath)
	return slices.Contains(dumper.globalExcludeAbsPaths, norm) || slices.Contains(excludes, norm)
}

func (dumper *Dumper) AddRecursiveExclude(insidePath, absPath string, excludes []string) error {
	excludes = slices.Clone(excludes)
	for i := range excludes {
		excludes[i] = dumper.normalizeFilePath(excludes[i])
	}
	return dumper.addFileOrDir(insidePath, absPath, excludes)
}

func (dumper *Dumper) addFileOrDir(insidePath, absPath string, excludes []string) error {
	absPath, err := filepath.Abs(absPath)
	if err != nil {
		return err
	}
	dir, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer dir.Close()

	files, err := dir.Readdir(0)
	if err != nil {
		return err
	}
	for _, file := range files {
		currentAbsPath := filepath.Join(absPath, file.Name())
		if dumper.shouldExclude(currentAbsPath, excludes) {
			continue
		}

		currentInsidePath := path.Join(insidePath, file.Name())
		if file.IsDir() {
			if err := dumper.AddFile(currentInsidePath, currentAbsPath); err != nil {
				return err
			}
			if err = dumper.addFileOrDir(currentInsidePath, currentAbsPath, excludes); err != nil {
				return err
			}
		} else {
			// only copy regular files and symlink regular files, skip non-regular files like socket/pipe/...
			shouldAdd := file.Mode().IsRegular()
			if !shouldAdd && file.Mode()&os.ModeSymlink == os.ModeSymlink {
				target, err := filepath.EvalSymlinks(currentAbsPath)
				if err != nil {
					return err
				}
				targetStat, err := os.Stat(target)
				if err != nil {
					return err
				}
				shouldAdd = targetStat.Mode().IsRegular()
			}
			if shouldAdd {
				if err = dumper.AddFile(currentInsidePath, currentAbsPath); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
