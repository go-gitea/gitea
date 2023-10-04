// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package issues_test

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

func addReaction(t *testing.T, doerID, issueID, commentID int64, content string) {
	var reaction *issues_model.Reaction
	var err error
	if commentID == 0 {
		reaction, err = issues_model.CreateIssueReaction(db.DefaultContext, doerID, issueID, content)
	} else {
		reaction, err = issues_model.CreateCommentReaction(db.DefaultContext, doerID, issueID, commentID, content)
	}
	assert.NoError(t, err)
	assert.NotNil(t, reaction)
}

func TestIssueAddReaction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	var issue1ID int64 = 1

	addReaction(t, user1.ID, issue1ID, 0, "heart")

	unittest.AssertExistsAndLoadBean(t, &issues_model.Reaction{Type: "heart", UserID: user1.ID, IssueID: issue1ID})
}

func TestIssueAddDuplicateReaction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	var issue1ID int64 = 1

	addReaction(t, user1.ID, issue1ID, 0, "heart")

	reaction, err := issues_model.CreateReaction(db.DefaultContext, &issues_model.ReactionOptions{
		DoerID:  user1.ID,
		IssueID: issue1ID,
		Type:    "heart",
	})
	assert.Error(t, err)
	assert.Equal(t, issues_model.ErrReactionAlreadyExist{Reaction: "heart"}, err)

	existingR := unittest.AssertExistsAndLoadBean(t, &issues_model.Reaction{Type: "heart", UserID: user1.ID, IssueID: issue1ID})
	assert.Equal(t, existingR.ID, reaction.ID)
}

func TestIssueDeleteReaction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})

	var issue1ID int64 = 1

	addReaction(t, user1.ID, issue1ID, 0, "heart")

	err := issues_model.DeleteIssueReaction(db.DefaultContext, user1.ID, issue1ID, "heart")
	assert.NoError(t, err)

	unittest.AssertNotExistsBean(t, &issues_model.Reaction{Type: "heart", UserID: user1.ID, IssueID: issue1ID})
}

func TestIssueReactionCount(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	setting.UI.ReactionMaxUserNum = 2

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})
	ghost := user_model.NewGhostUser()

	var issueID int64 = 2
	repo := unittest.AssertExistsAndLoadBean(t, &repo_model.Repository{ID: 1})

	addReaction(t, user1.ID, issueID, 0, "heart")
	addReaction(t, user2.ID, issueID, 0, "heart")
	addReaction(t, org3.ID, issueID, 0, "heart")
	addReaction(t, org3.ID, issueID, 0, "+1")
	addReaction(t, user4.ID, issueID, 0, "+1")
	addReaction(t, user4.ID, issueID, 0, "heart")
	addReaction(t, ghost.ID, issueID, 0, "-1")

	reactionsList, _, err := issues_model.FindReactions(db.DefaultContext, issues_model.FindReactionsOptions{
		IssueID: issueID,
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

	var issue1ID int64 = 1
	var comment1ID int64 = 1

	addReaction(t, user1.ID, issue1ID, comment1ID, "heart")

	unittest.AssertExistsAndLoadBean(t, &issues_model.Reaction{Type: "heart", UserID: user1.ID, IssueID: issue1ID, CommentID: comment1ID})
}

func TestIssueCommentDeleteReaction(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	user1 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 1})
	user2 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 2})
	org3 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 3})
	user4 := unittest.AssertExistsAndLoadBean(t, &user_model.User{ID: 4})

	var issue1ID int64 = 1
	var comment1ID int64 = 1

	addReaction(t, user1.ID, issue1ID, comment1ID, "heart")
	addReaction(t, user2.ID, issue1ID, comment1ID, "heart")
	addReaction(t, org3.ID, issue1ID, comment1ID, "heart")
	addReaction(t, user4.ID, issue1ID, comment1ID, "+1")

	reactionsList, _, err := issues_model.FindReactions(db.DefaultContext, issues_model.FindReactionsOptions{
		IssueID:   issue1ID,
		CommentID: comment1ID,
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

	var issue1ID int64 = 1
	var comment1ID int64 = 1

	addReaction(t, user1.ID, issue1ID, comment1ID, "heart")
	assert.NoError(t, issues_model.DeleteCommentReaction(db.DefaultContext, user1.ID, issue1ID, comment1ID, "heart"))

	unittest.AssertNotExistsBean(t, &issues_model.Reaction{Type: "heart", UserID: user1.ID, IssueID: issue1ID, CommentID: comment1ID})
}
