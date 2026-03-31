// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import "xorm.io/xorm"

// AddSecretSaltToTwoFactor adds a per-user salt column for TOTP secrets.
func AddSecretSaltToTwoFactor(x *xorm.Engine) error {
	type TwoFactor struct {
		SecretSalt string
		SecretAlgo string
	}

	if err := x.Sync(new(TwoFactor)); err != nil {
		return err
	}

	_, err := x.Exec("UPDATE two_factor SET secret_algo = 'md5' WHERE secret_algo = '' OR secret_algo IS NULL")
	return err
}
