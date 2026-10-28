// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"context"

	"gitea.dev/modelmigration/base"
)

func AddWebAuthnCred(_ context.Context, x base.EngineMigration) error {
	// NO-OP Don't migrate here - let v210 do this.

	return nil
}
