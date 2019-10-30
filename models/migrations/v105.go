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

	const batchSize = 100
	count := 0
	for start := 0; ; start += batchSize {
		issues := make([]*Issue, 0, batchSize)
		if err := sess.Limit(batchSize, start).Asc("id").Find(&issues); err != nil {
			return err
		}

		if len(issues) == 0 {
			break
		}

		for _, issue := range issues {
			issue.Ref = "refs/heads/" + issue.Ref
			if _, err := sess.ID(issue.ID).Cols("ref").Update(issue); err != nil {
				return err
			}
		}

		count++
		if count >= 1000 {
			if err := sess.Commit(); err != nil {
				return err
			}
			if err := sess.Begin(); err != nil {
				return err
			}
			count = 0
		}
	}
	return sess.Commit()
}
