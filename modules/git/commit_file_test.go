// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package git

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseCommitFileStatus(t *testing.T) {
	type testcase struct {
		outputs  []string
		added    []string
		removed  []string
		modified []string
	}

	kases := []testcase{
		{
			// Merge commit
			outputs: []string{
				"MM\x00options/locale/locale_en-US.ini\x00",
				// git 1.7.2 adds an unnecessary \x00 on merge commit
				"\x00MM\x00options/locale/locale_en-US.ini\x00",
			},
			modified: []string{
				"options/locale/locale_en-US.ini",
			},
			added:   []string{},
			removed: []string{},
		},
		{
			// Spaces commit
			outputs: []string{
				"D\x00b\x00D\x00b b/b\x00A\x00b b/b b/b b/b\x00A\x00b b/b b/b b/b b/b\x00",
				// git 1.7.2 adds an unnecessary \n on normal commit
				"\nD\x00b\x00D\x00b b/b\x00A\x00b b/b b/b b/b\x00A\x00b b/b b/b b/b b/b\x00",
			},
			removed: []string{
				"b",
				"b b/b",
			},
			modified: []string{},
			added: []string{
				"b b/b b/b b/b",
				"b b/b b/b b/b b/b",
			},
		},
		{
			// larger commit
			outputs: []string{"M\x00go.mod\x00M\x00go.sum\x00M\x00modules/ssh/ssh.go\x00M\x00vendor/github.com/gliderlabs/ssh/circle.yml\x00M\x00vendor/github.com/gliderlabs/ssh/context.go\x00A\x00vendor/github.com/gliderlabs/ssh/go.mod\x00A\x00vendor/github.com/gliderlabs/ssh/go.sum\x00M\x00vendor/github.com/gliderlabs/ssh/server.go\x00M\x00vendor/github.com/gliderlabs/ssh/session.go\x00M\x00vendor/github.com/gliderlabs/ssh/ssh.go\x00M\x00vendor/golang.org/x/sys/unix/mkerrors.sh\x00M\x00vendor/golang.org/x/sys/unix/syscall_darwin.go\x00M\x00vendor/golang.org/x/sys/unix/zerrors_darwin_amd64.go\x00M\x00vendor/golang.org/x/sys/unix/zerrors_darwin_arm64.go\x00M\x00vendor/golang.org/x/sys/unix/zerrors_freebsd_386.go\x00M\x00vendor/golang.org/x/sys/unix/zerrors_freebsd_amd64.go\x00M\x00vendor/golang.org/x/sys/unix/zerrors_freebsd_arm.go\x00M\x00vendor/golang.org/x/sys/unix/zerrors_freebsd_arm64.go\x00M\x00vendor/golang.org/x/sys/unix/zerrors_linux.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_darwin_amd64.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_darwin_arm64.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_dragonfly_amd64.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_freebsd_386.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_freebsd_amd64.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_freebsd_arm.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_freebsd_arm64.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_netbsd_386.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_netbsd_amd64.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_netbsd_arm.go\x00M\x00vendor/golang.org/x/sys/unix/ztypes_netbsd_arm64.go\x00M\x00vendor/modules.txt\x00"},
			modified: []string{
				"go.mod",
				"go.sum",
				"modules/ssh/ssh.go",
				"vendor/github.com/gliderlabs/ssh/circle.yml",
				"vendor/github.com/gliderlabs/ssh/context.go",
				"vendor/github.com/gliderlabs/ssh/server.go",
				"vendor/github.com/gliderlabs/ssh/session.go",
				"vendor/github.com/gliderlabs/ssh/ssh.go",
				"vendor/golang.org/x/sys/unix/mkerrors.sh",
				"vendor/golang.org/x/sys/unix/syscall_darwin.go",
				"vendor/golang.org/x/sys/unix/zerrors_darwin_amd64.go",
				"vendor/golang.org/x/sys/unix/zerrors_darwin_arm64.go",
				"vendor/golang.org/x/sys/unix/zerrors_freebsd_386.go",
				"vendor/golang.org/x/sys/unix/zerrors_freebsd_amd64.go",
				"vendor/golang.org/x/sys/unix/zerrors_freebsd_arm.go",
				"vendor/golang.org/x/sys/unix/zerrors_freebsd_arm64.go",
				"vendor/golang.org/x/sys/unix/zerrors_linux.go",
				"vendor/golang.org/x/sys/unix/ztypes_darwin_amd64.go",
				"vendor/golang.org/x/sys/unix/ztypes_darwin_arm64.go",
				"vendor/golang.org/x/sys/unix/ztypes_dragonfly_amd64.go",
				"vendor/golang.org/x/sys/unix/ztypes_freebsd_386.go",
				"vendor/golang.org/x/sys/unix/ztypes_freebsd_amd64.go",
				"vendor/golang.org/x/sys/unix/ztypes_freebsd_arm.go",
				"vendor/golang.org/x/sys/unix/ztypes_freebsd_arm64.go",
				"vendor/golang.org/x/sys/unix/ztypes_netbsd_386.go",
				"vendor/golang.org/x/sys/unix/ztypes_netbsd_amd64.go",
				"vendor/golang.org/x/sys/unix/ztypes_netbsd_arm.go",
				"vendor/golang.org/x/sys/unix/ztypes_netbsd_arm64.go",
				"vendor/modules.txt",
			},
			added: []string{
				"vendor/github.com/gliderlabs/ssh/go.mod",
				"vendor/github.com/gliderlabs/ssh/go.sum",
			},
			removed: []string{},
		},
	}

	for _, kase := range kases {
		for _, output := range kase.outputs {
			fileStatus := NewCommitFileStatus()
			parseCommitFileStatus(fileStatus, strings.NewReader(output))

			assert.Equal(t, kase.added, fileStatus.Added)
			assert.Equal(t, kase.removed, fileStatus.Removed)
			assert.Equal(t, kase.modified, fileStatus.Modified)
		}
	}
}

