// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import "fmt"

// Watch is connection request for receiving repository notification.
type Watch struct {
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"UNIQUE(watch)"`
	RepoID int64 `xorm:"UNIQUE(watch)"`
}

func isWatching(e Engine, userID, repoID int64) bool {
	has, _ := e.Get(&Watch{UserID: userID, RepoID: repoID})
	return has
}

// IsWatching checks if user has watched given repository.
func IsWatching(userID, repoID int64) bool {
	return isWatching(x, userID, repoID)
}

func watchRepo(e Engine, userID, repoID int64, watch bool) (err error) {
	if watch {
		if isWatching(e, userID, repoID) {
			return nil
		}
		if _, err = e.Insert(&Watch{RepoID: repoID, UserID: userID}); err != nil {
			return err
		}
		_, err = e.Exec("UPDATE `repository` SET num_watches = num_watches + 1 WHERE id = ?", repoID)
	} else {
		if !isWatching(e, userID, repoID) {
			return nil
		}
		if _, err = e.Delete(&Watch{0, userID, repoID}); err != nil {
			return err
		}
		_, err = e.Exec("UPDATE `repository` SET num_watches = num_watches - 1 WHERE id = ?", repoID)
	}
	return err
}

// WatchRepo watch or unwatch repository.
func WatchRepo(userID, repoID int64, watch bool) (err error) {
	return watchRepo(x, userID, repoID, watch)
}

func getWatchers(e Engine, repoID int64) ([]*Watch, error) {
	watches := make([]*Watch, 0, 10)
	return watches, e.Find(&watches, &Watch{RepoID: repoID})
}

// GetWatchers returns all watchers of given repository.
func GetWatchers(repoID int64) ([]*Watch, error) {
	return getWatchers(x, repoID)
}

// GetWatchers returns range of users watching given repository.
func (repo *Repository) GetWatchers(page int) ([]*User, error) {
	users := make([]*User, 0, ItemsPerPage)
	sess := x.Where("watch.repo_id=?", repo.ID).
		Join("LEFT", "watch", "`user`.id=`watch`.user_id")
	if page > 0 {
		sess = sess.Limit(ItemsPerPage, (page-1)*ItemsPerPage)
	}
	return users, sess.Find(&users)
}

func notifyWatchers(e Engine, act *Action) error {
	// Add feeds for user self and all watchers.
	watches, err := getWatchers(e, act.RepoID)
	if err != nil {
		return fmt.Errorf("get watchers: %v", err)
	}

	// Add feed for actioner.
	act.UserID = act.ActUserID
	if _, err = e.InsertOne(act); err != nil {
		return fmt.Errorf("insert new actioner: %v", err)
	}

	for i := range watches {
		if act.ActUserID == watches[i].UserID {
			continue
		}

		act.ID = 0
		act.UserID = watches[i].UserID
		if _, err = e.InsertOne(act); err != nil {
			return fmt.Errorf("insert new action: %v", err)
		}
	}
	return nil
}

// NotifyWatchers creates batch of actions for every watcher.
func NotifyWatchers(act *Action) error {
	return notifyWatchers(x, act)
}
