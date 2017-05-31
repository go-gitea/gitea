// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"github.com/go-xorm/xorm"
)

func addLoginSourceLdapPublicSSHKeySyncEnabled(x *xorm.Engine) error {
	// LoginSource see models/login_source.go
	type LoginSource struct {
		IsLdapPublicSSHKeySyncEnabled bool `xorm:"INDEX NOT NULL DEFAULT false"`
	}

	if err := x.Sync2(new(LoginSource)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}
	return nil
}
