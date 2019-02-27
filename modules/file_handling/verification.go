package file_handling

import (
	"code.gitea.io/git"
	"code.gitea.io/gitea/models"
	"code.gitea.io/sdk/gitea"
)

func GetPayloadCommitVerification(commit *git.Commit) *gitea.PayloadCommitVerification {
	verification := &gitea.PayloadCommitVerification{}
	commitVerification := models.ParseCommitWithSignature(commit)
	if commit.Signature != nil {
		verification.Signature = commit.Signature.Signature
		verification.Payload = commit.Signature.Payload
	}
	if verification.Reason != "" {
		verification.Reason = commitVerification.Reason
	} else {
		if verification.Verified {
			verification.Reason = "unsigned"
		}
	}
	return verification
}
