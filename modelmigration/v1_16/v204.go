// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import "gitea.dev/modelmigration/base"

func AddSSHKeyIsVerified(x base.EngineMigration) error {
	type PublicKey struct {
		Verified bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(PublicKey))
}
