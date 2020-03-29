// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"code.gitea.io/gitea/modules/timeutil"
)

// WebPushSubscription represents a HTML5 Web Push Subscription used for background notifications.
type WebPushSubscription struct {
	ID     int64 `xorm:"pk autoincr"`
	UserID int64 `xorm:"INDEX"`

	Endpoint string
	Auth     string
	P256DH   string

	CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
}
