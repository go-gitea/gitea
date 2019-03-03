// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"fmt"

	"github.com/go-xorm/xorm"
)

func changeU2FCounterType(x *xorm.Engine) error {
	type U2FRegistration struct {
        Counter     uint32         `xorm:"BIGINT"`
	}

	if err := x.Sync2(new(U2FRegistration)); err != nil {
		return fmt.Errorf("Sync2: %v", err)
	}

	return nil
}
