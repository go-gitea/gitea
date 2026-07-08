// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package context

import (
	"testing"

	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestCloneButtonsShow(t *testing.T) {
	defer func(disableHTTPGit, sshDisabled, sshExposeAnonymous bool) {
		setting.Repository.DisableHTTPGit = disableHTTPGit
		setting.SSH.Disabled = sshDisabled
		setting.SSH.ExposeAnonymous = sshExposeAnonymous
	}(setting.Repository.DisableHTTPGit, setting.SSH.Disabled, setting.SSH.ExposeAnonymous)

	cases := []struct {
		name               string
		disableHTTPGit     bool
		sshDisabled        bool
		sshExposeAnonymous bool
		isSigned           bool
		wantHTTPS          bool
		wantSSH            bool
	}{
		{name: "defaults signed", wantHTTPS: true, wantSSH: true, isSigned: true},
		{name: "defaults anonymous", wantHTTPS: true, wantSSH: false},
		{name: "expose anonymous ssh", sshExposeAnonymous: true, wantHTTPS: true, wantSSH: true},
		// issue #38339: DISABLE_HTTP_GIT must never re-enable the HTTPS clone button.
		{name: "http disabled signed shows ssh only", disableHTTPGit: true, isSigned: true, wantHTTPS: false, wantSSH: true},
		{name: "http disabled anonymous shows nothing", disableHTTPGit: true, wantHTTPS: false, wantSSH: false},
		{name: "http disabled anonymous exposed ssh", disableHTTPGit: true, sshExposeAnonymous: true, wantHTTPS: false, wantSSH: true},
		{name: "both disabled shows nothing", disableHTTPGit: true, sshDisabled: true, isSigned: true, wantHTTPS: false, wantSSH: false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setting.Repository.DisableHTTPGit = c.disableHTTPGit
			setting.SSH.Disabled = c.sshDisabled
			setting.SSH.ExposeAnonymous = c.sshExposeAnonymous

			showHTTPS, showSSH := cloneButtonsShow(c.isSigned)
			assert.Equal(t, c.wantHTTPS, showHTTPS, "showHTTPS")
			assert.Equal(t, c.wantSSH, showSSH, "showSSH")
		})
	}
}
