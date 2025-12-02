// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dump

import (
	"context"
	"errors"
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
	Verbose bool

	jobs            chan archives.ArchiveAsyncJob
	errArchiveAsync chan error
	errArchiveJob   chan error

	globalExcludeAbsPaths []string
}

func NewDumper(ctx context.Context, format string, output io.Writer) (*Dumper, error) {
	d := &Dumper{
		jobs:            make(chan archives.ArchiveAsyncJob, 1),
		errArchiveAsync: make(chan error, 1),
		errArchiveJob:   make(chan error, 1),
	}

	// TODO: in the future, we could completely drop the "mholt/archives" dependency.
	// Then we only need to support "zip" and ".tar.gz" natively, and let users provide custom command line tools
	// like "zstd" or "xz" with compression-level arguments.
	var comp archives.ArchiverAsync
	switch format {
	case "zip":
		comp = archives.Zip{}
	case "tar":
		comp = archives.Tar{}
	case "tar.sz":
		comp = archives.CompressedArchive{Compression: archives.Sz{}, Archival: archives.Tar{}}
	case "tar.gz":
		comp = archives.CompressedArchive{Compression: archives.Gz{}, Archival: archives.Tar{}}
	case "tar.xz":
		comp = archives.CompressedArchive{Compression: archives.Xz{}, Archival: archives.Tar{}}
	case "tar.bz2":
		comp = archives.CompressedArchive{Compression: archives.Bz2{}, Archival: archives.Tar{}}
	case "tar.br":
		comp = archives.CompressedArchive{Compression: archives.Brotli{}, Archival: archives.Tar{}}
	case "tar.lz4":
		comp = archives.CompressedArchive{Compression: archives.Lz4{}, Archival: archives.Tar{}}
	case "tar.zst":
		comp = archives.CompressedArchive{Compression: archives.Zstd{}, Archival: archives.Tar{}}
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
	go func() {
		d.errArchiveAsync <- comp.ArchiveAsync(ctx, output, d.jobs)
		close(d.errArchiveAsync)
	}()
	return d, nil
}

func (dumper *Dumper) runArchiveJob(job archives.ArchiveAsyncJob) error {
	dumper.jobs <- job
	select {
	case err := <-dumper.errArchiveAsync:
		if err == nil {
			return errors.New("archiver has been closed")
		}
		return err
	case err := <-dumper.errArchiveJob:
		return err
	}
}

// AddFileByPath adds a file by its filesystem path
func (dumper *Dumper) AddFileByPath(filePath, absPath string) error {
	if dumper.Verbose {
		log.Info("Adding local file %s", filePath)
	}

	fileInfo, err := os.Stat(absPath)
	if err != nil {
		return err
	}

	archiveFileInfo := archives.FileInfo{
		FileInfo:      fileInfo,
		NameInArchive: filePath,
		Open:          func() (fs.File, error) { return os.Open(absPath) },
	}

	return dumper.runArchiveJob(archives.ArchiveAsyncJob{
		File:   archiveFileInfo,
		Result: dumper.errArchiveJob,
	})
}

type readerFile struct {
	r    io.Reader
	info os.FileInfo
}

var _ fs.File = (*readerFile)(nil)

func (f *readerFile) Stat() (fs.FileInfo, error)     { return f.info, nil }
func (f *readerFile) Read(bytes []byte) (int, error) { return f.r.Read(bytes) }
func (f *readerFile) Close() error                   { return nil }

// AddFileByReader adds a file's contents from a Reader
func (dumper *Dumper) AddFileByReader(r io.Reader, info os.FileInfo, customName string) error {
	if dumper.Verbose {
		log.Info("Adding storage file %s", customName)
	}

	fileInfo := archives.FileInfo{
		FileInfo:      info,
		NameInArchive: customName,
		Open:          func() (fs.File, error) { return &readerFile{r, info}, nil },
	}
	return dumper.runArchiveJob(archives.ArchiveAsyncJob{
		File:   fileInfo,
		Result: dumper.errArchiveJob,
	})
}

func (dumper *Dumper) Close() error {
	close(dumper.jobs)
	return <-dumper.errArchiveAsync
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
			if err := dumper.AddFileByPath(currentInsidePath, currentAbsPath); err != nil {
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
				if err = dumper.AddFileByPath(currentInsidePath, currentAbsPath); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
