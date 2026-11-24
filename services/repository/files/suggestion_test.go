// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package files

import (
	"testing"

	issues_model "gitea.dev/models/issues"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestCodeSuggestionsCommitMessage(t *testing.T) {
	doer := &user_model.User{ID: 1, Name: "author"}
	reviewer := &user_model.User{ID: 2, Name: "reviewer", LowerName: "reviewer", Email: "reviewer@example.com", KeepEmailPrivate: true}
	comment := &issues_model.Comment{Poster: reviewer}
	trailer := "Co-authored-by: reviewer <2+reviewer@" + setting.Service.NoReplyAddress + ">"
	assert.Equal(t, "Apply suggestions from code review\n\n"+trailer,
		codeSuggestionsCommitMessage(doer, "", "", issues_model.CommentList{comment, {Poster: doer}, comment}))
	assert.Equal(t, "Fix: code\n\nDetails\n\n"+trailer, codeSuggestionsCommitMessage(doer, " Fix: code ", "Details\n\n"+trailer, issues_model.CommentList{comment}))
	assert.Equal(t, "Apply suggestion from @"+doer.Name, codeSuggestionsCommitMessage(doer, "", "", issues_model.CommentList{{Poster: doer}}))
}
