// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func removeStaleWatches(x *xorm.Engine) error {
	type Watch struct {
		ID     int64
		UserID int64
		RepoID int64
	}

	type IssueWatch struct {
		ID         int64
		UserID     int64
		RepoID     int64
		IsWatching bool
	}

	type Repository struct {
		ID        int64
		IsPrivate bool
		OwnerID   int64
	}

	type Access struct {
		UserID int64
		RepoID int64
		Mode   int
	}

	const (
		// AccessModeNone no access
		AccessModeNone int = iota // 0
		// AccessModeRead read access
		AccessModeRead // 1
	)

	accessLevel := func(e *xorm.Session, userID int64, repo *Repository) (int, error) {
		mode := AccessModeNone
		if !repo.IsPrivate {
			mode = AccessModeRead
		}

		if userID == 0 {
			return mode, nil
		}

		if userID == repo.OwnerID {
			return 4, nil
		}

		a := &Access{UserID: userID, RepoID: repo.ID}
		if has, err := e.Get(a); !has || err != nil {
			return mode, err
		}
		return a.Mode, nil
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	var issueWatch IssueWatch
	if exist, err := sess.IsTableExist(&issueWatch); err != nil {
		return fmt.Errorf("IsExist IssueWatch: %v", err)
	} else if !exist {
		return nil
	}

	repoCache := make(map[int64]*Repository)
	err := sess.BufferSize(setting.Database.IterateBufferSize).Iterate(new(Watch),
		func(idx int, bean interface{}) error {
			watch := bean.(*Watch)

			repo := repoCache[watch.RepoID]
			if repo == nil {
				repo = &Repository{
					ID: watch.RepoID,
				}
				if _, err := sess.Get(repo); err != nil {
					return err
				}
				repoCache[watch.RepoID] = repo
			}

			// Remove watches from now unaccessible repositories
			mode, err := accessLevel(sess, watch.UserID, repo)
			if err != nil {
				return err
			}
			has := AccessModeRead <= mode
			if has {
				return nil
			}

			if _, err = sess.Delete(&Watch{0, watch.UserID, repo.ID}); err != nil {
				return err
			}
			_, err = sess.Exec("UPDATE `repository` SET num_watches = num_watches - 1 WHERE id = ?", repo.ID)

			return err
		})
	if err != nil {
		return err
	}

	repoCache = make(map[int64]*Repository)
	err = sess.BufferSize(setting.Database.IterateBufferSize).
		Distinct("issue_watch.user_id", "issue.repo_id").
		Join("INNER", "issue", "issue_watch.issue_id = issue.id").
		Where("issue_watch.is_watching = ?", true).
		Iterate(new(IssueWatch),
			func(idx int, bean interface{}) error {
				watch := bean.(*IssueWatch)

				repo := repoCache[watch.RepoID]
				if repo == nil {
					repo = &Repository{
						ID: watch.RepoID,
					}
					if _, err := sess.Get(repo); err != nil {
						return err
					}
					repoCache[watch.RepoID] = repo
				}

				// Remove issue watches from now unaccssible repositories
				mode, err := accessLevel(sess, watch.UserID, repo)
				if err != nil {
					return err
				}
				has := AccessModeRead <= mode
				if has {
					return nil
				}

				iw := &IssueWatch{
					IsWatching: false,
				}

				_, err = sess.
					Join("INNER", "issue", "`issue`.id = `issue_watch`.issue_id AND `issue`.repo_id = ?", watch.RepoID).
					Cols("is_watching", "updated_unix").
					Where("`issue_watch`.user_id = ?", watch.UserID).
					Update(iw)

				return err

			})
	if err != nil {
		return err
	}

	return sess.Commit()
}
