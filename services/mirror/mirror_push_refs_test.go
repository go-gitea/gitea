// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package mirror

import (
	"errors"
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/modules/git"

	"github.com/stretchr/testify/assert"
)

func TestBuildPushRefspecs(t *testing.T) {
	local := []string{"refs/heads/main", "refs/heads/dev", "refs/tags/v1"}
	remote := []string{"refs/heads/main", "refs/heads/old", "refs/heads/release/1", "refs/tags/v1", "refs/tags/old"}

	assert.Equal(t,
		[]string{"+refs/heads/main:refs/heads/main", "+refs/heads/dev:refs/heads/dev", "+refs/tags/v1:refs/tags/v1", ":refs/heads/old", ":refs/heads/release/1", ":refs/tags/old"},
		buildPushRefspecs(&repo_model.PushMirrorConfig{}, local, remote))

	assert.Equal(t,
		[]string{"+refs/heads/main:refs/heads/main", "+refs/heads/dev:refs/heads/dev", "+refs/tags/v1:refs/tags/v1"},
		buildPushRefspecs(&repo_model.PushMirrorConfig{KeepRemoteBranches: true, KeepRemoteTags: true}, local, remote))

	// tags are not touched at all, and remote branches outside of the filter are kept
	assert.Equal(t,
		[]string{"+refs/heads/main:refs/heads/main", ":refs/heads/old"},
		buildPushRefspecs(&repo_model.PushMirrorConfig{NoPushTags: true, BranchFilters: []string{"main", "old"}}, local, remote))
	assert.False(t, needRemoteRefs(&repo_model.PushMirrorConfig{KeepRemoteBranches: true, NoPushTags: true}))
}

func TestPushResultStatus(t *testing.T) {
	res := &repo_model.PushMirrorResult{}
	addPushResults(res, git.ParsePushPorcelain("To x\n*\trefs/heads/a:refs/heads/a\t[new branch]\n!\trefs/heads/b:refs/heads/b\t[remote rejected] (hook declined)\nDone\n"), "")
	err := failedRefsError(res)
	assert.ErrorContains(t, err, "refs/heads/b: [remote rejected] (hook declined)")
	assert.Equal(t, repo_model.PushMirrorStatusPartial, pushResultStatus(res, err))
	assert.Equal(t, repo_model.PushMirrorStatusSuccess, pushResultStatus(&repo_model.PushMirrorResult{}, nil))
	assert.Equal(t, repo_model.PushMirrorStatusFailed, pushResultStatus(&repo_model.PushMirrorResult{Error: "x"}, errors.New("x")))
}
