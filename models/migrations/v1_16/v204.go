// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import "gitea.dev/models/db"

func AddSSHKeyIsVerified(x db.EngineMigration) error {
	type PublicKey struct {
		Verified bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(PublicKey))
}
