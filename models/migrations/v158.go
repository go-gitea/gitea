// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package migrations

import (
	"code.gitea.io/gitea/modules/log"

	"xorm.io/xorm"
)

func updateCodeCommentReplies(x *xorm.Engine) error {
	type Comment struct {
		ID          int64  `xorm:"pk autoincr"`
		CommitSHA   string `xorm:"VARCHAR(40)"`
		Patch       string `xorm:"TEXT patch"`
		Invalidated bool

		// Not extracted but used in the below query
		Type     int   `xorm:"INDEX"`
		Line     int64 // - previous line / + proposed line
		TreePath string
		ReviewID int64 `xorm:"index"`
	}

	if err := x.Sync2(new(Comment)); err != nil {
		return err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	var start = 0
	var batchSize = 100
	for {
		var comments = make([]*Comment, 0, batchSize)
		if err := sess.SQL(`SELECT comment.id as id, first.commit_sha as commit_sha, first.patch as patch, first.invalidated as invalidated
		FROM comment INNER JOIN (
			SELECT C.id, C.review_id, C.line, C.tree_path, C.patch, C.commit_sha, C.invalidated
			FROM comment AS C 
			WHERE C.type = 21
				AND C.created_unix = 
					(SELECT MIN(comment.created_unix)
						FROM comment
						WHERE comment.review_id = C.review_id
						AND comment.type = 21
						AND comment.line = C.line
						AND comment.tree_path = C.tree_path)
			) AS first
			ON comment.review_id = first.review_id
				AND comment.tree_path = first.tree_path AND comment.line = first.line
		WHERE comment.type = 21
			AND comment.id != first.id
			AND comment.commit_sha != first.commit_sha`).Limit(batchSize, start).Find(&comments); err != nil {
			log.Error("failed to select: %v", err)
			return err
		}

		for _, comment := range comments {
			if _, err := sess.Table("comment").Cols("commit_sha", "patch", "invalidated").Update(comment); err != nil {
				log.Error("failed to update comment[%d]: %v %v", comment.ID, comment, err)
				return err
			}
		}

		start += len(comments)

		if len(comments) < batchSize {
			break
		}
	}

	return sess.Commit()
}
