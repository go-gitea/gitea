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

	// Populate the subject table with distinct non-empty subject values from repository.subject
	_, err := x.Exec("INSERT INTO subject (name) SELECT DISTINCT subject FROM repository WHERE subject != '' AND subject IS NOT NULL")
	return err
}
