// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package routing

import (
	"fmt"
	"testing"
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
			if gotShort := shortenFilename(tt.filename, tt.fallback); gotShort != tt.expected {
				t.Errorf("shortenFilename('%s'), expect '%s', but get '%s'", tt.filename, tt.expected, gotShort)
			}
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
			if got := trimAnonymousFunctionSuffix(tt.name); got != tt.want {
				t.Errorf("trimAnonymousFunctionSuffix() = %v, want %v", got, tt.want)
			}
		})
	}
}
