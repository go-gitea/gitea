// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package auth

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/login"
)

// DeleteLoginSource deletes a LoginSource record in DB.
func DeleteLoginSource(source *login.LoginSource) error {
	count, err := db.DefaultContext().Engine().Count(&models.User{LoginSource: source.ID})
	if err != nil {
		return err
	} else if count > 0 {
		return login.ErrLoginSourceInUse{
			ID: source.ID,
		}
	}

	count, err = db.DefaultContext().Engine().Count(&models.ExternalLoginUser{LoginSourceID: source.ID})
	if err != nil {
		return err
	} else if count > 0 {
		return login.ErrLoginSourceInUse{
			ID: source.ID,
		}
	}

	if registerableSource, ok := source.Cfg.(login.RegisterableSource); ok {
		if err := registerableSource.UnregisterSource(); err != nil {
			return err
		}
	}

	_, err = db.DefaultContext().Engine().ID(source.ID).Delete(new(login.LoginSource))
	return err
}
