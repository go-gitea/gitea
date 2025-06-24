// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12 //nolint:revive // underscore in migration packages isn't a large issue

import (
	"fmt"

	"xorm.io/xorm"
)

func AddSystemWebhookColumn(x *xorm.Engine) error {
	type Webhook struct {
		IsSystemWebhook bool `xorm:"NOT NULL DEFAULT false"`
	}

	if err := x.Sync(new(Webhook)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
