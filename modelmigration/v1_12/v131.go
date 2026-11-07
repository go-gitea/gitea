// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_12

import (
	"context"
	"fmt"

	"gitea.dev/modelmigration/base"
)

func AddSystemWebhookColumn(_ context.Context, x base.EngineMigration) error {
	type Webhook struct {
		IsSystemWebhook bool `xorm:"NOT NULL DEFAULT false"`
	}

	if err := x.Sync(new(Webhook)); err != nil {
		return fmt.Errorf("Sync: %w", err)
	}
	return nil
}
