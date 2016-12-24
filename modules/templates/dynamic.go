// +build !bindata

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
	"gopkg.in/macaron.v1"
)

var (
	templates = template.New("")
)

// Renderer implements the macaron handler for serving the templates.
func Renderer() macaron.Handler {
	return macaron.Renderer(macaron.RenderOptions{
		Funcs:     NewFuncMap(),
		Directory: path.Join(setting.StaticRootPath, "templates"),
		AppendDirectories: []string{
			path.Join(setting.CustomPath, "templates"),
		},
	})
}

// Mailer provides the templates required for sending notification mails.
func Mailer() *template.Template {
	for _, funcs := range NewFuncMap() {
		templates.Funcs(funcs)
	}

	staticDir := path.Join(setting.StaticRootPath, "templates", "mail")

	if com.IsDir(staticDir) {
		files, err := com.StatDir(staticDir)

		if err != nil {
			log.Warn("Failed to read %s templates dir. %v", staticDir, err)
		} else {
			for _, filePath := range files {
				if !strings.HasSuffix(filePath, ".tmpl") {
					continue
				}

				content, err := ioutil.ReadFile(path.Join(staticDir, filePath))

				if err != nil {
					log.Warn("Failed to read static %s template. %v", filePath, err)
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
