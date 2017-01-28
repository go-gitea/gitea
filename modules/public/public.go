// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

import (
	"path"

	"code.gitea.io/gitea/modules/setting"
	"gopkg.in/macaron.v1"
)

//go:generate go-bindata -tags "bindata" -ignore "\\.go|\\.less" -pkg "public" -o "bindata.go" ../../public/...
//go:generate go fmt bindata.go
//go:generate sed -i.bak s/..\/..\/public\/// bindata.go
//go:generate rm -f bindata.go.bak

// Options represents the available options to configure the macaron handler.
type Options struct {
	Directory   string
	SkipLogging bool
}

// Custom implements the macaron static handler for serving custom assets.
func Custom(opts *Options) macaron.Handler {
	return macaron.Static(
		path.Join(setting.CustomPath, "public"),
		macaron.StaticOptions{
			SkipLogging: opts.SkipLogging,
		},
	)
}
