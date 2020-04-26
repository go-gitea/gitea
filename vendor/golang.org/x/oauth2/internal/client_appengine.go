// Copyright 2018 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build appengine

package internal
import "code.gitea.io/gitea/traceinit"

import "google.golang.org/appengine/urlfetch"

func init() {
traceinit.Trace("./vendor/golang.org/x/oauth2/internal/client_appengine.go")
	appengineClientHook = urlfetch.Client
}
