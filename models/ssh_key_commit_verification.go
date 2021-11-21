// Copyright 2021 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bytes"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"github.com/42wim/sshsig"
)

// ParseCommitWithSSHSignature check if signature is good against keystore.
func ParseCommitWithSSHSignature(c *git.Commit, committer *User) *CommitVerification {
	// Now try to associate the signature with the committer, if present
	if committer.ID != 0 {
		keys, err := ListPublicKeys(committer.ID, db.ListOptions{})
		if err != nil { // Skipping failed to get ssh keys of user
			log.Error("ListPublicKeys: %v", err)
			return &CommitVerification{
				CommittingUser: committer,
				Verified:       false,
				Reason:         "gpg.error.failed_retrieval_gpg_keys",
			}
		}

		committerEmailAddresses, _ := user_model.GetEmailAddresses(committer.ID)
		activated := false
		for _, e := range committerEmailAddresses {
			if e.IsActivated && strings.EqualFold(e.Email, c.Committer.Email) {
				activated = true
				break
			}
		}

		for _, k := range keys {
			canValidate := false
			email := ""
			if k.Verified && activated {
				canValidate = true
				email = c.Committer.Email
			}

			if !canValidate {
				continue // Skip this key
			}

			commitVerification := verifySSHCommitVerification(c.Signature.Signature, c.Signature.Payload, k, committer, committer, email)
			if commitVerification != nil {
				return commitVerification
			}
		}
	}

	return &CommitVerification{
		CommittingUser: committer,
		Verified:       false,
		Reason:         BadSignature,
	}
}

func verifySSHCommitVerification(sig, payload string, k *PublicKey, committer, signer *User, email string) *CommitVerification {
	if err := sshsig.Verify(bytes.NewBuffer([]byte(payload)), []byte(sig), []byte(k.Content), "git"); err != nil {
		return nil
	}

	return &CommitVerification{ // Everything is ok
		CommittingUser: committer,
		Verified:       true,
		Reason:         fmt.Sprintf("%s / %s", signer.Name, k.Fingerprint),
		SigningUser:    signer,
		SigningSSHKey:  k,
		SigningEmail:   email,
	}
}
