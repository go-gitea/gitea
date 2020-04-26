// Copyright 2018 Google LLC. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

// +build appenginevm

package internal
import "code.gitea.io/gitea/traceinit"

func init() {
traceinit.Trace("./vendor/google.golang.org/appengine/internal/identity_flex.go")
	appengineFlex = true
}
