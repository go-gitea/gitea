// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"

	bots_model "code.gitea.io/gitea/models/actions"

	"github.com/stretchr/testify/assert"
)

func TestFullSteps(t *testing.T) {
	tests := []struct {
		name string
		task *bots_model.BotTask
		want []*bots_model.BotTaskStep
	}{
		{
			name: "regular",
			task: &bots_model.BotTask{
				Steps: []*bots_model.BotTaskStep{
					{Status: bots_model.StatusSuccess, LogIndex: 10, LogLength: 80, Started: 10010, Stopped: 10090},
				},
				Status:    bots_model.StatusSuccess,
				Started:   10000,
				Stopped:   10100,
				LogLength: 100,
			},
			want: []*bots_model.BotTaskStep{
				{Name: preStepName, Status: bots_model.StatusSuccess, LogIndex: 0, LogLength: 10, Started: 10000, Stopped: 10010},
				{Status: bots_model.StatusSuccess, LogIndex: 10, LogLength: 80, Started: 10010, Stopped: 10090},
				{Name: postStepName, Status: bots_model.StatusSuccess, LogIndex: 90, LogLength: 10, Started: 10090, Stopped: 10100},
			},
		},
		{
			name: "failed step",
			task: &bots_model.BotTask{
				Steps: []*bots_model.BotTaskStep{
					{Status: bots_model.StatusSuccess, LogIndex: 10, LogLength: 20, Started: 10010, Stopped: 10020},
					{Status: bots_model.StatusFailure, LogIndex: 30, LogLength: 60, Started: 10020, Stopped: 10090},
					{Status: bots_model.StatusCancelled, LogIndex: 0, LogLength: 0, Started: 0, Stopped: 0},
				},
				Status:    bots_model.StatusFailure,
				Started:   10000,
				Stopped:   10100,
				LogLength: 100,
			},
			want: []*bots_model.BotTaskStep{
				{Name: preStepName, Status: bots_model.StatusSuccess, LogIndex: 0, LogLength: 10, Started: 10000, Stopped: 10010},
				{Status: bots_model.StatusSuccess, LogIndex: 10, LogLength: 20, Started: 10010, Stopped: 10020},
				{Status: bots_model.StatusFailure, LogIndex: 30, LogLength: 60, Started: 10020, Stopped: 10090},
				{Status: bots_model.StatusCancelled, LogIndex: 0, LogLength: 0, Started: 0, Stopped: 0},
				{Name: postStepName, Status: bots_model.StatusFailure, LogIndex: 90, LogLength: 10, Started: 10090, Stopped: 10100},
			},
		},
		{
			name: "first step is running",
			task: &bots_model.BotTask{
				Steps: []*bots_model.BotTaskStep{
					{Status: bots_model.StatusRunning, LogIndex: 10, LogLength: 80, Started: 10010, Stopped: 0},
				},
				Status:    bots_model.StatusRunning,
				Started:   10000,
				Stopped:   10100,
				LogLength: 100,
			},
			want: []*bots_model.BotTaskStep{
				{Name: preStepName, Status: bots_model.StatusSuccess, LogIndex: 0, LogLength: 10, Started: 10000, Stopped: 10010},
				{Status: bots_model.StatusRunning, LogIndex: 10, LogLength: 80, Started: 10010, Stopped: 0},
				{Name: postStepName, Status: bots_model.StatusWaiting, LogIndex: 0, LogLength: 0, Started: 0, Stopped: 0},
			},
		},
		{
			name: "first step has canceled",
			task: &bots_model.BotTask{
				Steps: []*bots_model.BotTaskStep{
					{Status: bots_model.StatusCancelled, LogIndex: 0, LogLength: 0, Started: 0, Stopped: 0},
				},
				Status:    bots_model.StatusFailure,
				Started:   10000,
				Stopped:   10100,
				LogLength: 100,
			},
			want: []*bots_model.BotTaskStep{
				{Name: preStepName, Status: bots_model.StatusFailure, LogIndex: 0, LogLength: 100, Started: 10000, Stopped: 10100},
				{Status: bots_model.StatusCancelled, LogIndex: 0, LogLength: 0, Started: 0, Stopped: 0},
				{Name: postStepName, Status: bots_model.StatusFailure, LogIndex: 100, LogLength: 0, Started: 10100, Stopped: 10100},
			},
		},
		{
			name: "empty steps",
			task: &bots_model.BotTask{
				Steps:     []*bots_model.BotTaskStep{},
				Status:    bots_model.StatusSuccess,
				Started:   10000,
				Stopped:   10100,
				LogLength: 100,
			},
			want: []*bots_model.BotTaskStep{
				{Name: preStepName, Status: bots_model.StatusSuccess, LogIndex: 0, LogLength: 100, Started: 10000, Stopped: 10100},
				{Name: postStepName, Status: bots_model.StatusSuccess, LogIndex: 100, LogLength: 0, Started: 10100, Stopped: 10100},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, FullSteps(tt.task), "FullSteps(%v)", tt.task)
		})
	}
}