func TestGetCommitFileStatusMerges(t *testing.T) {
	bareRepo6 := mockRepository("repo6_merge")

	commitFileStatus, err := GetCommitFileStatus(t.Context(), bareRepo6, "022f4ce6214973e018f02bf363bf8a2e3691f699")
	assert.NoError(t, err)

	expected := CommitFileStatus{
		[]string{
			"add_file.txt",
		},
		[]string{
			"to_remove.txt",
		},
		[]string{
			"to_modify.txt",
		},
	}

	assert.Equal(t, expected.Added, commitFileStatus.Added)
	assert.Equal(t, expected.Removed, commitFileStatus.Removed)
	assert.Equal(t, expected.Modified, commitFileStatus.Modified)
}

func TestGetCommitFileStatusMergesSha256(t *testing.T) {
	bareRepo6Sha256 := mockRepository("repo6_merge_sha256")

	for _, tc := range []struct {
		commitID string
		expected CommitFileStatus
	}{
		{
			"d2e5609f630dd8db500f5298d05d16def282412e3e66ed68cc7d0833b29129a1",
			CommitFileStatus{[]string{"add_file.txt"}, []string{}, []string{"to_modify.txt"}},
		},
		{
			"da1ded40dc8e5b7c564171f4bf2fc8370487decfb1cb6a99ef28f3ed73d09172",
			CommitFileStatus{[]string{}, []string{"to_remove.txt"}, []string{}},
		},
	} {
		commitFileStatus, err := GetCommitFileStatus(t.Context(), bareRepo6Sha256, tc.commitID)
		assert.NoError(t, err)
		assert.Equal(t, tc.expected.Added, commitFileStatus.Added)
		assert.Equal(t, tc.expected.Removed, commitFileStatus.Removed)
		assert.Equal(t, tc.expected.Modified, commitFileStatus.Modified)
	}
}
