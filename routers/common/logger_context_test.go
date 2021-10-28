// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package common

import (
	"testing"
)

func Test_shortenFilename(t *testing.T) {
	tests := []struct {
		name      string
		filename  string
		funcName  string
		wantShort string
	}{
		{
			"code.gitea.io/routers/common/logger_context.go to common/logger_context.go",
			"code.gitea.io/routers/common/logger_context.go",
			"FUNC_NAME",
			"common/logger_context.go",
		},
		{
			"common/logger_context.go to shortenFilename()",
			"common/logger_context.go",
			"shortenFilename()",
			"shortenFilename()",
		},
		{
			"logger_context.go to shortenFilename()",
			"logger_context.go",
			"shortenFilename()",
			"shortenFilename()",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if gotShort := shortenFilename(tt.filename, tt.funcName); gotShort != tt.wantShort {
				t.Errorf("shortenFilename() = %v, want %v", gotShort, tt.wantShort)
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
			"notAnonymous()",
			"notAnonymous()",
		},
		{
			"anonymous.func1()",
			"anonymous",
		},
		{
			"notAnonymous.funca()",
			"notAnonymous.funca()",
		},
		{
			"anonymous.func100()",
			"anonymous",
		},
		{
			"anonymous.func100.func6()",
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
