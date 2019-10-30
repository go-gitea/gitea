// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"xorm.io/xorm"
)

func prependRefsHeadsToIssueRefs(x *xorm.Engine) error {
	type Issue struct {
		ID  int64
		Ref string
	}

	sess := x.NewSession()
	defer sess.Close()

	if err := sess.Begin(); err != nil {
		return err
	}

	query := `
		UPDATE issue SET ref = 'refs/heads/' || ref
		WHERE ref IS NOT NULL AND ref <> ''
	`;
	if _, err := sess.Exec(query); err != nil {
		return err
	}

	return sess.Commit()
}
