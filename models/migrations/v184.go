// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func dropWebhookColumns(x *xorm.Engine) error {
	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}
	if err := dropTableColumns(sess, "webhook", "signature", "is_ssl"); err != nil {
		return err
	}
	if err := dropTableColumns(sess, "hook_task", "typ", "url", "signature", "http_method", "content_type", "is_ssl"); err != nil {
		return err
	}

	return sess.Commit()
}
