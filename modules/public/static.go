// +build bindata

// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

import (
	"gitea.com/macaron/macaron"
)

// Static implements the macaron static handler for serving assets.
func Static(opts *Options) macaron.Handler {
	opts.FileSystem = Assets
	// we don't need to pass the directory, because the directory var is only
	// used when in the options there is no FileSystem.
	return opts.staticHandler("")
}
