// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package httplib

import (
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestIsRiskyRedirectURL(t *testing.T) {
	setting.AppURL = "http://localhost:3000/"
	tests := []struct {
		input string
		want  bool
	}{
		{"", false},
		{"foo", false},
		{"/", false},
		{"/foo?k=%20#abc", false},

		{"//", true},
		{"\\\\", true},
		{"/\\", true},
		{"\\/", true},
		{"mail:a@b.com", true},
		{"https://test.com", true},
		{setting.AppURL + "/foo", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, IsRiskyRedirectURL(tt.input))
		})
	}
}
