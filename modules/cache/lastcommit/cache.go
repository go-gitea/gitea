// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package lastcommit

import (
	"fmt"

	"code.gitea.io/git"

	"github.com/go-macaron/cache"
)

type lastCommitCache struct {
	mc      cache.Cache
	timeout int64 // seconds
}

func getKey(repoPath, ref, entryPath string) string {
	return fmt.Sprintf("%s:%s:%s", repoPath, ref, entryPath)
}

func (l *lastCommitCache) Get(repoPath, ref, entryPath string) (*git.Commit, error) {
	res := l.mc.Get(getKey(repoPath, ref, entryPath))
	if res == nil {
		return nil, nil
	}
	return res.(*git.Commit), nil
}

func (l *lastCommitCache) Put(repoPath, ref, entryPath string, commit *git.Commit) error {
	return l.mc.Put(getKey(repoPath, ref, entryPath), commit, l.timeout)
}
