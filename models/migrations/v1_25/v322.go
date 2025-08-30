// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

func AddOAuth2DeviceFlowSupport(x *xorm.Engine) error {
	type OAuth2Application struct {
		EnableDeviceFlow bool `xorm:"NOT NULL DEFAULT FALSE"`
	}

	if err := x.Sync2(new(OAuth2Application)); err != nil {
		return err
	}

	type OAuth2Device struct {
		ID             int64  `xorm:"pk autoincr"`
		DeviceCode     string `xorm:"-"`
		DeviceCodeHash string `xorm:"UNIQUE"` // sha256 of device code
		DeviceCodeSalt string
		DeviceCodeID   string             `xorm:"INDEX"`
		UserCode       string             `xorm:"INDEX VARCHAR(9)"`
		Application    *OAuth2Application `xorm:"-"`
		ApplicationID  int64              `xorm:"INDEX"`
		Scope          string             `xorm:"TEXT"`
		GrantID        int64
		CreatedUnix    timeutil.TimeStamp `xorm:"created"`
		UpdatedUnix    timeutil.TimeStamp `xorm:"updated"`
		ExpiredUnix    timeutil.TimeStamp
	}

	if err := x.Sync2(new(OAuth2Device)); err != nil {
		return err
	}

	type OAuth2DeviceGrant struct {
		ID            int64              `xorm:"pk autoincr"`
		UserID        int64              `xorm:"INDEX"`
		Application   *OAuth2Application `xorm:"-"`
		ApplicationID int64              `xorm:"INDEX"`
		DeviceID      int64              `xorm:"INDEX"`
		Counter       int64              `xorm:"NOT NULL DEFAULT 1"`
		UserCode      string             `xorm:"VARCHAR(9)"`
		Scope         string             `xorm:"TEXT"`
		Nonce         string             `xorm:"TEXT"`
		CreatedUnix   timeutil.TimeStamp `xorm:"created"`
		UpdatedUnix   timeutil.TimeStamp `xorm:"updated"`
	}
	return x.Sync2(new(OAuth2DeviceGrant))
}
