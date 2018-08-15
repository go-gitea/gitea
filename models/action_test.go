package models

import (
	"fmt"
	"path"
	"strings"
	"testing"

	"code.gitea.io/git"
	"code.gitea.io/gitea/modules/base"
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

	_, err := x.ID(repo.ID).Cols("name", "lower_name").Update(repo)
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

func TestUpdateIssuesCommentIssues(t *testing.T) {
	for _, canOpenClose := range []bool{false, true} {
		// if cannot open or close then issue should not change status
		isOpen := "is_closed!=1"
		isClosed := "is_closed=1"
		if !canOpenClose {
			isClosed = isOpen
		}

		assert.NoError(t, PrepareTestDatabase())
		user := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
		repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
		repo.Owner = user

		commentIssue := AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 2}).(*Issue)
		refIssue1 := AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}).(*Issue)
		refIssue2 := AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 3}).(*Issue)
		assert.EqualValues(t, true, commentIssue.IsPull)
		assert.EqualValues(t, false, refIssue1.IsClosed)
		assert.EqualValues(t, false, refIssue2.IsClosed)

		// TODO: comments are loaded and then this doesnt work...
		commentBean := []*Comment{
			{
				Type:      CommentTypePullRef,
				CommitSHA: base.EncodeSha1(fmt.Sprintf("%d", commentIssue.ID)),
				PosterID:  user.ID,
				IssueID:   commentIssue.ID,
			},
			{
				Type:      CommentTypePullRef,
				CommitSHA: base.EncodeSha1(fmt.Sprintf("%d", commentIssue.ID)),
				PosterID:  user.ID,
				IssueID:   refIssue1.ID,
			},
			{
				Type:      CommentTypePullRef,
				CommitSHA: base.EncodeSha1(fmt.Sprintf("%d", commentIssue.ID)),
				PosterID:  user.ID,
				IssueID:   refIssue2.ID,
			},
		}

		// test issue/pull request closing multiple issues
		commentIssue.Title = "close #1"
		commentIssue.Content = "close #3"
		AssertNotExistsBean(t, commentBean[0])
		AssertNotExistsBean(t, commentBean[1])
		AssertNotExistsBean(t, commentBean[2])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isOpen)
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 3}, isOpen)
		assert.NoError(t, UpdateIssuesComment(user, repo, commentIssue, nil, canOpenClose))
		AssertNotExistsBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, commentBean[1])
		AssertExistsAndLoadBean(t, commentBean[2])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isClosed)
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 3}, isClosed)
		CheckConsistencyFor(t, &Action{})

		// test issue/pull request re-opening multiple issues
		commentIssue.Title = "reopen #1"
		commentIssue.Content = "reopen #3"
		AssertNotExistsBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, commentBean[1])
		AssertExistsAndLoadBean(t, commentBean[2])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isClosed)
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 3}, isClosed)
		assert.NoError(t, UpdateIssuesComment(user, repo, commentIssue, nil, canOpenClose))
		AssertNotExistsBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, commentBean[1])
		AssertExistsAndLoadBean(t, commentBean[2])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isOpen)
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 3}, isOpen)
		CheckConsistencyFor(t, &Action{})

		// test issue/pull request mixing re-opening and closing issue and self-referencing issue
		commentIssue.Title = "reopen #3"
		commentIssue.Content = "close #3 and reference #2"
		AssertNotExistsBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, commentBean[1])
		AssertExistsAndLoadBean(t, commentBean[2])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 3}, isOpen)
		assert.NoError(t, UpdateIssuesComment(user, repo, commentIssue, nil, canOpenClose))
		AssertNotExistsBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, commentBean[1])
		AssertExistsAndLoadBean(t, commentBean[2])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 3}, isOpen)
		CheckConsistencyFor(t, &Action{})
	}
}

