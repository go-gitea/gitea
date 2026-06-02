package gituser

import (
	"context"

	"gitea.dev/models/user"
	"gitea.dev/modules/container"
	"gitea.dev/modules/git"
)

// CommitParticipant is one participant of a commit (its author or a co-author): a git
// identity, optionally matched to a Gitea user.
type CommitParticipant struct {
	GiteaUser   *user.User     // matched Gitea user, nil if unmatched
	GitIdentity *git.Signature // git identity (name/email)
}

// UserCommit represents a commit with validation of user.
type UserCommit struct {
	GiteaUser *user.User
	GitCommit *git.Commit

	AllParticipants []*CommitParticipant // it includes the author and all co-authors (TODO: why not include the committer?)
}

// ValidateCommitsWithEmails checks if authors' e-mails of commits are corresponding to users.
func ValidateCommitsWithEmails(ctx context.Context, oldCommits []*git.Commit) ([]*UserCommit, error) {
	var (
		newCommits = make([]*UserCommit, 0, len(oldCommits))
		emailSet   = make(container.Set[string])
	)
	for _, c := range oldCommits {
		if c.Author != nil {
			emailSet.Add(c.Author.Email)
		}
		for _, sig := range c.AllAuthorSignatures() {
			emailSet.Add(sig.Email)
		}
	}

	emailUserMap, err := user.GetUsersByEmails(ctx, emailSet.Values())
	if err != nil {
		return nil, err
	}

	for _, c := range oldCommits {
		newCommits = append(newCommits, &UserCommit{
			GiteaUser:       emailUserMap.GetByEmail(c.Author.Email), // FIXME: why ValidateCommitsWithEmails uses "Author", but ParseCommitsWithSignature uses "Committer"?
			AllParticipants: CommitParticipantsFromSigs(c.AllAuthorSignatures(), emailUserMap),
			GitCommit:       c,
		})
	}
	return newCommits, nil
}

// AvatarStackData returns the view-model for rendering this commit's author + co-authors.
func (uc *UserCommit) AvatarStackData() *AvatarStackData {
	if uc == nil {
		return nil
	}
	var sig *git.Signature
	if uc.GitCommit != nil {
		sig = uc.GitCommit.Author
	}
	return NewAvatarStackData(uc.GiteaUser, sig, uc.AllParticipants)
}
