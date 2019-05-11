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
	if verification.Reason != "" {
		verification.Reason = commitVerification.Reason
	} else if verification.Verified {
		verification.Reason = "unsigned"
	}
	return verification
}
