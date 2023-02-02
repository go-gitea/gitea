// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !bindata

package templates

import (
	"html/template"
	"io/fs"
	"os"
	"path/filepath"
	texttmpl "text/template"

	"code.gitea.io/gitea/modules/setting"
)

var (
	subjectTemplates = texttmpl.New("")
	bodyTemplates    = template.New("")
)

// GetAsset returns asset content via name
func GetAsset(name string) ([]byte, error) {
	bs, err := os.ReadFile(filepath.Join(setting.CustomPath, name))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	} else if err == nil {
		return bs, nil
	}

	return os.ReadFile(filepath.Join(setting.StaticRootPath, name))
}

// GetAssetFilename returns the filename of the provided asset
func GetAssetFilename(name string) (string, error) {
	filename := filepath.Join(setting.CustomPath, name)
	_, err := os.Stat(filename)
	if err != nil && !os.IsNotExist(err) {
		return filename, err
	} else if err == nil {
		return filename, nil
	}

	filename = filepath.Join(setting.StaticRootPath, name)
	_, err = os.Stat(filename)
	return filename, err
}

// walkTemplateFiles calls a callback for each template asset
func walkTemplateFiles(callback func(path, name string, d fs.DirEntry, err error) error) error {
	if err := walkAssetDir(filepath.Join(setting.CustomPath, "templates"), true, callback); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := walkAssetDir(filepath.Join(setting.StaticRootPath, "templates"), true, callback); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// GetTemplateAssetNames returns list of template names
func GetTemplateAssetNames() []string {
	tmpls := getDirTemplateAssetNames(filepath.Join(setting.CustomPath, "templates"))
	tmpls2 := getDirTemplateAssetNames(filepath.Join(setting.StaticRootPath, "templates"))
	return append(tmpls, tmpls2...)
}

func walkMailerTemplates(callback func(path, name string, d fs.DirEntry, err error) error) error {
	if err := walkAssetDir(filepath.Join(setting.StaticRootPath, "templates", "mail"), false, callback); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := walkAssetDir(filepath.Join(setting.CustomPath, "templates", "mail"), false, callback); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// BuiltinAsset will read the provided asset from the embedded assets
// (This always returns os.ErrNotExist)
func BuiltinAsset(name string) ([]byte, error) {
	return nil, os.ErrNotExist
}

// BuiltinAssetNames returns the names of the embedded assets
// (This always returns nil)
func BuiltinAssetNames() []string {
	return nil
}
