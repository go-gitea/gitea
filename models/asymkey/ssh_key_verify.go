// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"bytes"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/modules/log"

	"github.com/42wim/sshsig"
)

// VerifySSHKey marks a SSH key as verified
func VerifySSHKey(ownerID int64, fingerprint, token, signature string) (string, error) {
	ctx, committer, err := db.TxContext(db.DefaultContext)
	if err != nil {
		return "", err
	}
	defer committer.Close()

	key := new(PublicKey)

	has, err := db.GetEngine(ctx).Where("owner_id = ? AND fingerprint = ?", ownerID, fingerprint).Get(key)
	if err != nil {
		return "", err
	} else if !has {
		return "", ErrKeyNotExist{}
	}

	err = sshsig.Verify(bytes.NewBuffer([]byte(token)), []byte(signature), []byte(key.Content), "gitea")
	if err != nil {
		// edge case for Windows based shells that will add CR LF if piped to ssh-keygen command
		// see https://github.com/PowerShell/PowerShell/issues/5974
		if sshsig.Verify(bytes.NewBuffer([]byte(token+"\r\n")), []byte(signature), []byte(key.Content), "gitea") != nil {
			log.Error("Unable to validate token signature. Error: %v", err)
			return "", ErrSSHInvalidTokenSignature{
				Fingerprint: key.Fingerprint,
			}
		}
	}

	key.Verified = true
	if _, err := db.GetEngine(ctx).ID(key.ID).Cols("verified").Update(key); err != nil {
		return "", err
	}

	if err := committer.Commit(); err != nil {
		return "", err
	}

	return key.Fingerprint, nil
}
