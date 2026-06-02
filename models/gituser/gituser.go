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
	AuthorUser      *user.User
	GitCommit       *git.Commit
	AvatarStackData *AvatarStackData
}

// GetUserCommitsByGitCommits checks if authors' e-mails of commits are corresponding to users.
func GetUserCommitsByGitCommits(ctx context.Context, gitCommits []*git.Commit) ([]*UserCommit, error) {
	userCommits := make([]*UserCommit, 0, len(gitCommits))
	emailSet := make(container.Set[string])
	for _, c := range gitCommits {
		emailSet.Add(c.Author.Email)
		emailSet.Add(c.Committer.Email)
		for _, p := range c.AllParticipantIdentities() {
			emailSet.Add(p.Email)
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
			AvatarStackData: BuildAvatarStackData(ctx, c.AllParticipantIdentities(), emailUserMap),
		})
	}
	return userCommits, nil
}
