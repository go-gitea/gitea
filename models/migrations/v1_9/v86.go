// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package v1_9 //nolint

import (
	"xorm.io/xorm"
)

func AddHTTPMethodToWebhook(x *xorm.Engine) error {
	type Webhook struct {
		HTTPMethod string `xorm:"http_method DEFAULT 'POST'"`
	}

	return x.Sync2(new(Webhook))
}
