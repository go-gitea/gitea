// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"
	"fmt"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/auth"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/services/audit"
)

// DeleteSource deletes a AuthSource record in DB.
func DeleteSource(ctx context.Context, doer *user_model.User, source *auth.Source) error {
	count, err := db.GetEngine(ctx).Count(&user_model.User{LoginSource: source.ID})
	if err != nil {
		return err
	} else if count > 0 {
		return auth.ErrSourceInUse{
			ID: source.ID,
		}
	}

	count, err = db.GetEngine(ctx).Count(&user_model.ExternalLoginUser{LoginSourceID: source.ID})
	if err != nil {
		return err
	} else if count > 0 {
		return auth.ErrSourceInUse{
			ID: source.ID,
		}
	}

	if registerableSource, ok := source.Cfg.(auth.RegisterableSource); ok {
		if err := registerableSource.UnregisterSource(); err != nil {
			return err
		}
	}

	if _, err = db.GetEngine(ctx).ID(source.ID).Delete(new(auth.Source)); err != nil {
		return err
	}

	audit.Record(ctx, audit_model.SystemAuthenticationSourceRemove, doer, nil,
		fmt.Sprintf("Removed authentication source %s.", source.Name), "auth_source", source.Name)

	return nil
}
