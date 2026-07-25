// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_15

import "gitea.dev/modelmigration/base"

func AddKeyIsVerified(x base.EngineMigration) error {
	type GPGKey struct {
		Verified bool `xorm:"NOT NULL DEFAULT false"`
	}

	return x.Sync(new(GPGKey))
}
