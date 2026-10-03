// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"

	"xorm.io/xorm"
)

type hookTypeDefMigration struct {
	ID                     int64  `xorm:"pk autoincr"`
	Name                   string `xorm:"UNIQUE VARCHAR(64) NOT NULL"`
	DisplayName            string `xorm:"VARCHAR(255) NOT NULL"`
	DocsURL                string `xorm:"TEXT"`
	IconMIME               string `xorm:"VARCHAR(64)"`
	IconData               []byte `xorm:"LONGBLOB"`
	IconAsset              string `xorm:"VARCHAR(255)"`
	FormSchema             string `xorm:"LONGTEXT"`
	PayloadJsonnet         string `xorm:"LONGTEXT"`
	RequestJsonnet         string `xorm:"LONGTEXT"`
	RequiresPayloadURL     bool   `xorm:"NOT NULL DEFAULT true"`
	UseAuthorizationHeader string `xorm:"VARCHAR(16)"`
	UseRequestSecret       string `xorm:"VARCHAR(16)"`
	IsBuiltin              bool   `xorm:"NOT NULL DEFAULT false"`
	IsActive               bool   `xorm:"NOT NULL DEFAULT true"`
	CreatedUnix            int64  `xorm:"INDEX created"`
	UpdatedUnix            int64  `xorm:"INDEX updated"`
}

func (*hookTypeDefMigration) TableName() string { return "hook_type" }

// AddHookTypeTableAndWidenWebhookType creates hook_type and widens webhook.type.
func AddHookTypeTableAndWidenWebhookType(_ context.Context, x base.EngineMigration) error {
	if err := x.Sync(new(hookTypeDefMigration)); err != nil {
		return err
	}

	type Webhook struct {
		Type string `xorm:"VARCHAR(64) 'type'"`
	}
	_, err := x.SyncWithOptions(xorm.SyncOptions{
		IgnoreDropIndices: true,
		IgnoreConstrains:  true,
	}, new(Webhook))
	return err
}
