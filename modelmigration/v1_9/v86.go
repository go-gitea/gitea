// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_9

import "gitea.dev/modelmigration/base"

func AddHTTPMethodToWebhook(x base.EngineMigration) error {
	type Webhook struct {
		HTTPMethod string `xorm:"http_method DEFAULT 'POST'"`
	}

	return x.Sync(new(Webhook))
}
