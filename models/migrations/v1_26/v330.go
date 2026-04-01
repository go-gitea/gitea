// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_26

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

// AddOAuth2DeviceAuthorizationTableAndFixBuiltinNativeApps creates the device authorization table
// and marks built-in loopback/native apps as public clients.
func AddOAuth2DeviceAuthorizationTableAndFixBuiltinNativeApps(x *xorm.Engine) error {
	type oauth2Application struct {
		ID                 int64  `xorm:"pk autoincr"`
		ClientID           string `xorm:"unique"`
		ConfidentialClient bool   `xorm:"NOT NULL DEFAULT TRUE"`
	}

	type oauth2DeviceAuthorization struct {
		ID                  int64              `xorm:"pk autoincr"`
		ApplicationID       int64              `xorm:"INDEX"`
		UserID              int64              `xorm:"INDEX"`
		GrantID             int64              `xorm:"INDEX"`
		DeviceCodeHash      string             `xorm:"unique"`
		UserCode            string             `xorm:"unique"`
		Scope               string             `xorm:"TEXT"`
		Status              string             `xorm:"INDEX NOT NULL"`
		PollIntervalSeconds int64              `xorm:"NOT NULL DEFAULT 5"`
		LastPolledUnix      timeutil.TimeStamp `xorm:"INDEX"`
		ExpiresAtUnix       timeutil.TimeStamp `xorm:"INDEX"`
		CreatedUnix         timeutil.TimeStamp `xorm:"created"`
		UpdatedUnix         timeutil.TimeStamp `xorm:"updated"`
	}

	if err := x.Sync(new(oauth2DeviceAuthorization)); err != nil {
		return err
	}

	// Sync the oauth2Application table to ensure the client_id column exists.
	// Coming from older versions there are potential situations where a column
	// was missed in a migration, and only added during the SyncAllTables() call
	if err := x.Sync(new(oauth2Application)); err != nil {
		return err
	}

	_, err := x.In("client_id", []string{
		"a4792ccc-144e-407e-86c9-5e7d8d9c3269",
		"e90ee53c-94e2-48ac-9358-a874fb9e0662",
		"d57cb8c4-630c-4168-8324-ec79935e18d4",
	}).Cols("confidential_client").Update(&oauth2Application{ConfidentialClient: false})
	return err
}
