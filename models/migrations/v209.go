// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func increaseCredentialIDTo410(x *xorm.Engine) error {
	// no-op
	// V208 is badly broken
	// So now we have to no-op again.

	return nil
}
