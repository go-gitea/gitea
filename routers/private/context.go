// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package private includes all internal routes. The package name internal is ideal but Golang is not allowed, so we use private as package name instead.
package private

import (
	"net/http"

	"code.gitea.io/gitea/modules/context"
)

type Context struct {
	*context.Context
}

// TODO
func GetContext(req *http.Request) *Context {
	return nil
}
