package models

import (
	"fmt"
	"path"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func TestAction_GetRepoPath(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	repo := AssertExistsAndLoadBean(t, &Repository{}).(*Repository)
	owner := AssertExistsAndLoadBean(t, &User{ID: repo.OwnerID}).(*User)
	action := &Action{RepoID: repo.ID}
	assert.Equal(t, path.Join(owner.Name, repo.Name), action.GetRepoPath())
}

func TestAction_GetRepoLink(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	repo := AssertExistsAndLoadBean(t, &Repository{}).(*Repository)
	owner := AssertExistsAndLoadBean(t, &User{ID: repo.OwnerID}).(*User)
	action := &Action{RepoID: repo.ID}
	setting.AppSubURL = "/suburl/"
	expected := path.Join(setting.AppSubURL, owner.Name, repo.Name)
	assert.Equal(t, expected, action.GetRepoLink())
}

func TestPushCommits_ToAPIPayloadCommits(t *testing.T) {
	pushCommits := NewPushCommits()
	pushCommits.Commits = []*PushCommit{
		{
			Sha1:           "abcdef1",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user4@example.com",
			AuthorName:     "User Four",
			Message:        "message1",
		},
		{
			Sha1:           "abcdef2",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User Two",
			Message:        "message2",
		},
	}
	pushCommits.Len = len(pushCommits.Commits)

	payloadCommits := pushCommits.ToAPIPayloadCommits("/username/reponame")
	if assert.Len(t, payloadCommits, 2) {
		assert.Equal(t, "abcdef1", payloadCommits[0].ID)
		assert.Equal(t, "message1", payloadCommits[0].Message)
		assert.Equal(t, "/username/reponame/commit/abcdef1", payloadCommits[0].URL)
		assert.Equal(t, "User Two", payloadCommits[0].Committer.Name)
		assert.Equal(t, "user2", payloadCommits[0].Committer.UserName)
		assert.Equal(t, "User Four", payloadCommits[0].Author.Name)
		assert.Equal(t, "user4", payloadCommits[0].Author.UserName)

		assert.Equal(t, "abcdef2", payloadCommits[1].ID)
		assert.Equal(t, "message2", payloadCommits[1].Message)
		assert.Equal(t, "/username/reponame/commit/abcdef2", payloadCommits[1].URL)
		assert.Equal(t, "User Two", payloadCommits[1].Committer.Name)
		assert.Equal(t, "user2", payloadCommits[1].Committer.UserName)
		assert.Equal(t, "User Two", payloadCommits[1].Author.Name)
		assert.Equal(t, "user2", payloadCommits[1].Author.UserName)
	}
}

func TestPushCommits_AvatarLink(t *testing.T) {
	pushCommits := NewPushCommits()
	pushCommits.Commits = []*PushCommit{
		{
			Sha1:           "abcdef1",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user4@example.com",
			AuthorName:     "User Four",
			Message:        "message1",
		},
		{
			Sha1:           "abcdef2",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User Two",
			Message:        "message2",
		},
	}
	pushCommits.Len = len(pushCommits.Commits)

	assert.Equal(t,
		"https://secure.gravatar.com/avatar/ab53a2911ddf9b4817ac01ddcd3d975f?d=identicon",
		pushCommits.AvatarLink("user2@example.com"))

	assert.Equal(t,
		"https://secure.gravatar.com/avatar/19ade630b94e1e0535b3df7387434154?d=identicon",
		pushCommits.AvatarLink("nonexistent@example.com"))
}

func Test_getIssueFromRef(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	for _, test := range []struct {
		Ref             string
		ExpectedIssueID int64
	}{
		{"#2", 2},
		{"reopen #2", 2},
		{"user2/repo2#1", 4},
		{"fixes user2/repo2#1", 4},
	} {
		issue, err := getIssueFromRef(repo, test.Ref)
		assert.NoError(t, err)
		if assert.NotNil(t, issue) {
			assert.EqualValues(t, test.ExpectedIssueID, issue.ID)
		}
	}

	for _, badRef := range []string{
		"doesnotexist/doesnotexist#1",
		fmt.Sprintf("#%d", NonexistentID),
	} {
		issue, err := getIssueFromRef(repo, badRef)
		assert.NoError(t, err)
		assert.Nil(t, issue)
	}
}

func TestUpdateIssuesCommit(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	pushCommits := []*PushCommit{
		{
			Sha1:           "abcdef1",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user4@example.com",
			AuthorName:     "User Four",
			Message:        "start working on #FST-1, #1",
		},
		{
			Sha1:           "abcdef2",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User Two",
			Message:        "a plain message",
		},
		{
			Sha1:           "abcdef2",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User Two",
			Message:        "close #2",
		},
	}

	user := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	repo.Owner = user

	commentBean := &Comment{
		Type:      CommentTypeCommitRef,
		CommitSHA: "abcdef1",
		PosterID:  user.ID,
		IssueID:   1,
	}
	issueBean := &Issue{RepoID: repo.ID, Index: 2}

	AssertNotExistsBean(t, commentBean)
	AssertNotExistsBean(t, &Issue{RepoID: repo.ID, Index: 2}, "is_closed=1")
	assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, repo.DefaultBranch))
	AssertExistsAndLoadBean(t, commentBean)
	AssertExistsAndLoadBean(t, issueBean, "is_closed=1")
	CheckConsistencyFor(t, &Action{})

	// Test that push to a non-default branch closes no issue.
	pushCommits = []*PushCommit{
		{
			Sha1:           "abcdef1",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user4@example.com",
			AuthorName:     "User Four",
			Message:        "close #1",
		},
	}
	repo = AssertExistsAndLoadBean(t, &Repository{ID: 3}).(*Repository)
	commentBean = &Comment{
		Type:      CommentTypeCommitRef,
		CommitSHA: "abcdef1",
		PosterID:  user.ID,
		IssueID:   6,
	}
	issueBean = &Issue{RepoID: repo.ID, Index: 1}

	AssertNotExistsBean(t, commentBean)
	AssertNotExistsBean(t, &Issue{RepoID: repo.ID, Index: 1}, "is_closed=1")
	assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, "non-existing-branch"))
	AssertExistsAndLoadBean(t, commentBean)
	AssertNotExistsBean(t, issueBean, "is_closed=1")
	CheckConsistencyFor(t, &Action{})
}

