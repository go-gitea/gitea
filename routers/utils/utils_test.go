// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"testing"
)

func TestSanitizeFlashErrorString(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want string
	}{
		{
			name: "no error",
			arg:  "",
			want: "",
		},
		{
			name: "normal error",
			arg:  "can not open file: \"abc.exe\"",
			want: "can not open file: &#34;abc.exe&#34;",
		},
		{
			name: "line break error",
			arg:  "some error:\n\nawesome!",
			want: "some error:<br><br>awesome!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SanitizeFlashErrorString(tt.arg); got != tt.want {
				t.Errorf("SanitizeFlashErrorString() = '%v', want '%v'", got, tt.want)
			}
		})
	}
}
