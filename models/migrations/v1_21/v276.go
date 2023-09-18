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
		if err := sess.Limit(limit, start).Find(&mirrors); err != nil {
			return err
		}

		if len(mirrors) == 0 {
			break
		}
		start += len(mirrors)

		for _, m := range mirrors {
			remoteAddress, err := getRemoteAddress(sess, m.RepoID, "origin")
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
		if err := sess.Limit(limit, start).Find(&mirrors); err != nil {
			return err
		}

		if len(mirrors) == 0 {
			break
		}
		start += len(mirrors)

		for _, m := range mirrors {
			remoteAddress, err := getRemoteAddress(sess, m.RepoID, m.RemoteName)
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

func getRemoteAddress(sess *xorm.Session, repoID int64, remoteName string) (string, error) {
	var ownerName string
	var repoName string
	has, err := sess.
		Table("repository").
		Cols("owner_name", "lower_name").
		Where("id=?", repoID).
		Get(&ownerName, &repoName)
	if err != nil {
		return "", err
	} else if !has {
		return "", fmt.Errorf("repository [%v] not found", repoID)
	}

	repoPath := filepath.Join(setting.RepoRootPath, strings.ToLower(ownerName), strings.ToLower(repoName)+".git")

	remoteURL, err := git.GetRemoteAddress(context.Background(), repoPath, remoteName)
	if err != nil {
		return "", err
	}

	u, err := giturl.Parse(remoteURL)
	if err != nil {
		return "", err
	}
	u.User = nil

	return u.String(), nil
}
