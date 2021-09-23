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
func DeleteLoginSource(source *login.Source) error {
	count, err := db.GetEngine(db.DefaultContext).Count(&models.User{LoginSource: source.ID})
	if err != nil {
		return err
	} else if count > 0 {
		return login.ErrSourceInUse{
			ID: source.ID,
		}
	}

	count, err = db.GetEngine(db.DefaultContext).Count(&models.ExternalLoginUser{LoginSourceID: source.ID})
	if err != nil {
		return err
	} else if count > 0 {
		return login.ErrSourceInUse{
			ID: source.ID,
		}
	}

	if registerableSource, ok := source.Cfg.(login.RegisterableSource); ok {
		if err := registerableSource.UnregisterSource(); err != nil {
			return err
		}
	}

	_, err = db.GetEngine(db.DefaultContext).ID(source.ID).Delete(new(login.Source))
	return err
}
