// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build bindata

package templates

import (
	"html/template"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	texttmpl "text/template"
	"time"

	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/timeutil"
)

var (
	subjectTemplates = texttmpl.New("")
	bodyTemplates    = template.New("")
)

// GlobalModTime provide a global mod time for embedded asset files
func GlobalModTime(filename string) time.Time {
	return timeutil.GetExecutableModTime()
}

// GetAssetFilename returns the filename of the provided asset
func GetAssetFilename(name string) (string, error) {
	filename := filepath.Join(setting.CustomPath, name)
	_, err := os.Stat(filename)
	if err != nil && !os.IsNotExist(err) {
		return name, err
	} else if err == nil {
		return filename, nil
	}
	return "(builtin) " + name, nil
}

// GetAsset get a special asset, only for chi
func GetAsset(name string) ([]byte, error) {
	bs, err := os.ReadFile(filepath.Join(setting.CustomPath, name))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	} else if err == nil {
		return bs, nil
	}
	return BuiltinAsset(strings.TrimPrefix(name, "templates/"))
}

// GetFiles calls a callback for each template asset
func walkTemplateFiles(callback func(path, name string, d fs.DirEntry, err error) error) error {
	if err := walkAssetDir(filepath.Join(setting.CustomPath, "templates"), true, callback); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// GetTemplateAssetNames only for chi
func GetTemplateAssetNames() []string {
	realFS := Assets.(vfsgen۰FS)
	tmpls := make([]string, 0, len(realFS))
	for k := range realFS {
		if strings.HasPrefix(k, "/mail/") {
			continue
		}
		tmpls = append(tmpls, "templates/"+k[1:])
	}

	customDir := path.Join(setting.CustomPath, "templates")
	customTmpls := getDirTemplateAssetNames(customDir)
	return append(tmpls, customTmpls...)
}

func walkMailerTemplates(callback func(path, name string, d fs.DirEntry, err error) error) error {
	if err := walkAssetDir(filepath.Join(setting.CustomPath, "templates", "mail"), false, callback); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// BuiltinAsset reads the provided asset from the builtin embedded assets
func BuiltinAsset(name string) ([]byte, error) {
	f, err := Assets.Open("/" + name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// BuiltinAssetNames returns the names of the built-in embedded assets
func BuiltinAssetNames() []string {
	realFS := Assets.(vfsgen۰FS)
	results := make([]string, 0, len(realFS))
	for k := range realFS {
		results = append(results, k[1:])
	}
	return results
}

// BuiltinAssetIsDir returns if a provided asset is a directory
func BuiltinAssetIsDir(name string) (bool, error) {
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
