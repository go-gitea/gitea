// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	org_model "gitea.dev/models/organization"
	"gitea.dev/models/unittest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTeamMemberHasReviewed(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	team := unittest.AssertExistsAndLoadBean(t, &org_model.Team{ID: 2}) // members: user2, user4

	for _, tc := range []struct {
		name    string
		reviews issues_model.ReviewList
		want    bool
	}{
		{"no reviews", nil, false},
		{"member approved", issues_model.ReviewList{{ReviewerID: 4, Type: issues_model.ReviewTypeApprove}}, true},
		{"member rejected", issues_model.ReviewList{{ReviewerID: 4, Type: issues_model.ReviewTypeReject}}, true},
		{"member only commented", issues_model.ReviewList{{ReviewerID: 4, Type: issues_model.ReviewTypeComment}}, false},
		{"member only requested", issues_model.ReviewList{{ReviewerID: 4, Type: issues_model.ReviewTypeRequest}}, false},
		{"non-member approved", issues_model.ReviewList{{ReviewerID: 5, Type: issues_model.ReviewTypeApprove}}, false},
		{"team review row", issues_model.ReviewList{{ReviewerTeamID: 2, Type: issues_model.ReviewTypeApprove}}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reviewed, err := teamMemberHasReviewed(t.Context(), tc.reviews, team)
			require.NoError(t, err)
			assert.Equal(t, tc.want, reviewed)
		})
	}
}
