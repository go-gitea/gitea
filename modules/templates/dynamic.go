// +build !bindata

// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package templates

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/log"
	template_func "code.gitea.io/gitea/modules/template"
	"gopkg.in/macaron.v1"
)

// Renderer implements the macaron handler for serving the templates.
func Renderer(opts *Options) macaron.Handler {
	return macaron.Renderer(macaron.RenderOptions{
		Directory:         opts.Directory,
		AppendDirectories: opts.Custom,
		Funcs:             template_func.NewFuncMap(),
	})
}

// Mailer provides the templates required for sending notification mails.
func Mailer(opts *Options) *template.Template {
	templates := template.New("")

	for _, funcs := range template_func.NewFuncMap() {
		templates.Funcs(funcs)
	}

	if _, err := os.Stat(opts.Directory); err == nil {
		if err := filepath.Walk(opts.Directory, func(path string, info os.FileInfo, _ error) error {
			if info.IsDir() || !strings.HasSuffix(path, ".tmpl") {
				return nil
			}
			name := strings.TrimSuffix(strings.TrimPrefix(path, opts.Directory+"/"), ".tmpl")
			log.Info("Found new template: %s", name)
			ts, err := loadTemplate(name, path)
			if err != nil {
				return nil
			}
			_, err = templates.Parse(ts)
			return err
		}); err != nil {
			log.Error(3, "Unable to parse template directory %s. %v", opts.Directory, err)
		}
	}

	for _, asset := range opts.Custom {
		if _, err := os.Stat(asset); err == nil {
			if err := filepath.Walk(asset, func(path string, info os.FileInfo, _ error) error {
				if info.IsDir() || !strings.HasSuffix(path, ".tmpl") {
					return nil
				}
				name := strings.TrimSuffix(strings.TrimPrefix(path, asset+"/"), ".tmpl")
				log.Info("Found new template: %s", name)
				ts, err := loadTemplate(name, path)
				if err != nil {
					return nil
				}
				_, err = templates.Parse(ts)
				return err
			}); err != nil {
				log.Error(3, "Unable to parse template directory %s. %v", asset, err)
			}
		}
	}

	log.Error(3, templates.DefinedTemplates())

	return templates
}

func loadTemplate(name, path string) (string, error) {
	t, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(`{{define "%s"}}%s{{end}}`, name, t), nil
}
