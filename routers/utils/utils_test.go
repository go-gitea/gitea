// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package utils

import (
	"html/template"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEscapeFlashErrorString(t *testing.T) {
	tests := []struct {
		name string
		arg  string
		want template.HTML
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
			want: "some error:\n\nawesome!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EscapeFlashErrorString(tt.arg)
			assert.Equal(t, tt.want, got)
		})
	}
}
