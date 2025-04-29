// Copyright 2017 The Gitea Authors. All rights reserved.
// Copyright 2014 The Gogs Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"testing"
	"time"

	"code.gitea.io/gitea/models/asymkey"
	git_model "code.gitea.io/gitea/models/git"
	"code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/timeutil"

	"github.com/stretchr/testify/assert"
)

func TestGroupCommitsByDate(t *testing.T) {
	// Create test data
	// These two commits represent the same moment but in different timezones
	// commit1: 2025-04-10T08:00:00+08:00
	// commit2: 2025-04-09T23:00:00-01:00
	// Their UTC time is both 2025-04-10T00:00:00Z

	// Create the first commit (Asia timezone +8)
	asiaTimezone := time.FixedZone("Asia/Shanghai", 8*60*60)
	commit1Time := time.Date(2025, 4, 10, 8, 0, 0, 0, asiaTimezone)
	commit1 := &git_model.SignCommitWithStatuses{
		SignCommit: &asymkey.SignCommit{
			UserCommit: &user.UserCommit{
				Commit: &git.Commit{
					Committer: &git.Signature{
						When: commit1Time,
					},
				},
			},
		},
	}

	// Create the second commit (Western timezone -1)
	westTimezone := time.FixedZone("West", -1*60*60)
	commit2Time := time.Date(2025, 4, 9, 23, 0, 0, 0, westTimezone)
	commit2 := &git_model.SignCommitWithStatuses{
		SignCommit: &asymkey.SignCommit{
			UserCommit: &user.UserCommit{
				Commit: &git.Commit{
					Committer: &git.Signature{
						When: commit2Time,
					},
				},
			},
		},
	}

	// Verify that the two timestamps actually represent the same moment
	assert.Equal(t, commit1Time.Unix(), commit2Time.Unix(), "The two commits should have the same Unix timestamp")

	// Test the modified grouping behavior
	commits := []*git_model.SignCommitWithStatuses{commit1, commit2}
	grouped := GroupCommitsByDate(commits)

	// Output the grouping results for observation
	t.Logf("Number of grouped results: %d", len(grouped))
	for i, group := range grouped {
		t.Logf("Group %d: Date %s, Number of commits %d", i, time.Unix(int64(group.Date), 0).Format("2006-01-02"), len(group.Commits))
		for j, c := range group.Commits {
			t.Logf("  Commit %d: Time %s", j, c.SignCommit.UserCommit.Commit.Committer.When.Format(time.RFC3339))
		}
	}

	// After modification, these two commits should be grouped together as they are on the same day in UTC timezone
	assert.Len(t, grouped, 1, "After modification, the two commits should be grouped together")

	// Verify the group date (should be 2025-04-10, the date in UTC timezone)
	utcDate := time.Date(2025, 4, 10, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, timeutil.TimeStamp(utcDate.Unix()), grouped[0].Date)
	assert.Len(t, grouped[0].Commits, 2)

	// Verify that both commits are in this group
	commitMap := make(map[*git_model.SignCommitWithStatuses]bool)
	for _, c := range grouped[0].Commits {
		commitMap[c] = true
	}
	assert.True(t, commitMap[commit1], "The first commit should be in the group")
	assert.True(t, commitMap[commit2], "The second commit should be in the group")

	// Add a commit with a different date for testing
	nextDayTimezone := time.FixedZone("NextDay", 0)
	commit3Time := time.Date(2025, 4, 11, 0, 0, 0, 0, nextDayTimezone)
	commit3 := &git_model.SignCommitWithStatuses{
		SignCommit: &asymkey.SignCommit{
			UserCommit: &user.UserCommit{
				Commit: &git.Commit{
					Committer: &git.Signature{
						When: commit3Time,
					},
				},
			},
		},
	}

	// Test with commits from different dates
	commits = append(commits, commit3)
	grouped = GroupCommitsByDate(commits)

	// Now there should be two groups
	assert.Len(t, grouped, 2, "There should be two different date groups")

	// Verify date sorting (descending, most recent date first)
	assert.True(t, time.Unix(int64(grouped[0].Date), 0).After(time.Unix(int64(grouped[1].Date), 0)),
		"Dates should be sorted in descending order")
}
