// +build bindata

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
	"github.com/go-macaron/bindata"
	"gopkg.in/macaron.v1"
)

// Renderer implements the macaron handler for serving the templates.
func Renderer(opts *Options) macaron.Handler {
	return macaron.Renderer(macaron.RenderOptions{
		AppendDirectories: opts.Custom,
		Funcs:             template_func.NewFuncMap(),
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
func Mailer(opts *Options) *template.Template {
	templates := template.New("")

	for _, funcs := range template_func.NewFuncMap() {
		templates.Funcs(funcs)
	}

	assets, err := AssetDir("mail")

	if err != nil {
		log.Error(3, "Unable to read mail asset dir. %s", err)
	}

	for _, asset := range assets {
		bytes, err := Asset(asset)

		if err != nil {
			log.Error(3, "Unable to parse template %s. %s", asset, err)
		}

		templates.New(asset).Parse(string(bytes))
	}

	for _, asset := range opts.Custom {
		if _, err := os.Stat(opts.Directory); err == nil {
			if _, err := templates.ParseGlob(path.Join(asset, "*", "*.tmpl")); err != nil {
				log.Error(3, "Unable to parse template directory %s. %s", opts.Directory, err)
			}
		}
	}

	return templates
}
