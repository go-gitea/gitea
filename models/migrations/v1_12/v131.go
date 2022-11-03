// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_12 //nolint

import (
	"fmt"

	"xorm.io/xorm"
)

func AddSystemWebhookColumn(x *xorm.Engine) error {
	type Webhook struct {
		IsSystemWebhook bool `xorm:"NOT NULL DEFAULT false"`
	}

	if err := x.Sync2(new(Webhook)); err != nil {
		return fmt.Errorf("Sync2: %w", err)
	}
	return nil
}
