package models

import (
	"code.gitea.io/gitea/modules/setting"
)

type Star struct {
	ID     int64 `xorm:"pk autoincr"`
	UID    int64 `xorm:"UNIQUE(s)"`
	RepoID int64 `xorm:"UNIQUE(s)"`
}

// Star or unstar repository.
func StarRepo(userID, repoID int64, star bool) (err error) {
	if star {
		if IsStaring(userID, repoID) {
			return nil
		}
		if _, err = x.Insert(&Star{UID: userID, RepoID: repoID}); err != nil {
			return err
		} else if _, err = x.Exec("UPDATE `repository` SET num_stars = num_stars + 1 WHERE id = ?", repoID); err != nil {
			return err
		}
		_, err = x.Exec("UPDATE `user` SET num_stars = num_stars + 1 WHERE id = ?", userID)
	} else {
		if !IsStaring(userID, repoID) {
			return nil
		}
		if _, err = x.Delete(&Star{0, userID, repoID}); err != nil {
			return err
		} else if _, err = x.Exec("UPDATE `repository` SET num_stars = num_stars - 1 WHERE id = ?", repoID); err != nil {
			return err
		}
		_, err = x.Exec("UPDATE `user` SET num_stars = num_stars - 1 WHERE id = ?", userID)
	}
	return err
}

// IsStaring checks if user has starred given repository.
func IsStaring(userID, repoID int64) bool {
	has, _ := x.Get(&Star{0, userID, repoID})
	return has
}

func (repo *Repository) GetStargazers(page int) ([]*User, error) {
	users := make([]*User, 0, ItemsPerPage)
	sess := x.
		Limit(ItemsPerPage, (page-1)*ItemsPerPage).
		Where("star.repo_id = ?", repo.ID)
	if setting.UsePostgreSQL {
		sess = sess.Join("LEFT", "star", `"user".id = star.uid`)
	} else {
		sess = sess.Join("LEFT", "star", "user.id = star.uid")
	}
	return users, sess.Find(&users)
}

func (u *User) GetStarredRepos(private bool) (repos []*Repository, err error) {
	sess := x.
		Join("INNER", "star", "star.repo_id = repository.id").
		Where("star.uid = ?", u.ID)

	if !private {
		sess = sess.And("is_private = ?", false)
	}

	err = sess.
		Find(&repos)
	return
}
