// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.
// SPDX-License-Identifier: MIT

package organization_test

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/organization"
	"code.gitea.io/gitea/models/unittest"

	_ "code.gitea.io/gitea/models/issues"

	"github.com/stretchr/testify/assert"
)

// TestTimesPrepareDB prepares the database for the following tests.
func TestTimesPrepareDB(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())
}

// TestTimesByRepos tests TimesByRepos functionality
func TestTimesByRepos(t *testing.T) {
	kases := []struct {
		name     string
		unixfrom int64
		unixto   int64
		orgname  int64
		expected []organization.ResultTimesByRepos
	}{
		{
			name:     "Full sum for org 1",
			unixfrom: 0,
			unixto:   9223372036854775807,
			orgname:  1,
			expected: []organization.ResultTimesByRepos(nil),
		},
		{
			name:     "Full sum for org 2",
			unixfrom: 0,
			unixto:   9223372036854775807,
			orgname:  2,
			expected: []organization.ResultTimesByRepos{
				{
					Name:    "repo1",
					SumTime: 4083,
				},
				{
					Name:    "repo2",
					SumTime: 75,
				},
			},
		},
		{
			name:     "Simple time bound",
			unixfrom: 946684801,
			unixto:   946684802,
			orgname:  2,
			expected: []organization.ResultTimesByRepos{
				{
					Name:    "repo1",
					SumTime: 3662,
				},
			},
		},
		{
			name:     "Both times inclusive",
			unixfrom: 946684801,
			unixto:   946684801,
			orgname:  2,
			expected: []organization.ResultTimesByRepos{
				{
					Name:    "repo1",
					SumTime: 3661,
				},
			},
		},
		{
			name:     "Should ignore deleted",
			unixfrom: 947688814,
			unixto:   947688815,
			orgname:  2,
			expected: []organization.ResultTimesByRepos{
				{
					Name:    "repo2",
					SumTime: 71,
				},
			},
		},
	}

	// Run test kases
	for _, kase := range kases {
		t.Run(kase.name, func(t *testing.T) {
			org, err := organization.GetOrgByID(db.DefaultContext, kase.orgname)
			assert.NoError(t, err)
			results, err := organization.GetTimesByRepos(org, kase.unixfrom, kase.unixto)
			assert.NoError(t, err)
			assert.Equal(t, kase.expected, results)
		})
	}
}

// TestTimesByMilestones tests TimesByMilestones functionality
func TestTimesByMilestones(t *testing.T) {
	kases := []struct {
		name     string
		unixfrom int64
		unixto   int64
		orgname  int64
		expected []organization.ResultTimesByMilestones
	}{
		{
			name:     "Full sum for org 1",
			unixfrom: 0,
			unixto:   9223372036854775807,
			orgname:  1,
			expected: []organization.ResultTimesByMilestones(nil),
		},
		{
			name:     "Full sum for org 2",
			unixfrom: 0,
			unixto:   9223372036854775807,
			orgname:  2,
			expected: []organization.ResultTimesByMilestones{
				{
					RepoName:     "repo1",
					Name:         "",
					ID:           "",
					SumTime:      401,
					HideRepoName: false,
				},
				{
					RepoName:     "repo1",
					Name:         "milestone1",
					ID:           "1",
					SumTime:      3682,
					HideRepoName: false,
				},
				{
					RepoName:     "repo2",
					Name:         "",
					ID:           "",
					SumTime:      75,
					HideRepoName: false,
				},
			},
		},
		{
			name:     "Simple time bound",
			unixfrom: 946684801,
			unixto:   946684802,
			orgname:  2,
			expected: []organization.ResultTimesByMilestones{
				{
					RepoName:     "repo1",
					Name:         "milestone1",
					ID:           "1",
					SumTime:      3662,
					HideRepoName: false,
				},
			},
		},
		{
			name:     "Both times inclusive",
			unixfrom: 946684801,
			unixto:   946684801,
			orgname:  2,
			expected: []organization.ResultTimesByMilestones{
				{
					RepoName:     "repo1",
					Name:         "milestone1",
					ID:           "1",
					SumTime:      3661,
					HideRepoName: false,
				},
			},
		},
		{
			name:     "Should ignore deleted",
			unixfrom: 947688814,
			unixto:   947688815,
			orgname:  2,
			expected: []organization.ResultTimesByMilestones{
				{
					RepoName:     "repo2",
					Name:         "",
					ID:           "",
					SumTime:      71,
					HideRepoName: false,
				},
			},
		},
	}

	// Run test kases
	for _, kase := range kases {
		t.Run(kase.name, func(t *testing.T) {
			org, err := organization.GetOrgByID(db.DefaultContext, kase.orgname)
			assert.NoError(t, err)
			results, err := organization.GetTimesByMilestones(org, kase.unixfrom, kase.unixto)
			assert.NoError(t, err)
			assert.Equal(t, kase.expected, results)
		})
	}
}

// TestTimesByMembers tests TimesByMembers functionality
func TestTimesByMembers(t *testing.T) {
	kases := []struct {
		name     string
		unixfrom int64
		unixto   int64
		orgname  int64
		expected []organization.ResultTimesByMembers
	}{
		{
			name:     "Full sum for org 1",
			unixfrom: 0,
			unixto:   9223372036854775807,
			orgname:  1,
			expected: []organization.ResultTimesByMembers(nil),
		},
		{
			// Test case: Sum of times forever in org no. 2
			name:     "Full sum for org 2",
			unixfrom: 0,
			unixto:   9223372036854775807,
			orgname:  2,
			expected: []organization.ResultTimesByMembers{
				{
					Name:    "user2",
					SumTime: 3666,
				},
				{
					Name:    "user1",
					SumTime: 491,
				},
			},
		},
		{
			name:     "Simple time bound",
			unixfrom: 946684801,
			unixto:   946684802,
			orgname:  2,
			expected: []organization.ResultTimesByMembers{
				{
					Name:    "user2",
					SumTime: 3662,
				},
			},
		},
		{
			name:     "Both times inclusive",
			unixfrom: 946684801,
			unixto:   946684801,
			orgname:  2,
			expected: []organization.ResultTimesByMembers{
				{
					Name:    "user2",
					SumTime: 3661,
				},
			},
		},
		{
			name:     "Should ignore deleted",
			unixfrom: 947688814,
			unixto:   947688815,
			orgname:  2,
			expected: []organization.ResultTimesByMembers{
				{
					Name:    "user1",
					SumTime: 71,
				},
			},
		},
	}

	// Run test kases
	for _, kase := range kases {
		t.Run(kase.name, func(t *testing.T) {
			org, err := organization.GetOrgByID(db.DefaultContext, kase.orgname)
			assert.NoError(t, err)
			results, err := organization.GetTimesByMembers(org, kase.unixfrom, kase.unixto)
			assert.NoError(t, err)
			assert.Equal(t, kase.expected, results)
		})
	}
}
