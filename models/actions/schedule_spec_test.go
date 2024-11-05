// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"testing"
	"time"

	"code.gitea.io/gitea/modules/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestActionScheduleSpec_Parse(t *testing.T) {
	// Mock the local timezone is not UTC
	tz, err := time.LoadLocation("Asia/Shanghai")
	require.NoError(t, err)
	defer test.MockVariableValue(&time.Local, tz)()

	now, err := time.Parse(time.RFC3339, "2024-07-31T15:47:55+08:00")
	require.NoError(t, err)

	tests := []struct {
		name    string
		spec    string
		want    string
		wantErr assert.ErrorAssertionFunc
	}{
		{
			name:    "regular",
			spec:    "0 10 * * *",
			want:    "2024-07-31T10:00:00Z",
			wantErr: assert.NoError,
		},
		{
			name:    "invalid",
			spec:    "0 10 * *",
			want:    "",
			wantErr: assert.Error,
		},
		{
			name:    "with timezone",
			spec:    "TZ=America/New_York 0 10 * * *",
			want:    "2024-07-31T14:00:00Z",
			wantErr: assert.NoError,
		},
		{
			name:    "timezone irrelevant",
			spec:    "@every 5m",
			want:    "2024-07-31T07:52:55Z",
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &ActionScheduleSpec{
				Spec: tt.spec,
			}
			got, err := s.Parse()
			tt.wantErr(t, err)

			if err == nil {
				assert.Equal(t, tt.want, got.Next(now).UTC().Format(time.RFC3339))
			}
		})
	}
}
