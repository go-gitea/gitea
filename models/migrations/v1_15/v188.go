// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_15

import "gitea.dev/models/db"

func AddKeyIsVerified(x db.EngineMigration) error {
	type GPGKey struct {
		Verified bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(GPGKey))
}
