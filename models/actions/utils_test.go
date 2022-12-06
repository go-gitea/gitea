// Copyright 2022 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package actions

import (
	"math"
	"testing"

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
