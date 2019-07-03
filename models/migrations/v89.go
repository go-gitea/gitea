// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import "github.com/go-xorm/xorm"

func addOriginalAuthorToIssues(x *xorm.Engine) error {
	// Issue see models/issue.go
	type Issue struct {
		OriginalAuthor   string
		OriginalAuthorID int64
	}

	return x.Sync2(new(Issue))

}

func addOriginalAuthorToIssueComment(x *xorm.Engine) error {
	// Issue see models/issue.go
	type Comment struct {
		OriginalAuthor   string
		OriginalAuthorID int64
	}

	return x.Sync2(new(Comment))

}
