package gituser

import (
	"context"

	"gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
)

// CommitParticipant is one participant of a commit (its author or a co-author):
// a git identity, optionally matched to a Gitea user.
type CommitParticipant struct {
	GiteaUser   *user.User          // matched Gitea user, nil if unmatched
	GitIdentity *git.CommitIdentity // git identity (name/email)
}

// UserCommit represents a commit with matched of database "author" user.
type UserCommit struct {
	AuthorUser *user.User
	GitCommit  *git.Commit
}

// GetUserCommitsByGitCommits checks if authors' e-mails of commits are corresponding to users.
func GetUserCommitsByGitCommits(ctx context.Context, gitCommits []*git.Commit) ([]*UserCommit, error) {
	userCommits := make([]*UserCommit, 0, len(gitCommits))
	emailSet := make(container.Set[string])
	for _, c := range gitCommits {
		if c.Author != nil {
			emailSet.Add(c.Author.Email)
		}
	}

	emailUserMap, err := user.GetUsersByEmails(ctx, emailSet.Values())
	if err != nil {
		return nil, err
	}

	for _, c := range gitCommits {
		userCommits = append(userCommits, &UserCommit{
			AuthorUser: emailUserMap.GetByEmail(c.Author.Email), // FIXME: why GetUserCommitsByGitCommits uses "Author", but ParseCommitsWithSignature uses "Committer"?
			GitCommit:  c,
		})
	}
	return userCommits, nil
}

// AvatarStackData returns the view-model for rendering this commit's author + co-authors.
func (uc *UserCommit) AvatarStackData(ctx context.Context) *AvatarStackData {
	return BuildAvatarStackData(ctx, uc.GitCommit.AllParticipantIdentities(), nil)
}
