package models

import (
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
			Sha1:           "69554a6",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User2",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User2",
			Message:        "not signed commit",
		},
		{
			Sha1:           "27566bd",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User2",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User2",
			Message:        "good signed commit (with not yet validated email)",
		},
		{
			Sha1:           "5099b81",
			CommitterEmail: "user2@example.com",
			CommitterName:  "User2",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User2",
			Message:        "good signed commit",
		},
	}
	pushCommits.Len = len(pushCommits.Commits)

	repo := AssertExistsAndLoadBean(t, &Repository{ID: 16}).(*Repository)
	payloadCommits, err := pushCommits.ToAPIPayloadCommits(repo.RepoPath(), "/user2/repo16")
	assert.NoError(t, err)
	assert.EqualValues(t, 3, len(payloadCommits))

	assert.Equal(t, "69554a6", payloadCommits[0].ID)
	assert.Equal(t, "not signed commit", payloadCommits[0].Message)
	assert.Equal(t, "/user2/repo16/commit/69554a6", payloadCommits[0].URL)
	assert.Equal(t, "User2", payloadCommits[0].Committer.Name)
	assert.Equal(t, "user2", payloadCommits[0].Committer.UserName)
	assert.Equal(t, "User2", payloadCommits[0].Author.Name)
	assert.Equal(t, "user2", payloadCommits[0].Author.UserName)
	assert.EqualValues(t, []string{}, payloadCommits[0].Added)
	assert.EqualValues(t, []string{}, payloadCommits[0].Removed)
	assert.EqualValues(t, []string{"readme.md"}, payloadCommits[0].Modified)

	assert.Equal(t, "27566bd", payloadCommits[1].ID)
	assert.Equal(t, "good signed commit (with not yet validated email)", payloadCommits[1].Message)
	assert.Equal(t, "/user2/repo16/commit/27566bd", payloadCommits[1].URL)
	assert.Equal(t, "User2", payloadCommits[1].Committer.Name)
	assert.Equal(t, "user2", payloadCommits[1].Committer.UserName)
	assert.Equal(t, "User2", payloadCommits[1].Author.Name)
	assert.Equal(t, "user2", payloadCommits[1].Author.UserName)
	assert.EqualValues(t, []string{}, payloadCommits[1].Added)
	assert.EqualValues(t, []string{}, payloadCommits[1].Removed)
	assert.EqualValues(t, []string{"readme.md"}, payloadCommits[1].Modified)

	assert.Equal(t, "5099b81", payloadCommits[2].ID)
	assert.Equal(t, "good signed commit", payloadCommits[2].Message)
	assert.Equal(t, "/user2/repo16/commit/5099b81", payloadCommits[2].URL)
	assert.Equal(t, "User2", payloadCommits[2].Committer.Name)
	assert.Equal(t, "user2", payloadCommits[2].Committer.UserName)
	assert.Equal(t, "User2", payloadCommits[2].Author.Name)
	assert.Equal(t, "user2", payloadCommits[2].Author.UserName)
	assert.EqualValues(t, []string{"readme.md"}, payloadCommits[2].Added)
	assert.EqualValues(t, []string{}, payloadCommits[2].Removed)
	assert.EqualValues(t, []string{}, payloadCommits[2].Modified)
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
		"/suburl/user/avatar/user2/-1",
		pushCommits.AvatarLink("user2@example.com"))

	assert.Equal(t,
		"https://secure.gravatar.com/avatar/19ade630b94e1e0535b3df7387434154?d=identicon",
		pushCommits.AvatarLink("nonexistent@example.com"))
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
