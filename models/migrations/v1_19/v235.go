// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_19 //nolint

import (
	"xorm.io/xorm"
)

func AddIndexForAccessToken(x *xorm.Engine) error {
	type AccessToken struct {
		TokenLastEight string `xorm:"INDEX token_last_eight"`
	}

	return x.Sync(new(AccessToken))
}
