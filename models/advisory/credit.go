// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
)

// CreditTypes are the same as GitHub's
var CreditTypes = []string{"analyst", "finder", "reporter", "coordinator", "remediation_developer", "remediation_reviewer", "remediation_verifier", "tool", "sponsor", "other"}

type Credit struct {
	ID         int64            `xorm:"pk autoincr"`
	AdvisoryID int64            `xorm:"UNIQUE(s) NOT NULL"`
	UserID     int64            `xorm:"UNIQUE(s) INDEX NOT NULL"`
	User       *user_model.User `xorm:"-"`
	Type       string           `xorm:"VARCHAR(32) NOT NULL"`
}

func init() {
	db.RegisterModel(new(Credit))
}

func (Credit) TableName() string {
	return "security_advisory_credit"
}
