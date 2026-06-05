// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package cmd

import (
	"testing"

	"gitea.dev/models/perm"
	"gitea.dev/modules/git"

	"github.com/stretchr/testify/assert"
)

func TestGetAccessMode(t *testing.T) {
	cases := []struct {
		verb, lfsVerb string
		expected      perm.AccessMode
	}{
		{git.CmdVerbUploadPack, "", perm.AccessModeRead},
		{git.CmdVerbUploadArchive, "", perm.AccessModeRead},
		{git.CmdVerbReceivePack, "", perm.AccessModeWrite},
		{git.CmdVerbLfsAuthenticate, git.CmdSubVerbLfsUpload, perm.AccessModeWrite},
		{git.CmdVerbLfsAuthenticate, git.CmdSubVerbLfsDownload, perm.AccessModeRead},
		{git.CmdVerbLfsTransfer, git.CmdSubVerbLfsUpload, perm.AccessModeWrite},
		{git.CmdVerbLfsTransfer, git.CmdSubVerbLfsDownload, perm.AccessModeRead},
	}
	for _, tc := range cases {
		t.Run(tc.verb+"/"+tc.lfsVerb, func(t *testing.T) {
			assert.Equal(t, tc.expected, getAccessMode(tc.verb, tc.lfsVerb))
		})
	}
}

// TestGetAccessModeUnknownLFSVerbPanics locks in the invariant that runServ
// must reject unknown LFS sub-verbs before calling getAccessMode. If this
// guard regresses, getAccessMode falls through to AccessModeNone (0), which
// bypasses the `userMode < mode` permission check in routers/private/serv.go
// and hands out valid LFS JWTs for any private repository.
func TestGetAccessModeUnknownLFSVerbPanics(t *testing.T) {
	cases := []struct{ verb, lfsVerb string }{
		{git.CmdVerbLfsAuthenticate, ""},
		{git.CmdVerbLfsAuthenticate, "badverb"},
		{git.CmdVerbLfsTransfer, "badverb"},
		{"git-unknown-verb", ""},
	}
	for _, tc := range cases {
		t.Run(tc.verb+"/"+tc.lfsVerb, func(t *testing.T) {
			assert.Panics(t, func() {
				_ = getAccessMode(tc.verb, tc.lfsVerb)
			})
		})
	}
}
