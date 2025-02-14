// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package routing

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_shortenFilename(t *testing.T) {
	tests := []struct {
		filename string
		fallback string
		expected string
	}{
		{
			"code.gitea.io/routers/common/logger_context.go",
			"NO_FALLBACK",
			"common/logger_context.go",
		},
		{
			"common/logger_context.go",
			"NO_FALLBACK",
			"common/logger_context.go",
		},
		{
			"logger_context.go",
			"NO_FALLBACK",
			"logger_context.go",
		},
		{
			"",
			"USE_FALLBACK",
			"USE_FALLBACK",
		},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("shortenFilename('%s')", tt.filename), func(t *testing.T) {
			gotShort := shortenFilename(tt.filename, tt.fallback)
			assert.Equal(t, tt.expected, gotShort)
		})
	}
}

func Test_trimAnonymousFunctionSuffix(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{
			"notAnonymous",
			"notAnonymous",
		},
		{
			"anonymous.func1",
			"anonymous",
		},
		{
			"notAnonymous.funca",
			"notAnonymous.funca",
		},
		{
			"anonymous.func100",
			"anonymous",
		},
		{
			"anonymous.func100.func6",
			"anonymous.func100",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimAnonymousFunctionSuffix(tt.name)
			assert.Equal(t, tt.want, got)
		})
	}
}
