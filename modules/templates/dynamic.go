// +build !bindata

// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package templates

import (
	"html/template"
	"os"
	"path"

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
		if _, err := templates.ParseGlob(path.Join(opts.Directory, "*", "*.tmpl")); err != nil {
			log.Error(3, "Unable to parse template directory %s. %s", opts.Directory, err)
		}
	}

	for _, asset := range opts.Custom {
		if _, err := os.Stat(asset); err == nil {
			if _, err := templates.ParseGlob(path.Join(asset, "*", "*.tmpl")); err != nil {
				log.Error(3, "Unable to parse template directory %s. %s", asset, err)
			}
		}
	}

	log.Error(3, templates.DefinedTemplates())

	return templates
}
