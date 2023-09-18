// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_21 //nolint

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"code.gitea.io/gitea/modules/git"
	giturl "code.gitea.io/gitea/modules/git/url"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func AddRemoteAddressToMirrors(x *xorm.Engine) error {
	type Mirror struct {
		RemoteAddress string `xorm:"VARCHAR(2048)"`
	}

	type PushMirror struct {
		RemoteAddress string `xorm:"VARCHAR(2048)"`
	}

	if err := x.Sync(new(Mirror), new(PushMirror)); err != nil {
		return err
	}

	if err := migratePullMirrors(x); err != nil {
		return err
	}

	return migratePushMirrors(x)
}

func migratePullMirrors(x *xorm.Engine) error {
	type Mirror struct {
		ID            int64  `xorm:"pk autoincr"`
		RepoID        int64  `xorm:"INDEX"`
		RemoteAddress string `xorm:"VARCHAR(2048)"`
		RepoOwner     string
		RepoName      string
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	limit := setting.Database.IterateBufferSize
	if limit <= 0 {
		limit = 50
	}

	start := 0

	for {
		var mirrors []Mirror
		if err := sess.Select("mirror.id, mirror.repo_id, mirror.remote_address, repository.owner_name as repo_owner, repository.name as repo_name").
			Join("INNER", "repository", "repository.id = mirror.repo_id").
			Limit(limit, start).Find(&mirrors); err != nil {
			return err
		}

		if len(mirrors) == 0 {
			break
		}
		start += len(mirrors)

		for _, m := range mirrors {
			remoteAddress, err := getRemoteAddress(m.RepoOwner, m.RepoName, "origin")
			if err != nil {
				return err
			}

			m.RemoteAddress = remoteAddress

			if _, err = sess.ID(m.ID).Cols("remote_address").Update(m); err != nil {
				return err
			}
		}

		if start%1000 == 0 { // avoid a too big transaction
			if err := sess.Commit(); err != nil {
				return err
			}
			if err := sess.Begin(); err != nil {
				return err
			}
		}
	}

	return sess.Commit()
}

func migratePushMirrors(x *xorm.Engine) error {
	type PushMirror struct {
		ID            int64 `xorm:"pk autoincr"`
		RepoID        int64 `xorm:"INDEX"`
		RemoteName    string
		RemoteAddress string `xorm:"VARCHAR(2048)"`
		RepoOwner     string
		RepoName      string
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	limit := setting.Database.IterateBufferSize
	if limit <= 0 {
		limit = 50
	}

	start := 0

	for {
		var mirrors []PushMirror
		if err := sess.Select("push_mirror.id, push_mirror.repo_id, push_mirror.remote_name, push_mirror.remote_address, repository.owner_name as repo_owner, repository.name as repo_name").
			Join("INNER", "repository", "repository.id = push_mirror.repo_id").
			Limit(limit, start).Find(&mirrors); err != nil {
			return err
		}

		if len(mirrors) == 0 {
			break
		}
		start += len(mirrors)

		for _, m := range mirrors {
			remoteAddress, err := getRemoteAddress(m.RepoOwner, m.RepoName, m.RemoteName)
			if err != nil {
				return err
			}

			m.RemoteAddress = remoteAddress

			if _, err = sess.ID(m.ID).Cols("remote_address").Update(m); err != nil {
				return err
			}
		}

		if start%1000 == 0 { // avoid a too big transaction
			if err := sess.Commit(); err != nil {
				return err
			}
			if err := sess.Begin(); err != nil {
				return err
			}
		}
	}

	return sess.Commit()
}

func getRemoteAddress(ownerName, repoName, remoteName string) (string, error) {
	repoPath := filepath.Join(setting.RepoRootPath, strings.ToLower(ownerName), strings.ToLower(repoName)+".git")

	remoteURL, err := git.GetRemoteAddress(context.Background(), repoPath, remoteName)
	if err != nil {
		return "", fmt.Errorf("get remote %s's address of %s/%s failed: %v", remoteName, ownerName, repoName, err)
	}

	u, err := giturl.Parse(remoteURL)
	if err != nil {
		return "", err
	}
	u.User = nil

	return u.String(), nil
}
