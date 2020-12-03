// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// +build gofuzz

package fuzz

import (
	"code.gitea.io/gitea/modules/markup"
	"code.gitea.io/gitea/modules/markup/markdown"
)

func Fuzz(data []byte) int {
	_ = markdown.RenderRaw(data, "", false)
	return 1
}

func Fuzz2(data []byte) int {
	var localMetas = map[string]string{
		"user": "gogits",
		"repo": "gogs",
	}
	_, err := markup.PostProcess(data, "/tmp", localMetas, false)
	if err != nil {
		return 0
	}
	return 1
}
