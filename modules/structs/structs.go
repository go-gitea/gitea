// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

var visibilityModes = map[string]int{
	"public":  1,
	"limited": 2,
	"private": 3,
}
