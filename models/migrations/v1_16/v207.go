// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"xorm.io/xorm"
)

func AddWebAuthnCred(x *xorm.Engine) error {
	// NO-OP Don't migrate here - let v210 do this.

	return nil
}
