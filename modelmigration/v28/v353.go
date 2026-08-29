// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v28

import (
	"context"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modules/timeutil"
)

// AddOAuth2DeviceAuthorizationTable creates the device authorization table.
func AddOAuth2DeviceAuthorizationTable(_ context.Context, x base.EngineMigration) error {
	type oauth2DeviceAuthorization struct {
		ID                  int64 `xorm:"pk autoincr"`
		ApplicationID       int64 `xorm:"INDEX"`
		UserID              int64 `xorm:"INDEX"`
		GrantID             int64
		DeviceCodeHash      string `xorm:"unique"`
		UserCode            string `xorm:"unique"`
		Scope               string `xorm:"TEXT"`
		Status              string `xorm:"NOT NULL"`
		PollIntervalSeconds int64  `xorm:"NOT NULL DEFAULT 5"`
		LastPolledUnix      timeutil.TimeStamp
		ExpiresAtUnix       timeutil.TimeStamp
		CreatedUnix         timeutil.TimeStamp `xorm:"created"`
		UpdatedUnix         timeutil.TimeStamp `xorm:"updated"`
	}

	return x.Sync(new(oauth2DeviceAuthorization))
}
