// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package dump

import (
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/mholt/archiver/v3"
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
	Writer  archiver.Writer
	Verbose bool

	globalExcludeAbsPaths []string
}

func (dumper *Dumper) AddReader(r io.ReadCloser, info os.FileInfo, customName string) error {
	if dumper.Verbose {
		log.Info("Adding file %s", customName)
	}

	return dumper.Writer.Write(archiver.File{
		FileInfo: archiver.FileInfo{
			FileInfo:   info,
			CustomName: customName,
		},
		ReadCloser: r,
	})
}

func (dumper *Dumper) AddFile(filePath, absPath string) error {
	file, err := os.Open(absPath)
	if err != nil {
		return err
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}
	return dumper.AddReader(file, fileInfo, filePath)
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
