// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

// Star represents a starred repo by an user.
type Star struct {
	ID     int64 `xorm:"pk autoincr"`
	UID    int64 `xorm:"UNIQUE(s)"`
	RepoID int64 `xorm:"UNIQUE(s)"`
}

// StarRepo or unstar repository.
func StarRepo(userID, repoID int64, star bool) error {
	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	if star {
		if isStaring(sess, userID, repoID) {
			return nil
		}

		if _, err := sess.Insert(&Star{UID: userID, RepoID: repoID}); err != nil {
			return err
		}
		if _, err := sess.Exec("UPDATE `repository` SET num_stars = num_stars + 1 WHERE id = ?", repoID); err != nil {
			return err
		}
		if _, err := sess.Exec("UPDATE `user` SET num_stars = num_stars + 1 WHERE id = ?", userID); err != nil {
			return err
		}
	} else {
		if !isStaring(sess, userID, repoID) {
			return nil
		}

		if _, err := sess.Delete(&Star{0, userID, repoID}); err != nil {
			return err
		}
		if _, err := sess.Exec("UPDATE `repository` SET num_stars = num_stars - 1 WHERE id = ?", repoID); err != nil {
			return err
		}
		if _, err := sess.Exec("UPDATE `user` SET num_stars = num_stars - 1 WHERE id = ?", userID); err != nil {
			return err
		}
	}

	return sess.Commit()
}

// IsStaring checks if user has starred given repository.
func IsStaring(userID, repoID int64) bool {
	return isStaring(x, userID, repoID)
}

func isStaring(e Engine, userID, repoID int64) bool {
	has, _ := e.Get(&Star{0, userID, repoID})
	return has
}

// GetStargazers returns the users that starred the repo.
func (repo *Repository) GetStargazers(page int) ([]*User, error) {
	users := make([]*User, 0, ItemsPerPage)
	sess := x.Where("star.repo_id = ?", repo.ID).
		Join("LEFT", "star", "`user`.id = star.uid")
	if page > 0 {
		sess = sess.Limit(ItemsPerPage, (page-1)*ItemsPerPage)
	}
	return users, sess.Find(&users)
}

// GetStarredRepos returns the repos the user starred.
func (u *User) GetStarredRepos(private bool, page, pageSize int, orderBy string) (repos RepositoryList, err error) {
	if len(orderBy) == 0 {
		orderBy = "updated_unix DESC"
	}
	sess := x.
		Join("INNER", "star", "star.repo_id = repository.id").
		Where("star.uid = ?", u.ID).
		OrderBy(orderBy)

	if !private {
		sess = sess.And("is_private = ?", false)
	}

	if page <= 0 {
		page = 1
	}
	sess.Limit(pageSize, (page-1)*pageSize)

	repos = make([]*Repository, 0, pageSize)

	if err = sess.Find(&repos); err != nil {
		return
	}

	if err = repos.loadAttributes(x); err != nil {
		return
	}

	return
}

// GetStarredRepoCount returns the numbers of repo the user starred.
func (u *User) GetStarredRepoCount(private bool) (int64, error) {
	sess := x.
		Join("INNER", "star", "star.repo_id = repository.id").
		Where("star.uid = ?", u.ID)

	if !private {
		sess = sess.And("is_private = ?", false)
	}

	return sess.Count(&Repository{})
}
