// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package auth

import (
	"context"

	audit_model "gitea.dev/models/audit"
	"gitea.dev/models/auth"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/services/audit"
)

// CreateSource creates a AuthSource record in DB.
func CreateSource(ctx context.Context, source *auth.Source) error {
	if err := auth.CreateSource(ctx, source); err != nil {
		return err
	}

	audit.Record(ctx, audit_model.SystemAuthenticationSourceAdd, nil,
		"auth_source", source.Name, "auth_source_type", source.Type.String(), "is_active", source.IsActive)

	return nil
}

// UpdateSource updates a AuthSource record in DB.
func UpdateSource(ctx context.Context, source *auth.Source) error {
	if err := auth.UpdateSource(ctx, source); err != nil {
		return err
	}

	audit.Record(ctx, audit_model.SystemAuthenticationSourceUpdate, nil,
		"auth_source", source.Name, "auth_source_type", source.Type.String(), "is_active", source.IsActive)

	return nil
}

// DeleteSource deletes a AuthSource record in DB.
func DeleteSource(ctx context.Context, source *auth.Source) error {
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

	audit.Record(ctx, audit_model.SystemAuthenticationSourceRemove, nil, "auth_source", source.Name)

	return nil
}
