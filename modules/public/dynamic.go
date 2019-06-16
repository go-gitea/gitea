// +build !bindata

// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package public

import (
	"github.com/gin-gonic/gin"
)

// Static implements the macaron static handler for serving assets.
func Static(opts *Options) gin.HandlerFunc {
	return opts.staticHandler(opts.Directory)
}
