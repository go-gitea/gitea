// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build bindata

package options

import (
	"fmt"
	"io"
	"path/filepath"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"
)

var directories = make(directorySet)

// Dir returns all files from custom directory or bindata.
func Dir(name string) ([]string, error) {
	if directories.Filled(name) {
		return directories.Get(name), nil
	}

	var result []string

	for _, dir := range []string{
		filepath.Join(setting.CustomPath, "options", name), // custom dir
		// no static dir
	} {
		files, err := statDirIfExist(dir)
		if err != nil {
			return nil, err
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
	d, err := Assets.Open(dirName)
	if err != nil {
		return nil, err
	}
	defer d.Close()

	files, err := d.Readdir(-1)
	if err != nil {
		return nil, err
	}
	results := make([]string, 0, len(files))
	for _, file := range files {
		results = append(results, file.Name())
	}
	return results, nil
}

// fileFromOptionsDir is a helper to read files from custom path or bindata.
func fileFromOptionsDir(elems ...string) ([]byte, error) {
	// only try custom dir, no static dir
	if data, err := readFileFromLocal([]string{setting.CustomPath}, "options", elems...); err == nil {
		return data, nil
	}

	f, err := Assets.Open(util.SafePathRelX(elems...))
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func Asset(name string) ([]byte, error) {
	f, err := Assets.Open("/" + name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

func AssetNames() []string {
	realFS := Assets.(vfsgen۰FS)
	results := make([]string, 0, len(realFS))
	for k := range realFS {
		results = append(results, k[1:])
	}
	return results
}

func AssetIsDir(name string) (bool, error) {
	if f, err := Assets.Open("/" + name); err != nil {
		return false, err
	} else {
		defer f.Close()
		if fi, err := f.Stat(); err != nil {
			return false, err
		} else {
			return fi.IsDir(), nil
		}
	}
}

// IsDynamic will return false when using embedded data (-tags bindata)
func IsDynamic() bool {
	return false
}
