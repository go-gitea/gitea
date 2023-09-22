// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"math"
	"testing"
	"time"

	"code.gitea.io/gitea/internal/modules/timeutil"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogIndexes_ToDB(t *testing.T) {
	tests := []struct {
		indexes LogIndexes
	}{
		{
			indexes: []int64{1, 2, 0, -1, -2, math.MaxInt64, math.MinInt64},
		},
	}
	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got, err := tt.indexes.ToDB()
			require.NoError(t, err)

			indexes := LogIndexes{}
			require.NoError(t, indexes.FromDB(got))

			assert.Equal(t, tt.indexes, indexes)
		})
	}
}

func Test_calculateDuration(t *testing.T) {
	oldTimeSince := timeSince
	defer func() {
		timeSince = oldTimeSince
	}()

	timeSince = func(t time.Time) time.Duration {
		return timeutil.TimeStamp(1000).AsTime().Sub(t)
	}
	type args struct {
		started timeutil.TimeStamp
		stopped timeutil.TimeStamp
		status  Status
	}
	tests := []struct {
		name string
		args args
		want time.Duration
	}{
		{
			name: "unknown",
			args: args{
				started: 0,
				stopped: 0,
				status:  StatusUnknown,
			},
			want: 0,
		},
		{
			name: "running",
			args: args{
				started: 500,
				stopped: 0,
				status:  StatusRunning,
			},
			want: 500 * time.Second,
		},
		{
			name: "done",
			args: args{
				started: 500,
				stopped: 600,
				status:  StatusSuccess,
			},
			want: 100 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, calculateDuration(tt.args.started, tt.args.stopped, tt.args.status), "calculateDuration(%v, %v, %v)", tt.args.started, tt.args.stopped, tt.args.status)
		})
	}
}
