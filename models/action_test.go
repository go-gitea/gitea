package models

import (
	"path"
	"strings"
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

func TestNewRepoAction(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	repo := AssertExistsAndLoadBean(t, &Repository{OwnerID: user.ID}).(*Repository)
	repo.Owner = user

	actionBean := &Action{
		OpType:    ActionCreateRepo,
		ActUserID: user.ID,
		RepoID:    repo.ID,
		ActUser:   user,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
	}

	AssertNotExistsBean(t, actionBean)
	assert.NoError(t, NewRepoAction(user, repo))
	AssertExistsAndLoadBean(t, actionBean)
	CheckConsistencyFor(t, &Action{})
}

func TestRenameRepoAction(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	repo := AssertExistsAndLoadBean(t, &Repository{OwnerID: user.ID}).(*Repository)
	repo.Owner = user

	oldRepoName := repo.Name
	const newRepoName = "newRepoName"
	repo.Name = newRepoName
	repo.LowerName = strings.ToLower(newRepoName)

	actionBean := &Action{
		OpType:    ActionRenameRepo,
		ActUserID: user.ID,
		ActUser:   user,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
		Content:   oldRepoName,
	}
	AssertNotExistsBean(t, actionBean)
	assert.NoError(t, RenameRepoAction(user, oldRepoName, repo))
	AssertExistsAndLoadBean(t, actionBean)

	_, err := x.Id(repo.ID).Cols("name", "lower_name").Update(repo)
	assert.NoError(t, err)
	CheckConsistencyFor(t, &Action{})
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
		"https://secure.gravatar.com/avatar/ab53a2911ddf9b4817ac01ddcd3d975f",
		pushCommits.AvatarLink("user2@example.com"))

	assert.Equal(t,
		"https://secure.gravatar.com/avatar/19ade630b94e1e0535b3df7387434154",
		pushCommits.AvatarLink("nonexistent@example.com"))
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
	assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits))
	AssertExistsAndLoadBean(t, commentBean)
	AssertExistsAndLoadBean(t, issueBean, "is_closed=1")
	CheckConsistencyFor(t, &Action{})
}

func TestCommitRepoAction(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 2, OwnerID: user.ID}).(*Repository)
	repo.Owner = user

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

	actionBean := &Action{
		OpType:    ActionCommitRepo,
		ActUserID: user.ID,
		ActUser:   user,
		RepoID:    repo.ID,
		Repo:      repo,
		RefName:   "refName",
		IsPrivate: repo.IsPrivate,
	}
	AssertNotExistsBean(t, actionBean)
	assert.NoError(t, CommitRepoAction(CommitRepoActionOptions{
		PusherName:  user.Name,
		RepoOwnerID: user.ID,
		RepoName:    repo.Name,
		RefFullName: "refName",
		OldCommitID: "oldCommitID",
		NewCommitID: "newCommitID",
		Commits:     pushCommits,
	}))
	AssertExistsAndLoadBean(t, actionBean)
	CheckConsistencyFor(t, &Action{})
}

func TestTransferRepoAction(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())

	user2 := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	user4 := AssertExistsAndLoadBean(t, &User{ID: 4}).(*User)
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1, OwnerID: user2.ID}).(*Repository)

	repo.OwnerID = user4.ID
	repo.Owner = user4

	actionBean := &Action{
		OpType:    ActionTransferRepo,
		ActUserID: user2.ID,
		ActUser:   user2,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
	}
	AssertNotExistsBean(t, actionBean)
	assert.NoError(t, TransferRepoAction(user2, user2, repo))
	AssertExistsAndLoadBean(t, actionBean)

	_, err := x.Id(repo.ID).Cols("owner_id").Update(repo)
	assert.NoError(t, err)
	CheckConsistencyFor(t, &Action{})
}

func TestMergePullRequestAction(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	user := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1, OwnerID: user.ID}).(*Repository)
	repo.Owner = user
	issue := AssertExistsAndLoadBean(t, &Issue{ID: 3, RepoID: repo.ID}).(*Issue)

	actionBean := &Action{
		OpType:    ActionMergePullRequest,
		ActUserID: user.ID,
		ActUser:   user,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
	}
	AssertNotExistsBean(t, actionBean)
	assert.NoError(t, MergePullRequestAction(user, repo, issue))
	AssertExistsAndLoadBean(t, actionBean)
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
	userID := AssertExistsAndLoadBean(t, &OrgUser{OrgID: org.ID, IsOwner: true}).(*OrgUser).UID

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
