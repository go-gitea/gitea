// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package options

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

// Locale reads the content of a specific locale from static/bindata or custom path.
func Locale(name string) ([]byte, error) {
	return fileFromDir(path.Join("locale", path.Clean("/"+name)))
}

// Readme reads the content of a specific readme from static/bindata or custom path.
func Readme(name string) ([]byte, error) {
	return fileFromDir(path.Join("readme", path.Clean("/"+name)))
}

// Gitignore reads the content of a gitignore locale from static/bindata or custom path.
func Gitignore(name string) ([]byte, error) {
	return fileFromDir(path.Join("gitignore", path.Clean("/"+name)))
}

// License reads the content of a specific license from static/bindata or custom path.
func License(name string) ([]byte, error) {
	return fileFromDir(path.Join("license", path.Clean("/"+name)))
}

// Labels reads the content of a specific labels from static/bindata or custom path.
func Labels(name string) ([]byte, error) {
	return fileFromDir(path.Join("label", path.Clean("/"+name)))
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

func statDirIfExist(dir string) ([]string, error) {
	isDir, err := util.IsDir(dir)
	if err != nil {
		return nil, fmt.Errorf("unable to check if static directory %s is a directory. %w", dir, err)
	}
	if !isDir {
		return nil, nil
	}
	files, err := util.StatDir(dir, true)
	if err != nil {
		return nil, fmt.Errorf("unable to read directory %q. %w", dir, err)
	}
	return files, nil
}
