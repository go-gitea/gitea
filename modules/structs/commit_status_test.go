// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNoBetterThan(t *testing.T) {
	type args struct {
		css  CommitStatusState
		css2 CommitStatusState
	}
	var unExpectedState CommitStatusState
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "success is no better than success",
			args: args{
				css:  CommitStatusSuccess,
				css2: CommitStatusSuccess,
			},
			want: true,
		},
		{
			name: "success is no better than pending",
			args: args{
				css:  CommitStatusSuccess,
				css2: CommitStatusPending,
			},
			want: false,
		},
		{
			name: "success is no better than failure",
			args: args{
				css:  CommitStatusSuccess,
				css2: CommitStatusFailure,
			},
			want: false,
		},
		{
			name: "success is no better than error",
			args: args{
				css:  CommitStatusSuccess,
				css2: CommitStatusError,
			},
			want: false,
		},
		{
			name: "pending is no better than success",
			args: args{
				css:  CommitStatusPending,
				css2: CommitStatusSuccess,
			},
			want: true,
		},
		{
			name: "pending is no better than pending",
			args: args{
				css:  CommitStatusPending,
				css2: CommitStatusPending,
			},
			want: true,
		},
		{
			name: "pending is no better than failure",
			args: args{
				css:  CommitStatusPending,
				css2: CommitStatusFailure,
			},
			want: false,
		},
		{
			name: "pending is no better than error",
			args: args{
				css:  CommitStatusPending,
				css2: CommitStatusError,
			},
			want: false,
		},
		{
			name: "failure is no better than success",
			args: args{
				css:  CommitStatusFailure,
				css2: CommitStatusSuccess,
			},
			want: true,
		},
		{
			name: "failure is no better than pending",
			args: args{
				css:  CommitStatusFailure,
				css2: CommitStatusPending,
			},
			want: true,
		},
		{
			name: "failure is no better than failure",
			args: args{
				css:  CommitStatusFailure,
				css2: CommitStatusFailure,
			},
			want: true,
		},
		{
			name: "failure is no better than error",
			args: args{
				css:  CommitStatusFailure,
				css2: CommitStatusError,
			},
			want: false,
		},
		{
			name: "error is no better than success",
			args: args{
				css:  CommitStatusError,
				css2: CommitStatusSuccess,
			},
			want: true,
		},
		{
			name: "error is no better than pending",
			args: args{
				css:  CommitStatusError,
				css2: CommitStatusPending,
			},
			want: true,
		},
		{
			name: "error is no better than failure",
			args: args{
				css:  CommitStatusError,
				css2: CommitStatusFailure,
			},
			want: true,
		},
		{
			name: "error is no better than error",
			args: args{
				css:  CommitStatusError,
				css2: CommitStatusError,
			},
			want: true,
		},
		{
			name: "unExpectedState is no better than success",
			args: args{
				css:  unExpectedState,
				css2: CommitStatusSuccess,
			},
			want: false,
		},
		{
			name: "unExpectedState is no better than unExpectedState",
			args: args{
				css:  unExpectedState,
				css2: unExpectedState,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.args.css.NoBetterThan(tt.args.css2)
			assert.Equal(t, tt.want, result)
		})
	}
}
