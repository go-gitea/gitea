// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"testing"

	"code.gitea.io/gitea/models"
	"github.com/stretchr/testify/assert"
)

func TestCleanUploadName(t *testing.T) {
	models.PrepareTestEnv(t)

	var kases = map[string]string{
		".git/refs/master": "git/refs/master",
		"/root/abc":        "root/abc",
		"./../../abc":      "abc",
		"a/../.git":        "a/.git",
		"a/../../../abc":   "a/abc",
		"../../../acd":     "acd",
		"../../.git/abc":   "git/abc",
		"..\\..\\.git/abc": "git/abc",
	}
	for k, v := range kases {
		assert.EqualValues(t, v, cleanUploadFileName(k))
	}
}
