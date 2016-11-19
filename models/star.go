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

	if err := sess.Begin(); err != nil {
		return err
	}

	if star {
		if IsStaring(userID, repoID) {
			sess.Rollback()
			return nil
		}

		if _, err := sess.Insert(&Star{UID: userID, RepoID: repoID}); err != nil {
			sess.Rollback()
			return err
		}
		if _, err := x.Exec("UPDATE `repository` SET num_stars = num_stars + 1 WHERE id = ?", repoID); err != nil {
			sess.Rollback()
			return err
		}
		if _, err := x.Exec("UPDATE `user` SET num_stars = num_stars + 1 WHERE id = ?", userID); err != nil {
			sess.Rollback()
			return err
		}
	} else {
		if !IsStaring(userID, repoID) {
			sess.Rollback()
			return nil
		}

		if _, err := x.Delete(&Star{0, userID, repoID}); err != nil {
			sess.Rollback()
			return err
		}
		if _, err := x.Exec("UPDATE `repository` SET num_stars = num_stars - 1 WHERE id = ?", repoID); err != nil {
			sess.Rollback()
			return err
		}
		if _, err := x.Exec("UPDATE `user` SET num_stars = num_stars - 1 WHERE id = ?", userID); err != nil {
			sess.Rollback()
			return err
		}
	}

	return sess.Commit()
}

// IsStaring checks if user has starred given repository.
func IsStaring(userID, repoID int64) bool {
	has, _ := x.Get(&Star{0, userID, repoID})
	return has
}

// GetStargazers returns the users that starred the repo.
func (repo *Repository) GetStargazers(page int) ([]*User, error) {
	users := make([]*User, 0, ItemsPerPage)
	err := x.
		Limit(ItemsPerPage, (page-1)*ItemsPerPage).
		Where("star.repo_id = ?", repo.ID).
		Join("LEFT", "star", "`user`.id = star.uid").
		Find(&users)
	return users, err
}

// GetStarredRepos returns the repos the user starred.
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
