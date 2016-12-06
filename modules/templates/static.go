// +build bindata

// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package templates

import (
	"html/template"
	"io/ioutil"
	"path"
	"strings"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"github.com/Unknwon/com"
	"github.com/go-macaron/bindata"
	"gopkg.in/macaron.v1"
)

var (
	templates = template.New("")
)

// Renderer implements the macaron handler for serving the templates.
func Renderer() macaron.Handler {
	return macaron.Renderer(macaron.RenderOptions{
		Funcs: NewFuncMap(),
		AppendDirectories: []string{
			path.Join(setting.CustomPath, "templates"),
		},
		TemplateFileSystem: bindata.Templates(
			bindata.Options{
				Asset:      Asset,
				AssetDir:   AssetDir,
				AssetInfo:  AssetInfo,
				AssetNames: AssetNames,
				Prefix:     "",
			},
		),
	})
}

// Mailer provides the templates required for sending notification mails.
func Mailer() *template.Template {
	for _, funcs := range NewFuncMap() {
		templates.Funcs(funcs)
	}

	for _, assetPath := range AssetNames() {
		if !strings.HasPrefix(assetPath, "mail/") {
			continue
		}

		if !strings.HasSuffix(assetPath, ".tmpl") {
			continue
		}

		content, err := Asset(assetPath)

		if err != nil {
			log.Warn("Failed to read embedded %s template. %v", assetPath, err)
			continue
		}

		templates.New(
			strings.TrimPrefix(
				strings.TrimSuffix(
					assetPath,
					".tmpl",
				),
				"mail/",
			),
		).Parse(string(content))
	}

	customDir := path.Join(setting.CustomPath, "templates", "mail")

	if com.IsDir(customDir) {
		files, err := com.StatDir(customDir)

		if err != nil {
			log.Warn("Failed to read %s templates dir. %v", customDir, err)
		} else {
			for _, filePath := range files {
				if !strings.HasSuffix(filePath, ".tmpl") {
					continue
				}

				content, err := ioutil.ReadFile(path.Join(customDir, filePath))

				if err != nil {
					log.Warn("Failed to read custom %s template. %v", filePath, err)
					continue
				}

				templates.New(
					strings.TrimSuffix(
						filePath,
						".tmpl",
					),
				).Parse(string(content))
			}
		}
	}

	return templates
}
