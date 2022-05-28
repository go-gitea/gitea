// Copyright 2016 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repo

import (
	"encoding/json"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
)

//Pin Repo or unpin repo
func PinRepo(ownerID, repoID int64, pin bool) error {

	log.Info("pinning %v %v %v", ownerID, repoID, pin)
	var newpinned []int64
	if pin {
		newpinned = append(newpinned, repoID)
	}
	pinnedstring, err := user_model.GetUserSetting(ownerID, "pinned")
	if err == nil {
		var pinned []int64
		err = json.Unmarshal([]byte(pinnedstring), &pinned)
		if err != nil {
			log.Info("E0 ", err)
			return err
		}
		for _, v := range pinned {
			if v != repoID {
				newpinned = append(newpinned, v)
			}
		}
	}
	stringed, jsonerr := json.Marshal(newpinned)
	if jsonerr != nil {
		log.Info("E1 ", err)
		return jsonerr
	}
	err = user_model.SetUserSetting(ownerID, "pinned", string(stringed))
	log.Info("E2 %v", err)
	return err
}
