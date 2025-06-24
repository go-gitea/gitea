// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16 //nolint:revive // underscore in migration packages isn't a large issue

import (
	"xorm.io/xorm"
)

func AddWebAuthnCred(x *xorm.Engine) error {
	// NO-OP Don't migrate here - let v210 do this.

	return nil
}
