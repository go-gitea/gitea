// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"xorm.io/xorm"
)

func addKeypairToPushMirror(x *xorm.Engine) error {
	type PushMirror struct {
		PublicKey  string
		PrivateKey string
	}

	if err := x.Sync2(new(PushMirror)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
