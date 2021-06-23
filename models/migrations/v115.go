// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"crypto/md5"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"time"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/xorm"
)

func renameExistingUserAvatarName(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()

	type User struct {
		ID        int64  `xorm:"pk autoincr"`
		LowerName string `xorm:"UNIQUE NOT NULL"`
		Avatar    string
	}

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	count, err := x.Count(new(User))
	if err != nil {
		return err
	}
	log.Info("%d User Avatar(s) to migrate ...", count)

	deleteList := make(map[string]struct{})
	start := 0
	migrated := 0
	for {
		if err := sess.Begin(); err != nil {
			return fmt.Errorf("session.Begin: %v", err)
		}
		users := make([]*User, 0, 50)
		if err := sess.Table("user").Asc("id").Limit(50, start).Find(&users); err != nil {
			return fmt.Errorf("select users from id [%d]: %v", start, err)
		}
		if len(users) == 0 {
			_ = sess.Rollback()
			break
		}

		log.Info("select users [%d - %d]", start, start+len(users))
		start += 50

		for _, user := range users {
			oldAvatar := user.Avatar

			if stat, err := os.Stat(filepath.Join(setting.Avatar.Path, oldAvatar)); err != nil || !stat.Mode().IsRegular() {
				if err == nil {
					err = fmt.Errorf("Error: \"%s\" is not a regular file", oldAvatar)
				}
				log.Warn("[user: %s] os.Stat: %v", user.LowerName, err)
				// avatar doesn't exist in the storage
				// no need to move avatar and update database
				// we can just skip this
				continue
			}

			newAvatar, err := copyOldAvatarToNewLocation(user.ID, oldAvatar)
			if err != nil {
				_ = sess.Rollback()
				return fmt.Errorf("[user: %s] %v", user.LowerName, err)
			} else if newAvatar == oldAvatar {
				continue
			}

			user.Avatar = newAvatar
			if _, err := sess.ID(user.ID).Cols("avatar").Update(user); err != nil {
				_ = sess.Rollback()
				return fmt.Errorf("[user: %s] user table update: %v", user.LowerName, err)
			}

			deleteList[filepath.Join(setting.Avatar.Path, oldAvatar)] = struct{}{}
			migrated++
			select {
			case <-ticker.C:
				log.Info(
					"%d/%d (%2.0f%%) User Avatar(s) migrated (%d old avatars to be deleted) in %d batches. %d Remaining ...",
					migrated,
					count,
					float64(migrated)/float64(count)*100,
					len(deleteList),
					int(math.Ceil(float64(migrated)/float64(50))),
					count-int64(migrated))
			default:
			}
		}
		if err := sess.Commit(); err != nil {
			_ = sess.Rollback()
			return fmt.Errorf("commit session: %v", err)
		}
	}

	deleteCount := len(deleteList)
	log.Info("Deleting %d old avatars ...", deleteCount)
	i := 0
	for file := range deleteList {
		if err := util.Remove(file); err != nil {
			log.Warn("util.Remove: %v", err)
		}
		i++
		select {
		case <-ticker.C:
			log.Info(
				"%d/%d (%2.0f%%) Old User Avatar(s) deleted. %d Remaining ...",
				i,
				deleteCount,
				float64(i)/float64(deleteCount)*100,
				deleteCount-i)
		default:
		}
	}

	log.Info("Completed migrating %d User Avatar(s) and deleting %d Old Avatars", count, deleteCount)

	return nil
}

// copyOldAvatarToNewLocation copies oldAvatar to newAvatarLocation
// and returns newAvatar location
func copyOldAvatarToNewLocation(userID int64, oldAvatar string) (string, error) {
	fr, err := os.Open(filepath.Join(setting.Avatar.Path, oldAvatar))
	if err != nil {
		return "", fmt.Errorf("os.Open: %v", err)
	}
	defer fr.Close()

	data, err := ioutil.ReadAll(fr)
	if err != nil {
		return "", fmt.Errorf("ioutil.ReadAll: %v", err)
	}

	newAvatar := fmt.Sprintf("%x", md5.Sum([]byte(fmt.Sprintf("%d-%x", userID, md5.Sum(data)))))
	if newAvatar == oldAvatar {
		return newAvatar, nil
	}

	if err := ioutil.WriteFile(filepath.Join(setting.Avatar.Path, newAvatar), data, 0o666); err != nil {
		return "", fmt.Errorf("ioutil.WriteFile: %v", err)
	}

	return newAvatar, nil
}
