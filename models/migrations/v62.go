// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addLastUsedPasscodeTOTP(x *xorm.Engine) error {
	type TwoFactor struct {
		LastUsedPasscode string `xorm:"VARCHAR(10)"`
	}

	if err := x.Sync2(new(TwoFactor)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
