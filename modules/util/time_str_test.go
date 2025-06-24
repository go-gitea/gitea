// Copyright 2024 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTimeStr(t *testing.T) {
	t.Run("Parse", func(t *testing.T) {
		// Test TimeEstimateParse
		tests := []struct {
			input  string
			output int64
			err    bool
		}{
			{"1h", 3600, false},
			{"1m", 60, false},
			{"1s", 1, false},
			{"1h 1m 1s", 3600 + 60 + 1, false},
			{"1d1x", 0, true},
		}
		for _, test := range tests {
			t.Run(test.input, func(t *testing.T) {
				output, err := TimeEstimateParse(test.input)
				if test.err {
					assert.Error(t, err)
				} else {
					assert.NoError(t, err)
				}
				assert.Equal(t, test.output, output)
			})
		}
	})
	t.Run("String", func(t *testing.T) {
		tests := []struct {
			input  int64
			output string
		}{
			{3600, "1h"},
			{60, "1m"},
			{1, "1s"},
			{3600 + 1, "1h 1s"},
		}
		for _, test := range tests {
			t.Run(test.output, func(t *testing.T) {
				output := TimeEstimateString(test.input)
				assert.Equal(t, test.output, output)
			})
		}
	})
}
