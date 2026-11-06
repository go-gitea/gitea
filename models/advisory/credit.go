// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package advisory

import (
	"context"
	"slices"

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

// VisibleCredits hides the users whose profile the viewer can't see once the advisory is public, like a reporter with a private profile
func (a *Advisory) VisibleCredits(ctx context.Context, viewer *user_model.User) []*Credit {
	if !a.IsPublic() {
		return a.Credits
	}
	return slices.DeleteFunc(slices.Clone(a.Credits), func(c *Credit) bool { return !user_model.IsUserVisibleToViewer(ctx, c.User, viewer) })
}