func TestUpdateIssuesCommentComments(t *testing.T) {
	isOpen := "is_closed!=1"

	assert.NoError(t, PrepareTestDatabase())
	user := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
	repo.Owner = user

	commentIssue := AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 2}).(*Issue)
	refIssue1 := AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}).(*Issue)
	refIssue2 := AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 3}).(*Issue)
	assert.EqualValues(t, true, commentIssue.IsPull)
	assert.EqualValues(t, false, refIssue1.IsClosed)
	assert.EqualValues(t, false, refIssue2.IsClosed)

	comment := Comment{
		ID:       123456789,
		Type:     CommentTypeComment,
		PosterID: user.ID,
		Poster:   user,
		IssueID:  commentIssue.ID,
		Content:  "",
	}

	commentBean := []*Comment{
		{
			Type:      CommentTypeCommentRef,
			CommitSHA: base.EncodeSha1(fmt.Sprintf("%d", comment.ID)),
			PosterID:  user.ID,
			IssueID:   commentIssue.ID,
		},
		{
			Type:      CommentTypeCommentRef,
			CommitSHA: base.EncodeSha1(fmt.Sprintf("%d", comment.ID)),
			PosterID:  user.ID,
			IssueID:   refIssue1.ID,
		},
		{
			Type:      CommentTypeCommentRef,
			CommitSHA: base.EncodeSha1(fmt.Sprintf("%d", comment.ID)),
			PosterID:  user.ID,
			IssueID:   refIssue2.ID,
		},
	}

	// test comment referencing issue including self-referencing
	comment.Content = "this is a comment that mentions #1 and #2 too"
	AssertNotExistsBean(t, commentBean[0])
	AssertNotExistsBean(t, commentBean[1])
	AssertNotExistsBean(t, commentBean[2])
	AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isOpen)
	AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 3}, isOpen)
	assert.NoError(t, UpdateIssuesComment(user, repo, commentIssue, &comment, false))
	AssertNotExistsBean(t, commentBean[0])
	AssertExistsAndLoadBean(t, commentBean[1])
	AssertNotExistsBean(t, commentBean[2])
	AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isOpen)
	AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 3}, isOpen)
	CheckConsistencyFor(t, &Action{})

	// test comment updating issue reference
	comment.Content = "this is a comment that mentions #1 and #3 too"
	AssertNotExistsBean(t, commentBean[0])
	AssertExistsAndLoadBean(t, commentBean[1])
	AssertNotExistsBean(t, commentBean[2])
	AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isOpen)
	AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 3}, isOpen)
	assert.NoError(t, UpdateIssuesComment(user, repo, commentIssue, &comment, false))
	AssertNotExistsBean(t, commentBean[0])
	AssertExistsAndLoadBean(t, commentBean[1])
	AssertExistsAndLoadBean(t, commentBean[2])
	AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isOpen)
	AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 3}, isOpen)
	CheckConsistencyFor(t, &Action{})
}

type Message struct {
	Message string
	Sha1    string
	IssueID int64
}

func preparePushCommits(userID int64, repoID int64, messages []*Message) ([]*PushCommit, []*Comment) {
	var pushCommits []*PushCommit
	var commentBean []*Comment

	for _, message := range messages {
		pushCommits = append(pushCommits, &PushCommit{
			Sha1:           message.Sha1,
			CommitterEmail: "user2@example.com",
			CommitterName:  "User Two",
			AuthorEmail:    "user2@example.com",
			AuthorName:     "User Two",
			Message:        message.Message,
		})
		commentBean = append(commentBean, &Comment{
			Type:      CommentTypeCommitRef,
			CommitSHA: base.EncodeSha1(fmt.Sprintf("%d %s", repoID, message.Sha1)),
			PosterID:  userID,
			IssueID:   message.IssueID,
		})
	}

	return pushCommits, commentBean
}

