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
		{"http://localhost/user1/repo1.git", "/", "/", "http://localhost/user1/repo1"},
		{"git@github.com:user1/repo1.git", "/", "/", "http://github.com/user1/repo1"},
		{"ssh://git@git.zefie.net:2222/zefie/lge_g6_kernel_scripts.git", "/", "/", "http://git.zefie.net/zefie/lge_g6_kernel_scripts"},
		{"git@git.zefie.net:2222/zefie/lge_g6_kernel_scripts.git", "/", "/", "http://git.zefie.net/2222/zefie/lge_g6_kernel_scripts"},
		{"git@try.gitea.io:go-gitea/gitea", "https://try.gitea.io/go-gitea/gitea", "/", "https://try.gitea.io/go-gitea/gitea"},
		{"ssh://git@try.gitea.io:9999/go-gitea/gitea", "https://try.gitea.io/go-gitea/gitea", "/", "https://try.gitea.io/go-gitea/gitea"},
		{"git://git@try.gitea.io:9999/go-gitea/gitea", "https://try.gitea.io/go-gitea/log", "/", "https://try.gitea.io/go-gitea/gitea"},
		{"ssh://git@127.0.0.1:9999/go-gitea/gitea", "https://127.0.0.1:3000/go-gitea/log", "/", "https://127.0.0.1:3000/go-gitea/gitea"},
		{"https://gitea.com:3000/user1/repo1.git", "https://127.0.0.1:3000/go-gitea/gitea", "/", "https://gitea.com:3000/user1/repo1"},
		{"https://username:password@github.com/username/repository.git", "/", "/", "https://username:password@github.com/username/repository"},
		{"somethingbad", "https://127.0.0.1:3000/go-gitea/gitea", "/", ""},
		{"git@localhost:user/repo", "https://localhost/user/repo2", "/", "https://localhost/user/repo"},
		{"../path/to/repo.git/", "https://localhost/user/repo2/src/branch/master/test", "/", "../path/to/repo.git/"},
	}

	for _, kase := range kases {
		assert.EqualValues(t, kase.expect, getRefURL(kase.refURL, kase.prefixURL, kase.parentPath))
	}
}
