// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"errors"

	asymkey_model "gitea.dev/models/asymkey"
	audit_model "gitea.dev/models/audit"
	auth_model "gitea.dev/models/auth"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/services/audit"

	"xorm.io/builder"
)

var ErrGPGKeyExternallyManaged = errors.New("GPG key is externally managed for this user")

func SourceManagesGPGKeys(ctx context.Context, sourceID int64) (bool, error) {
	if sourceID == 0 {
		return false, nil
	}
	source, err := auth_model.GetSourceByID(ctx, sourceID)
	if auth_model.IsErrSourceNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	provider, ok := source.Cfg.(auth_model.GPGKeyProvider)
	return ok && provider.ProvidesGPGKeys(), nil
}

// DeleteGPGKey handles user-initiated deletion through the settings page and API.
func DeleteGPGKey(ctx context.Context, user *user_model.User, id int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		key, err := asymkey_model.GetGPGKeyForUserByID(ctx, user.ID, id)
		if asymkey_model.IsErrGPGKeyNotExist(err) {
			return nil
		}
		if err != nil {
			return err
		}
		sourceID := key.LoginSourceID
		if key.PrimaryKeyID != "" {
			parent, found, err := db.Get[asymkey_model.GPGKey](ctx, builder.Eq{"key_id": key.PrimaryKeyID, "primary_key_id": "", "owner_id": user.ID})
			if err != nil {
				return err
			}
			if found {
				sourceID = parent.LoginSourceID
			}
		}
		managed, err := SourceManagesGPGKeys(ctx, sourceID)
		if err != nil {
			return err
		}
		if managed {
			return ErrGPGKeyExternallyManaged
		}
		if err := asymkey_model.DeleteGPGKey(ctx, user, id); err != nil {
			return err
		}
		audit.Record(ctx, audit_model.UserKeyGPGRemove, user, "gpg_key_id", key.KeyID)
		return nil
	})
}