func TestUpdateIssuesCommit(t *testing.T) {
	for _, commitsAreMerged := range []bool{false, true} {
		// if commits were not merged then issue should not change status
		isOpen := "is_closed!=1"
		isClosed := "is_closed=1"
		if !commitsAreMerged {
			isClosed = isOpen
		}

		assert.NoError(t, PrepareTestDatabase())
		user := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
		repo := AssertExistsAndLoadBean(t, &Repository{ID: 1}).(*Repository)
		repo.Owner = user

		commitIssue := AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 2}).(*Issue)
		refIssue := AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}).(*Issue)
		assert.EqualValues(t, true, commitIssue.IsPull)
		assert.EqualValues(t, false, refIssue.IsClosed)

		// test re-open of already open issue
		pushCommits, commentBean := preparePushCommits(user.ID, repo.ID, []*Message{
			{
				Message: "reopen #1",
				Sha1:    "abcdef1",
				IssueID: refIssue.ID,
			},
		})
		AssertNotExistsBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isOpen)
		assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, commitsAreMerged))
		AssertExistsAndLoadBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isOpen)
		CheckConsistencyFor(t, &Action{})

		// test simultaneous open and close on an already open issue
		pushCommits, commentBean = preparePushCommits(user.ID, repo.ID, []*Message{
			{
				Message: "reopen #1 and then close #1",
				Sha1:    "abcdef2",
				IssueID: refIssue.ID,
			},
		})
		AssertNotExistsBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isOpen)
		assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, commitsAreMerged))
		AssertExistsAndLoadBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isOpen)
		CheckConsistencyFor(t, &Action{})

		// test close of an open issue
		pushCommits, commentBean = preparePushCommits(user.ID, repo.ID, []*Message{
			{
				Message: "closes #1",
				Sha1:    "abcdef3",
				IssueID: refIssue.ID,
			},
		})
		AssertNotExistsBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isOpen)
		assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, commitsAreMerged))
		AssertExistsAndLoadBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isClosed)
		CheckConsistencyFor(t, &Action{})

		// test close of an already closed issue
		pushCommits, commentBean = preparePushCommits(user.ID, repo.ID, []*Message{
			{
				Message: "closes #1",
				Sha1:    "abcdef4",
				IssueID: refIssue.ID,
			},
		})
		AssertNotExistsBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isClosed)
		assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, commitsAreMerged))
		AssertExistsAndLoadBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isClosed)
		CheckConsistencyFor(t, &Action{})

		// test simultaneous open and close on a closed issue
		pushCommits, commentBean = preparePushCommits(user.ID, repo.ID, []*Message{
			{
				Message: "close #1 and reopen #1",
				Sha1:    "abcdef5",
				IssueID: refIssue.ID,
			},
		})
		AssertNotExistsBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isClosed)
		assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, commitsAreMerged))
		AssertExistsAndLoadBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isClosed)
		CheckConsistencyFor(t, &Action{})

		// test referencing an closed issue
		pushCommits, commentBean = preparePushCommits(user.ID, repo.ID, []*Message{
			{
				Message: "for details on how to open, see #1",
				Sha1:    "abcdef6",
				IssueID: refIssue.ID,
			},
		})
		AssertNotExistsBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isClosed)
		assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, commitsAreMerged))
		AssertExistsAndLoadBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isClosed)
		CheckConsistencyFor(t, &Action{})

		// test re-open a closed issue
		pushCommits, commentBean = preparePushCommits(user.ID, repo.ID, []*Message{
			{
				Message: "reopens #1",
				Sha1:    "abcdef7",
				IssueID: refIssue.ID,
			},
		})
		AssertNotExistsBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isClosed)
		assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, commitsAreMerged))
		AssertExistsAndLoadBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isOpen)
		CheckConsistencyFor(t, &Action{})

		// test referencing an open issue
		pushCommits, commentBean = preparePushCommits(user.ID, repo.ID, []*Message{
			{
				Message: "for details on how to close, see #1",
				Sha1:    "abcdef8",
				IssueID: refIssue.ID,
			},
		})
		AssertNotExistsBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isOpen)
		assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, commitsAreMerged))
		AssertExistsAndLoadBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isOpen)
		CheckConsistencyFor(t, &Action{})

		// test close-then-open commit order
		pushCommits, commentBean = preparePushCommits(user.ID, repo.ID, []*Message{
			{
				Message: "reopened #1",
				Sha1:    "abcdef10",
				IssueID: refIssue.ID,
			},
			{
				Message: "fixes #1",
				Sha1:    "abcdef9",
				IssueID: refIssue.ID,
			},
		})
		AssertNotExistsBean(t, commentBean[0])
		AssertNotExistsBean(t, commentBean[1])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isOpen)
		assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, commitsAreMerged))
		AssertExistsAndLoadBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, commentBean[1])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isOpen)
		CheckConsistencyFor(t, &Action{})

		// test open-then-close commit order
		pushCommits, commentBean = preparePushCommits(user.ID, repo.ID, []*Message{
			{
				Message: "resolved #1",
				Sha1:    "abcdef12",
				IssueID: refIssue.ID,
			},
			{
				Message: "reopened #1",
				Sha1:    "abcdef11",
				IssueID: refIssue.ID,
			},
		})
		AssertNotExistsBean(t, commentBean[0])
		AssertNotExistsBean(t, commentBean[1])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isOpen)
		assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, commitsAreMerged))
		AssertExistsAndLoadBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, commentBean[1])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isClosed)
		CheckConsistencyFor(t, &Action{})

		// test more complex commit pattern
		pushCommits, commentBean = preparePushCommits(user.ID, repo.ID, []*Message{
			{
				Message: "start working on #FST-1, #2",
				Sha1:    "abcdef15",
				IssueID: commitIssue.ID,
			},
			{
				Message: "reopen #1",
				Sha1:    "abcdef14",
				IssueID: refIssue.ID,
			},
			{
				Message: "close #1",
				Sha1:    "abcdef13",
				IssueID: refIssue.ID,
			},
		})
		AssertNotExistsBean(t, commentBean[0])
		AssertNotExistsBean(t, commentBean[1])
		AssertNotExistsBean(t, commentBean[2])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isClosed)
		assert.NoError(t, UpdateIssuesCommit(user, repo, pushCommits, commitsAreMerged))
		AssertExistsAndLoadBean(t, commentBean[0])
		AssertExistsAndLoadBean(t, commentBean[1])
		AssertExistsAndLoadBean(t, commentBean[2])
		AssertExistsAndLoadBean(t, &Issue{RepoID: repo.ID, Index: 1}, isOpen)
		CheckConsistencyFor(t, &Action{})
	}
}

