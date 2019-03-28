// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetRefURL(t *testing.T) {
	var kases = []struct {
		refURL     string
		prefixURL  string
		parentPath string
		expect     string
	}{
		{"git://github.com/user1/repo1", "/", "/", "http://github.com/user1/repo1"},
		{"https://localhost/user1/repo1.git", "/", "/", "https://localhost/user1/repo1"},
		{"git@github.com/user1/repo1.git", "/", "/", "git@github.com/user1/repo1"},
		{"ssh://git@git.zefie.net:2222/zefie/lge_g6_kernel_scripts.git", "/", "/", "http://git.zefie.net/zefie/lge_g6_kernel_scripts"},
	}

	for _, kase := range kases {
		assert.EqualValues(t, kase.expect, getRefURL(kase.refURL, kase.prefixURL, kase.parentPath))
	}
}
