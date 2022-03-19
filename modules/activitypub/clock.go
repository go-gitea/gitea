// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"time"

	"code.gitea.io/gitea/modules/setting"

	"github.com/go-fed/activity/pub"
)

var _ pub.Clock = &Clock{}

// Clock struct
type Clock struct{}

// NewClock function
func NewClock() (c *Clock, err error) {
	c = &Clock{}
	return
}

// Now function
func (c *Clock) Now() time.Time {
	return time.Now().In(setting.DefaultUILocation)
}
