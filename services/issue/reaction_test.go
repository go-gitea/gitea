// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issue

import (
	"testing"

	"code.gitea.io/gitea/models/db"
	issues_model "code.gitea.io/gitea/models/issues"
	repo_model "code.gitea.io/gitea/models/repo"
	"code.gitea.io/gitea/models/unittest"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func addReaction(t *testing.T, doer *user_model.User, issue *issues_model.Issue, comment *issues_model.Comment, content string) {
	var reaction *issues_model.Reaction
	var err error
	if comment == nil {
		reaction, err = CreateIssueReaction(db.DefaultContext, doer, issue, content)
	} else {
		reaction, err = CreateCommentReaction(db.DefaultContext, doer, comment, content)
	}
	assert.NoError(t, err)
	assert.NotNil(t, reaction)
}

func TestIssueAddReaction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})

	addReaction(t, user1, issue, nil, "heart")

	unittest.AssertExistsAndLoadBean(t, &issues_model.Reaction{Type: "heart", UserID: user1.ID, IssueID: issue.ID})
}

func TestIssueAddDuplicateReaction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})

	addReaction(t, user1, issue, nil, "heart")

	reaction, err := CreateIssueReaction(db.DefaultContext, user1, issue, "heart")
	assert.Error(t, err)
	assert.Equal(t, issues_model.ErrReactionAlreadyExist{Reaction: "heart"}, err)

	existingR := unittest.AssertExistsAndLoadBean(t, &issues_model.Reaction{Type: "heart", UserID: user1.ID, IssueID: issue.ID})
	assert.Equal(t, existingR.ID, reaction.ID)
}

func TestIssueDeleteReaction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 1})

	addReaction(t, user1, issue, nil, "heart")

	err := issues_model.DeleteIssueReaction(db.DefaultContext, user1.ID, issue.ID, "heart")
	assert.NoError(t, err)

	unittest.AssertNotExistsBean(t, &issues_model.Reaction{Type: "heart", UserID: user1.ID, IssueID: issue.ID})
}

func TestIssueReactionCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	setting.UI.ReactionMaxUserNum = 2

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	ghost := user_model.NewGhostUser()

	issue := unittest.AssertExistsAndLoadBean(t, &issues_model.Issue{ID: 2})
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	addReaction(t, user1, issue, nil, "heart")
	addReaction(t, user2, issue, nil, "heart")
	addReaction(t, org3, issue, nil, "heart")
	addReaction(t, org3, issue, nil, "+1")
	addReaction(t, user4, issue, nil, "+1")
	addReaction(t, user4, issue, nil, "heart")
	addReaction(t, ghost, issue, nil, "-1")

	reactionsList, _, err := issues_model.FindReactions(db.DefaultContext, issues_model.FindReactionsOptions{
		IssueID: issue.ID,
	})
	assert.NoError(t, err)
	assert.Len(t, reactionsList, 7)
	_, err = reactionsList.LoadUsers(db.DefaultContext, repo)
	assert.NoError(t, err)

	reactions := reactionsList.GroupByType()
	assert.Len(t, reactions["heart"], 4)
	assert.Equal(t, 2, reactions["heart"].GetMoreUserCount())
	assert.Equal(t, user1.Name+", "+user2.Name, reactions["heart"].GetFirstUsers())
	assert.True(t, reactions["heart"].HasUser(1))
	assert.False(t, reactions["heart"].HasUser(5))
	assert.False(t, reactions["heart"].HasUser(0))
	assert.Len(t, reactions["+1"], 2)
	assert.Equal(t, 0, reactions["+1"].GetMoreUserCount())
	assert.Len(t, reactions["-1"], 1)
}

func TestIssueCommentAddReaction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 1})

	addReaction(t, user1, nil, comment, "heart")

	unittest.AssertExistsAndLoadBean(t, &issues_model.Reaction{Type: "heart", UserID: user1.ID, IssueID: comment.IssueID, CommentID: comment.ID})
}

func TestIssueCommentDeleteReaction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 1})

	addReaction(t, user1, nil, comment, "heart")
	addReaction(t, user2, nil, comment, "heart")
	addReaction(t, org3, nil, comment, "heart")
	addReaction(t, user4, nil, comment, "+1")

	reactionsList, _, err := issues_model.FindReactions(db.DefaultContext, issues_model.FindReactionsOptions{
		IssueID:   comment.IssueID,
		CommentID: comment.ID,
	})
	assert.NoError(t, err)
	assert.Len(t, reactionsList, 4)

	reactions := reactionsList.GroupByType()
	assert.Len(t, reactions["heart"], 3)
	assert.Len(t, reactions["+1"], 1)
}

func TestIssueCommentReactionCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	comment := unittest.AssertExistsAndLoadBean(t, &issues_model.Comment{ID: 1})

	addReaction(t, user1, nil, comment, "heart")
	assert.NoError(t, issues_model.DeleteCommentReaction(db.DefaultContext, user1.ID, comment.IssueID, comment.ID, "heart"))

	unittest.AssertNotExistsBean(t, &issues_model.Reaction{Type: "heart", UserID: user1.ID, IssueID: comment.IssueID, CommentID: comment.ID})
}
