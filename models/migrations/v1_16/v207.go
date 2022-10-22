// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_16 //nolint

import (
	"xorm.io/xorm"
)

func AddWebAuthnCred(x *xorm.Engine) error {
	// NO-OP Don't migrate here - let v210 do this.

	return nil
}
