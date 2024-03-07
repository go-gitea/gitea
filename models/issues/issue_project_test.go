// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/issues"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/project"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"github.com/stretchr/testify/assert"
)

func Test_LoadIssuesFromBoard(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	user3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	user5 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 5})
	user15 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 15})

	org3 := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 3})
	org17 := unittest.AssertExistsAndLoadBean(t, &organization.Organization{ID: 17})

	projectBoard1 := unittest.AssertExistsAndLoadBean(t, &project.Board{ID: 1})
	projectBoard4 := unittest.AssertExistsAndLoadBean(t, &project.Board{ID: 4})
	projectBoard5 := unittest.AssertExistsAndLoadBean(t, &project.Board{ID: 5})
	projectBoard6 := unittest.AssertExistsAndLoadBean(t, &project.Board{ID: 6})
	projectBoard7 := unittest.AssertExistsAndLoadBean(t, &project.Board{ID: 7})
	projectBoard8 := unittest.AssertExistsAndLoadBean(t, &project.Board{ID: 8})

	tests := []struct {
		name   string
		board  *project.Board
		user   *user_model.User
		org    *organization.Organization
		expect int
	}{
		{
			name:   "individual public repo, repo project, owner",
			board:  projectBoard1,
			user:   user2,
			org:    nil,
			expect: 1,
		},
		{
			name:   "individual public repo, repo project, login user",
			board:  projectBoard1,
			user:   user3,
			org:    nil,
			expect: 1,
		},
		{
			name:   "individual public repo, repo project, non-login",
			board:  projectBoard1,
			user:   nil,
			org:    nil,
			expect: 1,
		},
		{
			name:   "individual public repo, individual project, owner",
			board:  projectBoard4,
			user:   user2,
			org:    nil,
			expect: 1,
		},
		{
			name:   "individual public repo, individual project, login user",
			board:  projectBoard4,
			user:   user3,
			org:    nil,
			expect: 1,
		},
		{
			name:   "individual public repo, individual project, non-login",
			board:  projectBoard4,
			user:   nil,
			org:    nil,
			expect: 1,
		},
		{
			name:   "organization public repo, repo project, org admin",
			board:  projectBoard5,
			user:   user15,
			org:    nil,
			expect: 1,
		},
		{
			name:   "organization public repo, repo project, member",
			board:  projectBoard5,
			user:   user2,
			org:    nil,
			expect: 1,
		},
		{
			name:   "organization public repo, repo project, login user",
			board:  projectBoard5,
			user:   user3,
			org:    nil,
			expect: 1,
		},
		{
			name:   "organization public repo, repo project, non-login",
			board:  projectBoard5,
			user:   nil,
			org:    nil,
			expect: 1,
		},
		{
			name:   "organization public repo, org project, org admin",
			board:  projectBoard6,
			user:   user15,
			org:    org17,
			expect: 1,
		},
		{
			name:   "organization public repo, org project, member",
			board:  projectBoard6,
			user:   user2,
			org:    org17,
			expect: 1,
		},
		{
			name:   "organization public repo, org project, login user",
			board:  projectBoard6,
			user:   user3,
			org:    org17,
			expect: 1,
		},
		{
			name:   "organization public repo, org project, non-login",
			board:  projectBoard6,
			user:   nil,
			org:    org17,
			expect: 1,
		},
		{
			name:   "organization private repo, repo project, org admin",
			board:  projectBoard7,
			user:   user2,
			org:    nil,
			expect: 1,
		},
		{
			name:   "organization private repo, repo project, member with issue access",
			board:  projectBoard7,
			user:   user5,
			org:    nil,
			expect: 1,
		},
		{
			name:   "organization private repo, repo project, member without issue access",
			board:  projectBoard7,
			user:   user4,
			org:    nil,
			expect: 0,
		},
		{
			name:   "organization private repo, repo project, login user",
			board:  projectBoard7,
			user:   user3,
			org:    nil,
			expect: 0,
		},
		{
			name:   "organization private repo, repo project, non-login",
			board:  projectBoard7,
			user:   nil,
			org:    nil,
			expect: 0,
		},
		{
			name:   "organization private repo, org project, org admin",
			board:  projectBoard8,
			user:   user2,
			org:    org3,
			expect: 1,
		},
		{
			name:   "organization private repo, org project, member with issue access",
			board:  projectBoard8,
			user:   user5,
			org:    org3,
			expect: 1,
		},
		{
			name:   "organization private repo, org project, member without issue access",
			board:  projectBoard8,
			user:   user4,
			org:    org3,
			expect: 0,
		},
		{
			name:   "organization private repo, org project, login user",
			board:  projectBoard8,
			user:   user3,
			org:    org3,
			expect: 0,
		},
		{
			name:   "organization private repo, org project, non-login",
			board:  projectBoard8,
			user:   nil,
			org:    org3,
			expect: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := issues.LoadIssuesFromBoard(db.DefaultContext, tt.board, tt.user, tt.org)
			assert.NoError(t, err)
			assert.Equal(t, tt.expect, len(results))
		})

	}
}
