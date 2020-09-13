// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package svg

// SVGs contains discovered SVGs
var SVGs map[string]string

// Init discovers SVGs and populates the `SVGs` variable
func Init() {
	SVGs = Discover()
}
