// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build !windows

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSafeJoinFilepath(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "empty elems",
			args: []string{},
			want: "",
		},
		{
			name: "empty string",
			args: []string{"", ""},
			want: "",
		},
		{
			name: "escape root",
			args: []string{"/tmp", "../etc/passwd", "../../../../etc/passwd"},
			want: "/tmp/etc/passwd/etc/passwd",
		},
		{
			name: "normal upward",
			args: []string{"/tmp", "/test1/../b", "test2/./test3/../../c"},
			want: "/tmp/b/c",
		},
		{
			name: "relative path",
			args: []string{"./tmp", "/test1/../b", "test2/./test3/../../c"},
			want: "tmp/b/c",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, SafeJoinFilepath(tt.args...), "SafeJoinPath(%v)", tt.args)
		})
	}
}
