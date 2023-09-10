// Copyright 2020 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_14 //nolint

import (
	"fmt"
	"strconv"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"xorm.io/xorm"
)

func UpdateCodeCommentReplies(x *xorm.Engine) error {
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

	if err := x.Sync(new(Comment)); err != nil {
		return err
	}

	sqlSelect := `SELECT comment.id as id, first.commit_sha as commit_sha, first.patch as patch, first.invalidated as invalidated`
	sqlTail := ` FROM comment INNER JOIN (
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
		AND comment.commit_sha != first.commit_sha`

	var (
		sqlCmd    string
		start     = 0
		batchSize = 100
		sess      = x.NewSession()
	)
	defer sess.Close()
	for {
		if err := sess.Begin(); err != nil {
			return err
		}

		if setting.Database.Type.IsMSSQL() {
			if _, err := sess.Exec(sqlSelect + " INTO #temp_comments" + sqlTail); err != nil {
				log.Error("unable to create temporary table")
				return err
			}
		}

		comments := make([]*Comment, 0, batchSize)

		switch {
		case setting.Database.Type.IsMySQL():
			sqlCmd = sqlSelect + sqlTail + " LIMIT " + strconv.Itoa(batchSize) + ", " + strconv.Itoa(start)
		case setting.Database.Type.IsPostgreSQL():
			fallthrough
		case setting.Database.Type.IsSQLite3():
			sqlCmd = sqlSelect + sqlTail + " LIMIT " + strconv.Itoa(batchSize) + " OFFSET " + strconv.Itoa(start)
		case setting.Database.Type.IsMSSQL():
			sqlCmd = "SELECT TOP " + strconv.Itoa(batchSize) + " * FROM #temp_comments WHERE " +
				"(id NOT IN ( SELECT TOP " + strconv.Itoa(start) + " id FROM #temp_comments ORDER BY id )) ORDER BY id"
		default:
			return fmt.Errorf("Unsupported database type")
		}

		if err := sess.SQL(sqlCmd).Find(&comments); err != nil {
			log.Error("failed to select: %v", err)
			return err
		}

		for _, comment := range comments {
			if _, err := sess.Table("comment").ID(comment.ID).Cols("commit_sha", "patch", "invalidated").Update(comment); err != nil {
				log.Error("failed to update comment[%d]: %v %v", comment.ID, comment, err)
				return err
			}
		}

		start += len(comments)

		if err := sess.Commit(); err != nil {
			return err
		}
		if len(comments) < batchSize {
			break
		}
	}

	return nil
}