func testCorrectRepoAction(t *testing.T, opts CommitRepoActionOptions, actionBean *Action) {
	AssertNotExistsBean(t, actionBean)
	assert.NoError(t, CommitRepoAction(opts))
	AssertExistsAndLoadBean(t, actionBean)
	CheckConsistencyFor(t, &Action{})
}

func TestCommitRepoAction(t *testing.T) {
	samples := []struct {
		userID                  int64
		repositoryID            int64
		commitRepoActionOptions CommitRepoActionOptions
		action                  Action
	}{
		{
			userID:       2,
			repositoryID: 2,
			commitRepoActionOptions: CommitRepoActionOptions{
				RefFullName: "refName",
				OldCommitID: "oldCommitID",
				NewCommitID: "newCommitID",
				Commits: &PushCommits{
					avatars: make(map[string]string),
					Commits: []*PushCommit{
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
					},
					Len: 2,
				},
			},
			action: Action{
				OpType:  ActionCommitRepo,
				RefName: "refName",
			},
		},
		{
			userID:       2,
			repositoryID: 1,
			commitRepoActionOptions: CommitRepoActionOptions{
				RefFullName: git.TagPrefix + "v1.1",
				OldCommitID: git.EmptySHA,
				NewCommitID: "newCommitID",
				Commits:     &PushCommits{},
			},
			action: Action{
				OpType:  ActionPushTag,
				RefName: "v1.1",
			},
		},
		{
			userID:       2,
			repositoryID: 1,
			commitRepoActionOptions: CommitRepoActionOptions{
				RefFullName: git.TagPrefix + "v1.1",
				OldCommitID: "oldCommitID",
				NewCommitID: git.EmptySHA,
				Commits:     &PushCommits{},
			},
			action: Action{
				OpType:  ActionDeleteTag,
				RefName: "v1.1",
			},
		},
		{
			userID:       2,
			repositoryID: 1,
			commitRepoActionOptions: CommitRepoActionOptions{
				RefFullName: git.BranchPrefix + "feature/1",
				OldCommitID: "oldCommitID",
				NewCommitID: git.EmptySHA,
				Commits:     &PushCommits{},
			},
			action: Action{
				OpType:  ActionDeleteBranch,
				RefName: "feature/1",
			},
		},
	}

	for _, s := range samples {
		PrepareTestEnv(t)

		user := AssertExistsAndLoadBean(t, &User{ID: s.userID}).(*User)
		repo := AssertExistsAndLoadBean(t, &Repository{ID: s.repositoryID, OwnerID: user.ID}).(*Repository)
		repo.Owner = user

		s.commitRepoActionOptions.PusherName = user.Name
		s.commitRepoActionOptions.RepoOwnerID = user.ID
		s.commitRepoActionOptions.RepoName = repo.Name

		s.action.ActUserID = user.ID
		s.action.RepoID = repo.ID
		s.action.Repo = repo
		s.action.IsPrivate = repo.IsPrivate

		testCorrectRepoAction(t, s.commitRepoActionOptions, &s.action)
	}
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

	_, err := x.ID(repo.ID).Cols("owner_id").Update(repo)
	assert.NoError(t, err)
	CheckConsistencyFor(t, &Action{})
}

func TestMergePullRequestAction(t *testing.T) {
	assert.NoError(t, PrepareTestDatabase())
	user := AssertExistsAndLoadBean(t, &User{ID: 2}).(*User)
	repo := AssertExistsAndLoadBean(t, &Repository{ID: 1, OwnerID: user.ID}).(*Repository)
	repo.Owner = user
	issue := AssertExistsAndLoadBean(t, &Issue{ID: 3, RepoID: repo.ID}).(*Issue)
	commits := &PushCommits{0, make([]*PushCommit, 0), "", nil}

	actionBean := &Action{
		OpType:    ActionMergePullRequest,
		ActUserID: user.ID,
		ActUser:   user,
		RepoID:    repo.ID,
		Repo:      repo,
		IsPrivate: repo.IsPrivate,
	}
	AssertNotExistsBean(t, actionBean)
	assert.NoError(t, MergePullRequestAction(user, repo, issue, commits))
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
