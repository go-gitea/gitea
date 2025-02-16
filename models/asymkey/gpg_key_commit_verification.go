// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"fmt"
	"hash"

	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"

	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

//   __________________  ________   ____  __.
//  /  _____/\______   \/  _____/  |    |/ _|____ ___.__.
// /   \  ___ |     ___/   \  ___  |      <_/ __ <   |  |
// \    \_\  \|    |   \    \_\  \ |    |  \  ___/\___  |
//  \______  /|____|    \______  / |____|__ \___  > ____|
//         \/                  \/          \/   \/\/
// _________                        .__  __
// \_   ___ \  ____   _____   _____ |__|/  |_
// /    \  \/ /  _ \ /     \ /     \|  \   __\
// \     \___(  <_> )  Y Y  \  Y Y  \  ||  |
//  \______  /\____/|__|_|  /__|_|  /__||__|
//         \/             \/      \/
// ____   ____           .__  _____.__               __  .__
// \   \ /   /___________|__|/ ____\__| ____ _____ _/  |_|__| ____   ____
//  \   Y   // __ \_  __ \  \   __\|  |/ ___\\__  \\   __\  |/  _ \ /    \
//   \     /\  ___/|  | \/  ||  |  |  \  \___ / __ \|  | |  (  <_> )   |  \
//    \___/  \___  >__|  |__||__|  |__|\___  >____  /__| |__|\____/|___|  /
//               \/                        \/     \/                    \/

// This file provides functions relating commit verification

// CommitVerification represents a commit validation of signature
type CommitVerification struct {
	Verified       bool
	Warning        bool
	Reason         string
	SigningUser    *user_model.User
	CommittingUser *user_model.User
	SigningEmail   string
	SigningKey     *GPGKey
	SigningSSHKey  *PublicKey
	TrustStatus    string
}

// SignCommit represents a commit with validation of signature.
type SignCommit struct {
	Verification *CommitVerification
	*user_model.UserCommit
}

const (
	// BadSignature is used as the reason when the signature has a KeyID that is in the db
	// but no key that has that ID verifies the signature. This is a suspicious failure.
	BadSignature = "gpg.error.probable_bad_signature"
	// BadDefaultSignature is used as the reason when the signature has a KeyID that matches the
	// default Key but is not verified by the default key. This is a suspicious failure.
	BadDefaultSignature = "gpg.error.probable_bad_default_signature"
	// NoKeyFound is used as the reason when no key can be found to verify the signature.
	NoKeyFound = "gpg.error.no_gpg_keys_found"
)

func verifySign(s *packet.Signature, h hash.Hash, k *GPGKey) error {
	// Check if key can sign
	if !k.CanSign {
		return fmt.Errorf("key can not sign")
	}
	// Decode key
	pkey, err := base64DecPubKey(k.Content)
	if err != nil {
		return err
	}
	return pkey.VerifySignature(h, s)
}

func hashAndVerify(sig *packet.Signature, payload string, k *GPGKey) (*GPGKey, error) {
	// Generating hash of commit
	hash, err := populateHash(sig.Hash, []byte(payload))
	if err != nil { // Skipping as failed to generate hash
		log.Error("PopulateHash: %v", err)
		return nil, err
	}
	// We will ignore errors in verification as they don't need to be propagated up
	err = verifySign(sig, hash, k)
	if err != nil {
		return nil, nil
	}
	return k, nil
}

func hashAndVerifyWithSubKeys(sig *packet.Signature, payload string, k *GPGKey) (*GPGKey, error) {
	verified, err := hashAndVerify(sig, payload, k)
	if err != nil || verified != nil {
		return verified, err
	}
	for _, sk := range k.SubsKey {
		verified, err := hashAndVerify(sig, payload, sk)
		if err != nil || verified != nil {
			return verified, err
		}
	}
	return nil, nil
}

func HashAndVerifyWithSubKeysCommitVerification(sig *packet.Signature, payload string, k *GPGKey, committer, signer *user_model.User, email string) *CommitVerification {
	key, err := hashAndVerifyWithSubKeys(sig, payload, k)
	if err != nil { // Skipping failed to generate hash
		return &CommitVerification{
			CommittingUser: committer,
			Verified:       false,
			Reason:         "gpg.error.generate_hash",
		}
	}

	if key != nil {
		return &CommitVerification{ // Everything is ok
			CommittingUser: committer,
			Verified:       true,
			Reason:         fmt.Sprintf("%s / %s", signer.Name, key.KeyID),
			SigningUser:    signer,
			SigningKey:     key,
			SigningEmail:   email,
		}
	}
	return nil
}

// CalculateTrustStatus will calculate the TrustStatus for a commit verification within a repository
// There are several trust models in Gitea
func CalculateTrustStatus(verification *CommitVerification, repoTrustModel repo_model.TrustModelType, isOwnerMemberCollaborator func(*user_model.User) (bool, error), keyMap *map[string]bool) error {
	if !verification.Verified {
		return nil
	}

	// In the Committer trust model a signature is trusted if it matches the committer
	// - it doesn't matter if they're a collaborator, the owner, Gitea or Github
	// NB: This model is commit verification only
	if repoTrustModel == repo_model.CommitterTrustModel {
		// default to "unmatched"
		verification.TrustStatus = "unmatched"

		// We can only verify against users in our database but the default key will match
		// against by email if it is not in the db.
		if (verification.SigningUser.ID != 0 &&
			verification.CommittingUser.ID == verification.SigningUser.ID) ||
			(verification.SigningUser.ID == 0 && verification.CommittingUser.ID == 0 &&
				verification.SigningUser.Email == verification.CommittingUser.Email) {
			verification.TrustStatus = "trusted"
		}
		return nil
	}

	// Now we drop to the more nuanced trust models...
	verification.TrustStatus = "trusted"

	if verification.SigningUser.ID == 0 {
		// This commit is signed by the default key - but this key is not assigned to a user in the DB.

		// However in the repo_model.CollaboratorCommitterTrustModel we cannot mark this as trusted
		// unless the default key matches the email of a non-user.
		if repoTrustModel == repo_model.CollaboratorCommitterTrustModel && (verification.CommittingUser.ID != 0 ||
			verification.SigningUser.Email != verification.CommittingUser.Email) {
			verification.TrustStatus = "untrusted"
		}
		return nil
	}

	// Check we actually have a GPG SigningKey
	var err error
	if verification.SigningKey != nil {
		var isMember bool
		if keyMap != nil {
			var has bool
			isMember, has = (*keyMap)[verification.SigningKey.KeyID]
			if !has {
				isMember, err = isOwnerMemberCollaborator(verification.SigningUser)
				(*keyMap)[verification.SigningKey.KeyID] = isMember
			}
		} else {
			isMember, err = isOwnerMemberCollaborator(verification.SigningUser)
		}

		if !isMember {
			verification.TrustStatus = "untrusted"
			if verification.CommittingUser.ID != verification.SigningUser.ID {
				// The committing user and the signing user are not the same
				// This should be marked as questionable unless the signing user is a collaborator/team member etc.
				verification.TrustStatus = "unmatched"
			}
		} else if repoTrustModel == repo_model.CollaboratorCommitterTrustModel && verification.CommittingUser.ID != verification.SigningUser.ID {
			// The committing user and the signing user are not the same and our trustmodel states that they must match
			verification.TrustStatus = "unmatched"
		}
	}

	return err
}
