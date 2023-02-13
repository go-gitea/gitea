// Copyright 2016 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !servedynamic

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
	"code.gitea.io/gitea/templates"
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

func GetTemplateAssetNames() []string {
	var tmpls []string
	for _, k := range templates.AssetNames() {
		if strings.HasPrefix(k, "mail/") {
			continue
		}
		tmpls = append(tmpls, path.Join("templates", k))
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
	f, err := templates.TemplatesFS.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(f)
}

// BuiltinAssetNames returns the names of the built-in embedded assets
func BuiltinAssetNames() []string {
	return templates.AssetNames()
}

// BuiltinAssetIsDir returns if a provided asset is a directory
func BuiltinAssetIsDir(name string) (bool, error) {
	f, err := templates.TemplatesFS.Open(name)
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
