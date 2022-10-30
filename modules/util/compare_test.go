// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package util

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindStringInSlice(t *testing.T) {
	tests := []struct {
		target          string
		slice           []string
		caseInsensitive bool
		want            int
	}{
		{target: "a", slice: []string{"a", "b", "c"}, want: 0},
		{target: "c", slice: []string{"a", "b", "c"}, want: 2},
		{target: "d", slice: []string{"a", "b", "c"}, want: -1},
		{target: "C", slice: []string{"a", "b", "c"}, caseInsensitive: true, want: 2},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			got := FindStringInSlice(test.target, test.slice, test.caseInsensitive)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestIsStringInSlice(t *testing.T) {
	tests := []struct {
		target          string
		slice           []string
		caseInsensitive bool
		want            bool
	}{
		{target: "a", slice: []string{"a", "b", "c"}, want: true},
		{target: "c", slice: []string{"a", "b", "c"}, want: true},
		{target: "d", slice: []string{"a", "b", "c"}, want: false},
		{target: "C", slice: []string{"a", "b", "c"}, caseInsensitive: true, want: true},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			got := IsStringInSlice(test.target, test.slice, test.caseInsensitive)
			assert.Equal(t, test.want, got)
		})
	}
}
