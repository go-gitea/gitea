// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !servedynamic

package options

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/options"
)

var directories = make(directorySet)

// Dir returns all files from embedded assets or custom directory.
func Dir(name string) ([]string, error) {
	if directories.Filled(name) {
		return directories.Get(name), nil
	}

	var result []string

	customDir := path.Join(setting.CustomPath, "options", name)
	isDir, err := util.IsDir(customDir)
	if err != nil {
		return []string{}, fmt.Errorf("unable to check if custom directory %q is a directory. %w", customDir, err)
	}
	if isDir {
		files, err := util.StatDir(customDir, true)
		if err != nil {
			return []string{}, fmt.Errorf("unable to read custom directory %q. %w", customDir, err)
		}

		result = append(result, files...)
	}

	files, err := AssetDir(name)
	if err != nil {
		return []string{}, fmt.Errorf("unable to read embedded directory %q. %w", name, err)
	}

	result = append(result, files...)
	return directories.AddAndGet(name, result), nil
}

func AssetDir(dirName string) ([]string, error) {
	return options.AssetDir(dirName)
}

// Locale reads the content of a specific locale from embedded assets or custom path.
func Locale(name string) ([]byte, error) {
	return fileFromDir(path.Join("locale", name))
}

// WalkLocales reads the content of a specific locale from static or custom path.
func WalkLocales(callback func(path, name string, d fs.DirEntry, err error) error) error {
	if err := walkAssetDir(filepath.Join(setting.CustomPath, "options", "locale"), callback); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to walk locales. Error: %w", err)
	}
	return nil
}

// Readme reads the content of a specific readme from embedded assets or custom path.
func Readme(name string) ([]byte, error) {
	return fileFromDir(path.Join("readme", name))
}

// Gitignore reads the content of a gitignore locale from embedded assets or custom path.
func Gitignore(name string) ([]byte, error) {
	return fileFromDir(path.Join("gitignore", name))
}

// License reads the content of a specific license from embedded assets or custom path.
func License(name string) ([]byte, error) {
	return fileFromDir(path.Join("license", name))
}

// Labels reads the content of a specific labels from static or custom path.
func Labels(name string) ([]byte, error) {
	return fileFromDir(path.Join("label", name))
}

// fileFromDir is a helper to read files from embedded assets or custom path.
func fileFromDir(name string) ([]byte, error) {
	customPath := path.Join(setting.CustomPath, "options", name)

	isFile, err := util.IsFile(customPath)
	if err != nil {
		log.Error("Unable to check if %s is a file. Error: %v", customPath, err)
	}
	if isFile {
		return os.ReadFile(customPath)
	}

	f, err := options.OptionsFS.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return io.ReadAll(f)
}

func Asset(name string) ([]byte, error) {
	f, err := options.OptionsFS.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func AssetNames() []string {
	return options.AssetNames()
}

func AssetIsDir(name string) (bool, error) {
	f, err := options.OptionsFS.Open(name)
	if err != nil {
		return false, err
	}
	defer f.Close()
	fi, err := f.Stat()
	if err != nil {
		return false, err
	}
	return fi.IsDir(), nil
}

// IsDynamic will return false when using embedded data.
func IsDynamic() bool {
	return false
}
