// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_16 //nolint

import (
	"xorm.io/xorm"
)

func IncreaseCredentialIDTo410(x *xorm.Engine) error {
	// no-op
	// v208 was completely wrong
	// So now we have to no-op again.

	return nil
}
