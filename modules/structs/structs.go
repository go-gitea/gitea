// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

// VisibilityModes is a map of org Visibility types
var VisibilityModes = map[string]int{
	"public":  0,
	"limited": 1,
	"private": 2,
}
