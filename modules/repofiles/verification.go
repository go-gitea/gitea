// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package repofiles

import (
	"code.gitea.io/gitea/models"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/structs"
)

// GetPayloadCommitVerification returns the verification information of a commit
func GetPayloadCommitVerification(commit *git.Commit) *structs.PayloadCommitVerification {
	verification := &structs.PayloadCommitVerification{}
	commitVerification := models.ParseCommitWithSignature(commit)
	if commit.Signature != nil {
		verification.Signature = commit.Signature.Signature
		verification.Payload = commit.Signature.Payload
	}
	if commitVerification.SigningUser != nil {
		verification.Signer = &structs.PayloadUser{
			Name:  commitVerification.SigningUser.Name,
			Email: commitVerification.SigningUser.Email,
		}
	}
	verification.Verified = commitVerification.Verified
	verification.Reason = commitVerification.Reason
	if verification.Reason == "" && !verification.Verified {
		verification.Reason = "gpg.error.not_signed_commit"
	}
	return verification
}
