// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package packages

import (
	"context"
	"errors"
	"fmt"

	user_model "gitea.dev/models/user"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/util"
)

// GetOrCreateKeyPair gets the owner's key pair used to sign repository files,
// generating and storing it if it does not exist yet.
func GetOrCreateKeyPair(ctx context.Context, ownerID int64, settingKeyPriv, settingKeyPub string, generate func() (priv, pub string, err error)) (string, string, error) {
	priv, pub, err := getKeyPair(ctx, ownerID, settingKeyPriv, settingKeyPub)
	if err != nil || (priv != "" && pub != "") {
		return priv, pub, err
	}

	err = globallock.LockAndDo(ctx, fmt.Sprintf("pkg-keypair-%s-%d", settingKeyPriv, ownerID), func(ctx context.Context) error {
		priv, pub, err = getKeyPair(ctx, ownerID, settingKeyPriv, settingKeyPub) // re-read inside the lock, another request may have created it
		if err != nil || (priv != "" && pub != "") {
			return err
		}

		if priv, pub, err = generate(); err != nil {
			return err
		}

		if err := user_model.SetUserSetting(ctx, ownerID, settingKeyPriv, priv); err != nil {
			return err
		}
		return user_model.SetUserSetting(ctx, ownerID, settingKeyPub, pub)
	})
	if err != nil {
		return "", "", err
	}
	return priv, pub, nil
}

func getKeyPair(ctx context.Context, ownerID int64, settingKeyPriv, settingKeyPub string) (string, string, error) {
	priv, err := user_model.GetSetting(ctx, ownerID, settingKeyPriv)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		return "", "", err
	}

	pub, err := user_model.GetSetting(ctx, ownerID, settingKeyPub)
	if err != nil && !errors.Is(err, util.ErrNotExist) {
		return "", "", err
	}

	return priv, pub, nil
}
