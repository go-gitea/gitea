// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package context

import (
	"net/http"
)

// PrivateContext represents a context for private routes
type PrivateContext struct {
	*Context
}

// GetPrivateContext returned private context
// TODO
func GetPrivateContext(req *http.Request) *PrivateContext {
	return nil
}
