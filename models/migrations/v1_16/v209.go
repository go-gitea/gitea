// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_16

import (
	"xorm.io/xorm"
)

func IncreaseCredentialIDTo410(x *xorm.Engine) error {
	// no-op
	// v208 was completely wrong
	// So now we have to no-op again.

	return nil
}
