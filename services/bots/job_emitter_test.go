// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package bots

import (
	"testing"

	bots_model "code.gitea.io/gitea/models/bots"

	"github.com/stretchr/testify/assert"
)

func Test_jobStatusResolver_Resolve(t *testing.T) {
	tests := []struct {
		name string
		jobs bots_model.RunJobList
		want map[int64]bots_model.Status
	}{
		{
			name: "no blocked",
			jobs: bots_model.RunJobList{
				{ID: 1, JobID: "1", Status: bots_model.StatusWaiting, Needs: []string{}},
				{ID: 2, JobID: "2", Status: bots_model.StatusWaiting, Needs: []string{}},
				{ID: 3, JobID: "3", Status: bots_model.StatusWaiting, Needs: []string{}},
			},
			want: map[int64]bots_model.Status{},
		},
		{
			name: "single blocked",
			jobs: bots_model.RunJobList{
				{ID: 1, JobID: "1", Status: bots_model.StatusSuccess, Needs: []string{}},
				{ID: 2, JobID: "2", Status: bots_model.StatusBlocked, Needs: []string{"1"}},
				{ID: 3, JobID: "3", Status: bots_model.StatusWaiting, Needs: []string{}},
			},
			want: map[int64]bots_model.Status{
				2: bots_model.StatusWaiting,
			},
		},
		{
			name: "multiple blocked",
			jobs: bots_model.RunJobList{
				{ID: 1, JobID: "1", Status: bots_model.StatusSuccess, Needs: []string{}},
				{ID: 2, JobID: "2", Status: bots_model.StatusBlocked, Needs: []string{"1"}},
				{ID: 3, JobID: "3", Status: bots_model.StatusBlocked, Needs: []string{"1"}},
			},
			want: map[int64]bots_model.Status{
				2: bots_model.StatusWaiting,
				3: bots_model.StatusWaiting,
			},
		},
		{
			name: "chain blocked",
			jobs: bots_model.RunJobList{
				{ID: 1, JobID: "1", Status: bots_model.StatusFailure, Needs: []string{}},
				{ID: 2, JobID: "2", Status: bots_model.StatusBlocked, Needs: []string{"1"}},
				{ID: 3, JobID: "3", Status: bots_model.StatusBlocked, Needs: []string{"2"}},
			},
			want: map[int64]bots_model.Status{
				2: bots_model.StatusSkipped,
				3: bots_model.StatusSkipped,
			},
		},
		{
			name: "loop need",
			jobs: bots_model.RunJobList{
				{ID: 1, JobID: "1", Status: bots_model.StatusBlocked, Needs: []string{"3"}},
				{ID: 2, JobID: "2", Status: bots_model.StatusBlocked, Needs: []string{"1"}},
				{ID: 3, JobID: "3", Status: bots_model.StatusBlocked, Needs: []string{"2"}},
			},
			want: map[int64]bots_model.Status{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := newJobStatusResolver(tt.jobs)
			assert.Equal(t, tt.want, r.Resolve())
		})
	}
}
