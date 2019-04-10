// Copyright 2019-present The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build tools

// tools is a shim package to cause go-bindata to register as a dependency
package tools

import (
	_ "github.com/jteeuwen/go-bindata/go-bindata"
)
