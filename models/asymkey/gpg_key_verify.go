// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"strconv"
	"time"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/base"
	"code.gitea.io/gitea/modules/log"
)

//   __________________  ________   ____  __.
//  /  _____/\______   \/  _____/  |    |/ _|____ ___.__.
// /   \  ___ |     ___/   \  ___  |      <_/ __ <   |  |
// \    \_\  \|    |   \    \_\  \ |    |  \  ___/\___  |
//  \______  /|____|    \______  / |____|__ \___  > ____|
//         \/                  \/          \/   \/\/
// ____   ____           .__  _____
// \   \ /   /___________|__|/ ____\__.__.
//  \   Y   // __ \_  __ \  \   __<   |  |
//   \     /\  ___/|  | \/  ||  |  \___  |
//    \___/  \___  >__|  |__||__|  / ____|
//               \/                \/

// This file provides functions relating verifying gpg keys

// VerifyGPGKey marks a GPG key as verified
func VerifyGPGKey(ctx context.Context, ownerID int64, keyID, token, signature string) (string, error) {
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return "", err
	}
	defer committer.Close()

	key := new(GPGKey)

	has, err := db.GetEngine(ctx).Where("owner_id = ? AND key_id = ?", ownerID, keyID).Get(key)
	if err != nil {
		return "", err
	} else if !has {
		return "", ErrGPGKeyNotExist{}
	}

	if err := key.LoadSubKeys(ctx); err != nil {
		return "", err
	}

	sig, err := ExtractSignature(signature)
	if err != nil {
		return "", ErrGPGInvalidTokenSignature{
			ID:      key.KeyID,
			Wrapped: err,
		}
	}

	signer, err := hashAndVerifyWithSubKeys(sig, token, key)
	if err != nil {
		return "", ErrGPGInvalidTokenSignature{
			ID:      key.KeyID,
			Wrapped: err,
		}
	}
	if signer == nil {
		signer, err = hashAndVerifyWithSubKeys(sig, token+"\n", key)
		if err != nil {
			return "", ErrGPGInvalidTokenSignature{
				ID:      key.KeyID,
				Wrapped: err,
			}
		}
	}
	if signer == nil {
		signer, err = hashAndVerifyWithSubKeys(sig, token+"\n\n", key)
		if err != nil {
			return "", ErrGPGInvalidTokenSignature{
				ID:      key.KeyID,
				Wrapped: err,
			}
		}
	}

	if signer == nil {
		log.Error("Unable to validate token signature. Error: %v", err)
		return "", ErrGPGInvalidTokenSignature{
			ID: key.KeyID,
		}
	}

	if signer.PrimaryKeyID != key.KeyID && signer.KeyID != key.KeyID {
		return "", ErrGPGKeyNotExist{}
	}

	key.Verified = true
	if _, err := db.GetEngine(ctx).ID(key.ID).SetExpr("verified", true).Update(new(GPGKey)); err != nil {
		return "", err
	}

	if err := committer.Commit(); err != nil {
		return "", err
	}

	return key.KeyID, nil
}

// VerificationToken returns token for the user that will be valid in minutes (time)
func VerificationToken(user *user_model.User, minutes int) string {
	return base.EncodeSha256(
		time.Now().Truncate(1*time.Minute).Add(time.Duration(minutes)*time.Minute).Format(
			time.RFC1123Z) + ":" +
			user.CreatedUnix.Format(time.RFC1123Z) + ":" +
			user.Name + ":" +
			user.Email + ":" +
			strconv.FormatInt(user.ID, 10))
}
