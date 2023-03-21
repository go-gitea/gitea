// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package options

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

var directories = make(directorySet)

// Locale reads the content of a specific locale from static/bindata or custom path.
func Locale(name string) ([]byte, error) {
	return fileFromOptionsDir("locale", name)
}

// Readme reads the content of a specific readme from static/bindata or custom path.
func Readme(name string) ([]byte, error) {
	return fileFromOptionsDir("readme", name)
}

// Gitignore reads the content of a gitignore locale from static/bindata or custom path.
func Gitignore(name string) ([]byte, error) {
	return fileFromOptionsDir("gitignore", name)
}

// License reads the content of a specific license from static/bindata or custom path.
func License(name string) ([]byte, error) {
	return fileFromOptionsDir("license", name)
}

// Labels reads the content of a specific labels from static/bindata or custom path.
func Labels(name string) ([]byte, error) {
	return fileFromOptionsDir("label", name)
}

// WalkLocales reads the content of a specific locale
func WalkLocales(callback func(path, name string, d fs.DirEntry, err error) error) error {
	if IsDynamic() {
		if err := walkAssetDir(filepath.Join(setting.StaticRootPath, "options", "locale"), callback); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to walk locales. Error: %w", err)
		}
	}

	if err := walkAssetDir(filepath.Join(setting.CustomPath, "options", "locale"), callback); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to walk locales. Error: %w", err)
	}
	return nil
}

func walkAssetDir(root string, callback func(path, name string, d fs.DirEntry, err error) error) error {
	if err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		// name is the path relative to the root
		name := path[len(root):]
		if len(name) > 0 && name[0] == '/' {
			name = name[1:]
		}
		if err != nil {
			if os.IsNotExist(err) {
				return callback(path, name, d, err)
			}
			return err
		}
		if util.CommonSkip(d.Name()) {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		return callback(path, name, d, err)
	}); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("unable to get files for assets in %s: %w", root, err)
	}
	return nil
}

// mustLocalPathAbs coverts a path to absolute path
// FIXME: the old behavior (StaticRootPath might not be absolute), not ideal, just keep the same as before
func mustLocalPathAbs(s string) string {
	abs, err := filepath.Abs(s)
	if err != nil {
		// This should never happen in a real system. If it happens, the user must have already been in trouble: the system is not able to resolve its own paths.
		log.Fatal("Unable to get absolute path for %q: %v", s, err)
	}
	return abs
}

func joinLocalPaths(baseDirs []string, subDir string, elems ...string) (paths []string) {
	abs := make([]string, len(elems)+2)
	abs[1] = subDir
	copy(abs[2:], elems)
	for _, baseDir := range baseDirs {
		abs[0] = mustLocalPathAbs(baseDir)
		paths = append(paths, util.FilePathJoinAbs(abs...))
	}
	return paths
}

func listLocalDirIfExist(baseDirs []string, subDir string, elems ...string) (files []string, err error) {
	for _, localPath := range joinLocalPaths(baseDirs, subDir, elems...) {
		isDir, err := util.IsDir(localPath)
		if err != nil {
			return nil, fmt.Errorf("unable to check if path %q is a directory. %w", localPath, err)
		} else if !isDir {
			continue
		}

		dirFiles, err := util.StatDir(localPath, true)
		if err != nil {
			return nil, fmt.Errorf("unable to read directory %q. %w", localPath, err)
		}
		files = append(files, dirFiles...)
	}
	return files, nil
}

func readLocalFile(baseDirs []string, subDir string, elems ...string) ([]byte, error) {
	for _, localPath := range joinLocalPaths(baseDirs, subDir, elems...) {
		data, err := os.ReadFile(localPath)
		if err == nil {
			return data, nil
		} else if !os.IsNotExist(err) {
			log.Error("Unable to read file %q. Error: %v", localPath, err)
		}
	}
	return nil, os.ErrNotExist
}
