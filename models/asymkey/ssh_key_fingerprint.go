// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"fmt"

	"code.gitea.io/gitea/models/db"

	"golang.org/x/crypto/ssh"
	"xorm.io/builder"
)

// The database is used in checkKeyFingerprint however most of these functions probably belong in a module

// checkKeyFingerprint only checks if key fingerprint has been used as public key,
// it is OK to use same key as deploy key for multiple repositories/users.
func checkKeyFingerprint(ctx context.Context, fingerprint string) error {
	has, err := db.Exist[PublicKey](ctx, builder.Eq{"fingerprint": fingerprint})
	if err != nil {
		return err
	} else if has {
		return ErrKeyAlreadyExist{0, fingerprint, ""}
	}
	return nil
}

func calcFingerprintNative(publicKeyContent string) (string, error) {
	// Calculate fingerprint.
	pk, _, _, _, err := ssh.ParseAuthorizedKey([]byte(publicKeyContent))
	if err != nil {
		return "", err
	}
	return ssh.FingerprintSHA256(pk), nil
}

// CalcFingerprint calculate public key's fingerprint
func CalcFingerprint(publicKeyContent string) (string, error) {
	fp, err := calcFingerprintNative(publicKeyContent)
	if err != nil {
		if IsErrKeyUnableVerify(err) {
			return "", err
		}
		return "", fmt.Errorf("CalcFingerprint: %w", err)
	}
	return fp, nil
}
