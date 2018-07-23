// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
<<<<<<< HEAD
	"fmt"

	"code.gitea.io/gitea/modules/util"
=======
	"code.gitea.io/gitea/modules/setting"
>>>>>>> origin/master

	"github.com/go-xorm/xorm"
)

<<<<<<< HEAD
func addPullRequestRebaseWithMergeCommit(x *xorm.Engine) error {
	// RepoUnit describes all units of a repository
	type RepoUnit struct {
		ID          int64
		RepoID      int64                  `xorm:"INDEX(s)"`
		Type        int                    `xorm:"INDEX(s)"`
		Config      map[string]interface{} `xorm:"JSON"`
		CreatedUnix util.TimeStamp         `xorm:"INDEX CREATED"`
=======
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

	accessLevel := func(userID int64, repo *Repository) (int, error) {
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
		if has, err := x.Get(a); !has || err != nil {
			return mode, err
		}
		return a.Mode, nil
>>>>>>> origin/master
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

<<<<<<< HEAD
	//Updating existing issue units
	units := make([]*RepoUnit, 0, 100)
	if err := sess.Where("`type` = ?", V16UnitTypePRs).Find(&units); err != nil {
		return fmt.Errorf("Query repo units: %v", err)
	}
	for _, unit := range units {
		if unit.Config == nil {
			unit.Config = make(map[string]interface{})
		}
		if _, ok := unit.Config["AllowRebaseMergeCommit"]; !ok {
			unit.Config["AllowRebaseMergeCommit"] = true
		}
		if _, err := sess.ID(unit.ID).Cols("config").Update(unit); err != nil {
			return err
		}
	}
	return sess.Commit()

=======
	repoCache := make(map[int64]*Repository)
	err := x.BufferSize(setting.IterateBufferSize).Iterate(new(Watch),
		func(idx int, bean interface{}) error {
			watch := bean.(*Watch)

			repo := repoCache[watch.RepoID]
			if repo == nil {
				repo = &Repository{
					ID: watch.RepoID,
				}
				if _, err := x.Get(repo); err != nil {
					return err
				}
				repoCache[watch.RepoID] = repo
			}

			// Remove watches from now unaccessible repositories
			mode, err := accessLevel(watch.UserID, repo)
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
	err = x.BufferSize(setting.IterateBufferSize).
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
					if _, err := x.Get(repo); err != nil {
						return err
					}
					repoCache[watch.RepoID] = repo
				}

				// Remove issue watches from now unaccssible repositories
				mode, err := accessLevel(watch.UserID, repo)
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
>>>>>>> origin/master
}
