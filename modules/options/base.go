// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package options

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"code.gitea.io/gitea/modules/util"
)

// Locale reads the content of a specific locale from static/bindata or custom path.
func Locale(name string) ([]byte, error) {
	return fileFromDir(path.Join("locale", name))
}

// Readme reads the content of a specific readme from static/bindata or custom path.
func Readme(name string) ([]byte, error) {
	return fileFromDir(path.Join("readme", name))
}

// Gitignore reads the content of a gitignore locale from static/bindata or custom path.
func Gitignore(name string) ([]byte, error) {
	return fileFromDir(path.Join("gitignore", name))
}

// License reads the content of a specific license from static/bindata or custom path.
func License(name string) ([]byte, error) {
	return fileFromDir(path.Join("license", name))
}

// Labels reads the content of a specific labels from static/bindata or custom path.
func Labels(name string) ([]byte, error) {
	return fileFromDir(path.Join("label", name))
}

// WalkLocales reads the content of a specific locale from static or custom path.
func WalkLocales(callback func(path, name string, d fs.DirEntry, err error) error) error {
	for _, v := range pathsForWalkLocales() {
		if err := walkAssetDir(v, callback); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to walk locales. Error: %w", err)
		}
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
