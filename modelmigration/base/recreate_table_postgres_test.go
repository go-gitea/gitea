// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package base_test

import (
	"testing"

	"gitea.dev/modelmigration/base"
	"gitea.dev/modelmigration/migrationtest"
	issues_model "gitea.dev/models/issues"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/require"
	"xorm.io/xorm/names"
)

// Test_RecreateTable_ReviewTypeVarchar_Postgres reproduces go-gitea/gitea#40439:
// on PostgreSQL, `doctor recreate-table` failed for the review table when the
// existing column `review.type` is VARCHAR(255) (old schema, see #22275) while
// the current Review bean defines it as an integer (ReviewType).
// The INSERT...SELECT copy then failed with:
// pq: column "type" is of type integer but expression is of type character varying (42804)
func Test_RecreateTable_ReviewTypeVarchar_Postgres(t *testing.T) {
	if !setting.Database.Type.IsPostgreSQL() {
		// The failure is PostgreSQL-specific (strict INSERT...SELECT typing);
		// SQLite accepts the copy, so only run this on PostgreSQL.
		t.Skip("Test_RecreateTable_ReviewTypeVarchar_Postgres is PostgreSQL-only")
	}

	x, deferable := migrationtest.PrepareTestEnv(t, 0)
	defer deferable()
	if x == nil || t.Failed() {
		t.Skip("PrepareTestEnv did not yield a usable engine")
	}
	x.SetMapper(names.GonicMapper{})

	// Create the review table with the current bean definition (type INTEGER).
	require.NoError(t, x.Sync(new(issues_model.Review)))

	// Downgrade the type column to the old VARCHAR(255) schema.
	_, err := x.Exec(`ALTER TABLE "review" ALTER COLUMN "type" TYPE VARCHAR(255)`)
	require.NoError(t, err)

	// Insert rows as they exist in an old database (string type values).
	_, err = x.Exec(`INSERT INTO "review" ("type", "reviewer_id", "issue_id") VALUES ('1', 1, 1), ('2', 2, 1)`)
	require.NoError(t, err)

	sess := x.NewSession()
	require.NoError(t, sess.Begin())
	require.NoError(t, base.RecreateTable(sess, new(issues_model.Review)))
	require.NoError(t, sess.Commit())
	require.NoError(t, sess.Close())

	// The type column is now INTEGER and the rows survived the copy.
	res, err := x.Query(`SELECT data_type FROM information_schema.columns WHERE table_schema = ? AND table_name = 'review' AND column_name = 'type'`, setting.Database.Schema)
	require.NoError(t, err)
	require.Len(t, res, 1)
	require.Equal(t, "integer", string(res[0]["data_type"]))

	var reviews []issues_model.Review
	require.NoError(t, x.Find(&reviews))
	require.Len(t, reviews, 2)
	types := []issues_model.ReviewType{reviews[0].Type, reviews[1].Type}
	require.Contains(t, types, issues_model.ReviewTypeApprove)
	require.Contains(t, types, issues_model.ReviewTypeComment)

	// The id sequence was preserved: the next row gets id 3.
	review := &issues_model.Review{Type: issues_model.ReviewTypeReject, ReviewerID: 3, IssueID: 1}
	_, err = x.Insert(review)
	require.NoError(t, err)
	require.Equal(t, int64(3), review.ID)
}
