// Copyright 2025 okTurtles Foundation. All rights reserved.
// SPDX-License-Identifier: MIT

package v1_25_custom

import (
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/xorm"
)

// CreateSubjectsTable creates the subjects table and populates it with existing subject data
func CreateSubjectsTable(x *xorm.Engine) error {
	// Define the new Subject table
	type Subject struct {
		ID          int64              `xorm:"pk autoincr"`
		Name        string             `xorm:"VARCHAR(255) UNIQUE NOT NULL"`
		CreatedUnix timeutil.TimeStamp `xorm:"INDEX created"`
		UpdatedUnix timeutil.TimeStamp `xorm:"INDEX updated"`
	}

	// Create the subjects table
	if err := x.Sync(new(Subject)); err != nil {
		return err
	}

	// Extract all distinct non-empty subject values from repository.subject
	type SubjectRow struct {
		Subject string
	}

	var subjects []SubjectRow
	if err := x.SQL("SELECT DISTINCT subject FROM repository WHERE subject != '' AND subject IS NOT NULL ORDER BY subject").Find(&subjects); err != nil {
		return err
	}

	// Insert each unique subject value as a new row in the subject table
	for _, subjectRow := range subjects {
		subject := Subject{
			Name: subjectRow.Subject,
		}
		if _, err := x.Insert(&subject); err != nil {
			return err
		}
	}

	return nil
}
