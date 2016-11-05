// +build !bindata

// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package templates

import (
	"github.com/go-gitea/gitea/modules/template"
	"gopkg.in/macaron.v1"
)

func Renderer(opts *Options) macaron.Handler {
	return macaron.Renderer(macaron.RenderOptions{
		Directory:         opts.Directory,
		AppendDirectories: opts.AppendDirectories,
		Funcs:             template.NewFuncMap(),
	})
}
