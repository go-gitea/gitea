// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gitrepo

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommitsCount(t *testing.T) {
	bareRepo1 := &mockRepository{path: "repo1_bare"}

	commitsCount, err := CommitsCount(t.Context(), bareRepo1,
		CommitsCountOptions{
			Revision: []string{"8006ff9adbf0cb94da7dad9e537e53817f9fa5c0"},
		})

	assert.NoError(t, err)
	assert.Equal(t, int64(3), commitsCount)
}

func TestCommitsCountWithSinceUntil(t *testing.T) {
	bareRepo1 := &mockRepository{path: "repo1_bare"}
	revision := []string{"8006ff9adbf0cb94da7dad9e537e53817f9fa5c0"}

	// The three commits on this revision are dated 2018-04-18, 2017-12-19 and 2017-12-19.
	cases := []struct {
		name     string
		since    string
		until    string
		expected int64
	}{
		{name: "no filter", expected: 3},
		{name: "since keeps newer commits", since: "2018-01-01", expected: 1},
		{name: "until keeps older commits", until: "2018-01-01", expected: 2},
		{name: "since and until bound the range", since: "2017-12-19T22:16:00-08:00", until: "2018-01-01", expected: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			commitsCount, err := CommitsCount(t.Context(), bareRepo1,
				CommitsCountOptions{
					Revision: revision,
					Since:    tc.since,
					Until:    tc.until,
				})

			assert.NoError(t, err)
			assert.Equal(t, tc.expected, commitsCount)
		})
	}
}

func TestCommitsCountWithoutBase(t *testing.T) {
	bareRepo1 := &mockRepository{path: "repo1_bare"}

	commitsCount, err := CommitsCount(t.Context(), bareRepo1,
		CommitsCountOptions{
			Not:      "master",
			Revision: []string{"branch1"},
		})

	assert.NoError(t, err)
	assert.Equal(t, int64(2), commitsCount)
}

func TestCommitsCountWithSinceUntil(t *testing.T) {
	bareRepo1 := &mockRepository{path: "repo1_bare"}
	revision := []string{"8006ff9adbf0cb94da7dad9e537e53817f9fa5c0"}

	// The three commits on this revision are dated 2018-04-18, 2017-12-19 and 2017-12-19.
	cases := []struct {
		name     string
		since    string
		until    string
		expected int64
	}{
		{name: "no filter", expected: 3},
		{name: "since keeps newer commits", since: "2018-01-01", expected: 1},
		{name: "until keeps older commits", until: "2018-01-01", expected: 2},
		{name: "since and until bound the range", since: "2017-12-19T22:16:00-08:00", until: "2018-01-01", expected: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			commitsCount, err := CommitsCount(t.Context(), bareRepo1,
				CommitsCountOptions{
					Revision: revision,
					Since:    tc.since,
					Until:    tc.until,
				})

			assert.NoError(t, err)
			assert.Equal(t, tc.expected, commitsCount)
		})
	}
}

func TestGetLatestCommitTime(t *testing.T) {
	bareRepo1 := &mockRepository{path: "repo1_bare"}
	lct, err := GetLatestCommitTime(t.Context(), bareRepo1)
	assert.NoError(t, err)
	// Time is Sun Nov 13 16:40:14 2022 +0100
	// which is the time of commit
	// ce064814f4a0d337b333e646ece456cd39fab612 (refs/heads/master)
	assert.EqualValues(t, 1668354014, lct.Unix())
}
