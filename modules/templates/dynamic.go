// +build !bindata

// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package templates

import (
	"html/template"
	"io/ioutil"
	"path"
	"regexp"
	"strings"
	texttmpl "text/template"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"gitea.com/macaron/macaron"
	"github.com/unknwon/com"
)

var (
	bodyTemplates    = template.New("")
	subjectTemplates = texttmpl.New("")
	mailSubjectSplit = regexp.MustCompile(`(?m)^-{3,}[\s]*$`)
)

// HTMLRenderer implements the macaron handler for serving HTML templates.
func HTMLRenderer() macaron.Handler {
	return macaron.Renderer(macaron.RenderOptions{
		Funcs:     NewFuncMap(),
		Directory: path.Join(setting.StaticRootPath, "templates"),
		AppendDirectories: []string{
			path.Join(setting.CustomPath, "templates"),
		},
	})
}

// JSONRenderer implements the macaron handler for serving JSON templates.
func JSONRenderer() macaron.Handler {
	return macaron.Renderer(macaron.RenderOptions{
		Funcs:     NewFuncMap(),
		Directory: path.Join(setting.StaticRootPath, "templates"),
		AppendDirectories: []string{
			path.Join(setting.CustomPath, "templates"),
		},
		HTMLContentType: "application/json",
	})
}

// JSRenderer implements the macaron handler for serving JS templates.
func JSRenderer() macaron.Handler {
	return macaron.Renderer(macaron.RenderOptions{
		Funcs:     NewFuncMap(),
		Directory: path.Join(setting.StaticRootPath, "templates"),
		AppendDirectories: []string{
			path.Join(setting.CustomPath, "templates"),
		},
		HTMLContentType: "application/javascript",
	})
}

// Mailer provides the templates required for sending notification mails.
func Mailer() (*texttmpl.Template, *template.Template) {
	for _, funcs := range NewTextFuncMap() {
		subjectTemplates.Funcs(funcs)
	}
	for _, funcs := range NewFuncMap() {
		bodyTemplates.Funcs(funcs)
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

				buildSubjectBodyTemplate(strings.TrimSuffix(filePath, ".tmpl"), content)
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

				buildSubjectBodyTemplate(strings.TrimSuffix(filePath, ".tmpl"), content)
			}
		}
	}

	return subjectTemplates, bodyTemplates
}

func buildSubjectBodyTemplate(name string, content []byte) {
	// Split template into subject and body
	var subjectContent []byte
	bodyContent := content
	loc := mailSubjectSplit.FindIndex(content)
	if loc != nil {
		subjectContent = content[0:loc[0]]
		bodyContent = content[loc[1]:]
	}
	if _, err := subjectTemplates.New(name).
		Parse(string(subjectContent)); err != nil {
		log.Warn("Failed to parse template [%s/subject]: %v", name, err)
	}
	if _, err := bodyTemplates.New(name).
		Parse(string(bodyContent)); err != nil {
		log.Warn("Failed to parse template [%s/body]: %v", name, err)
	}
}