func TestUpdateIssuesCommit_Issue5957(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	user := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)

	// Test that push to a non-default branch closes an issue.
	pushCommits := []*PushCommit{
		{
			Sha1:           "abcdef1",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user4@example.com",
			AuthorName:     "User Four",
			Message:        "close #2",
		},
	}

	repo := AssertExistsAndLoadBean(t, &Repository{ID: 2}).(*Repository)
	commentBean := &Comment{
		Type:      CommentTypeCommitRef,
		CommitSHA: "abcdef1",
		PosterID:  user.ID,
		IssueID:   7,
	}

	issueBean := &Issue{RepoID: repo.ID, Index: 2, ID: 7}

	AssertNotExistsBean(t, commentBean)
	AssertNotExistsBean(t, issueBean, "is_closed=1")
	assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, "non-existing-branch"))
	AssertExistsAndLoadBean(t, commentBean)
	AssertExistsAndLoadBean(t, issueBean, "is_closed=1")
	CheckConsistencyFor(t, &Action{})
}

func TestGetFeeds(t *testing.T) {
	// test with an individual user
	assert.NoError(t, PrepareTestDatabase())
	user := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)

	actions, err := GetFeeds(GetFeedsOptions{
		RequestedUser:    user,
		RequestingUserID: user.ID,
		IncludePrivate:   true,
		OnlyPerformedBy:  false,
		IncludeDeleted:   true,
	})
	assert.NoError(t, err)
	if assert.Len(t, actions, 1) {
		assert.EqualValues(t, 1, actions[0].ID)
		assert.EqualValues(t, user.ID, actions[0].UserID)
	}

	actions, err = GetFeeds(GetFeedsOptions{
		RequestedUser:    user,
		RequestingUserID: user.ID,
		IncludePrivate:   false,
		OnlyPerformedBy:  false,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 0)
}

func TestGetFeeds2(t *testing.T) {
	// test with an organization user
	assert.NoError(t, PrepareTestDatabase())
	org := AssertExistsAndLoadBean(t, &User{ID: 3}).(*User)
	const userID = 2 // user2 is an owner of the organization

	actions, err := GetFeeds(GetFeedsOptions{
		RequestedUser:    org,
		RequestingUserID: userID,
		IncludePrivate:   true,
		OnlyPerformedBy:  false,
		IncludeDeleted:   true,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 1)
	if assert.Len(t, actions, 1) {
		assert.EqualValues(t, 2, actions[0].ID)
		assert.EqualValues(t, org.ID, actions[0].UserID)
	}

	actions, err = GetFeeds(GetFeedsOptions{
		RequestedUser:    org,
		RequestingUserID: userID,
		IncludePrivate:   false,
		OnlyPerformedBy:  false,
		IncludeDeleted:   true,
	})
	assert.NoError(t, err)
	assert.Len(t, actions, 0)
}
