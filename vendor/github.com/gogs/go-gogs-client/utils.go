// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gogs

import (
	"net/http"
)

var jsonHeader = http.Header{"content-type": []string{"application/json"}}

func Bool(v bool) *bool {
	return &v
}

func String(v string) *string {
	return &v
}

func Int64(v int64) *int64 {
	return &v
}
