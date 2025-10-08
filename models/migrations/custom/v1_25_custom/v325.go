// Copyright 2025 okTurtles Foundation. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25_custom

import (
	"xorm.io/xorm"
)

// AddSubjectForeignKeyToRepository adds the subject_id foreign key column and establishes relationships
// This is Phase 2 of the subject refactoring (Phase 1 in v324 created the subjects table)
//
// NOTE: The legacy `subject` VARCHAR column is kept in this migration for backward compatibility.
// It can be safely dropped in a future migration (v326 or later) after this release has been
// deployed and verified in production. The application code no longer uses this column.
func AddSubjectForeignKeyToRepository(x *xorm.Engine) error {
	// Temporary repository struct for migration
	type Repository struct {
		ID        int64  `xorm:"pk autoincr"`
		Subject   string `xorm:"VARCHAR(255) NOT NULL DEFAULT ''"` // Legacy field - can be dropped in future migration
		SubjectID int64  `xorm:"INDEX"`
	}

	// Step 1: Add SubjectID column to repository table
	if err := x.Sync(new(Repository)); err != nil {
		return err
	}

	// Step 2: Update repositories to reference the subject via foreign key
	// This is done in a single SQL statement for efficiency
	_, err := x.Exec(`
		UPDATE repository 
		SET subject_id = (
			SELECT id FROM subject WHERE subject.name = repository.subject
		)
		WHERE subject != '' AND subject IS NOT NULL
	`)
	if err != nil {
		return err
	}

	// Step 3: For repositories with empty subjects, set subject_id to NULL
	// (they will fall back to using the repository name)
	_, err = x.Exec("UPDATE repository SET subject_id = NULL WHERE subject = '' OR subject IS NULL")
	if err != nil {
		return err
	}

	return nil
}

