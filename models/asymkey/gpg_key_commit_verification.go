// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"fmt"
	"hash"
	"strings"

	"code.gitea.io/gitea/models/db"
	repo_model "code.gitea.io/gitea/models/repo"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

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

// ParseCommitsWithSignature checks if signaute of commits are corresponding to users gpg keys.
func ParseCommitsWithSignature(ctx context.Context, oldCommits []*user_model.UserCommit, repoTrustModel repo_model.TrustModelType, isOwnerMemberCollaborator func(*user_model.User) (bool, error)) []*SignCommit {
	newCommits := make([]*SignCommit, 0, len(oldCommits))
	keyMap := map[string]bool{}

	for _, c := range oldCommits {
		signCommit := &SignCommit{
			UserCommit:   c,
			Verification: ParseCommitWithSignature(ctx, c.Commit),
		}

		_ = CalculateTrustStatus(signCommit.Verification, repoTrustModel, isOwnerMemberCollaborator, &keyMap)

		newCommits = append(newCommits, signCommit)
	}
	return newCommits
}

// ParseCommitWithSignature check if signature is good against keystore.
func ParseCommitWithSignature(ctx context.Context, c *git.Commit) *CommitVerification {
	var committer *user_model.User
	if c.Committer != nil {
		var err error
		// Find Committer account
		committer, err = user_model.GetUserByEmail(ctx, c.Committer.Email) // This finds the user by primary email or activated email so commit will not be valid if email is not
		if err != nil {                                                    // Skipping not user for committer
			committer = &user_model.User{
				Name:  c.Committer.Name,
				Email: c.Committer.Email,
			}
			// We can expect this to often be an ErrUserNotExist. in the case
			// it is not, however, it is important to log it.
			if !user_model.IsErrUserNotExist(err) {
				log.Error("GetUserByEmail: %v", err)
				return &CommitVerification{
					CommittingUser: committer,
					Verified:       false,
					Reason:         "gpg.error.no_committer_account",
				}
			}
		}
	}

	// If no signature just report the committer
	if c.Signature == nil {
		return &CommitVerification{
			CommittingUser: committer,
			Verified:       false,                         // Default value
			Reason:         "gpg.error.not_signed_commit", // Default value
		}
	}

	// If this a SSH signature handle it differently
	if strings.HasPrefix(c.Signature.Signature, "-----BEGIN SSH SIGNATURE-----") {
		return ParseCommitWithSSHSignature(ctx, c, committer)
	}

	// Parsing signature
	sig, err := extractSignature(c.Signature.Signature)
	if err != nil { // Skipping failed to extract sign
		log.Error("SignatureRead err: %v", err)
		return &CommitVerification{
			CommittingUser: committer,
			Verified:       false,
			Reason:         "gpg.error.extract_sign",
		}
	}

	keyID := tryGetKeyIDFromSignature(sig)
	defaultReason := NoKeyFound

	// First check if the sig has a keyID and if so just look at that
	if commitVerification := hashAndVerifyForKeyID(
		ctx,
		sig,
		c.Signature.Payload,
		committer,
		keyID,
		setting.AppName,
		""); commitVerification != nil {
		if commitVerification.Reason == BadSignature {
			defaultReason = BadSignature
		} else {
			return commitVerification
		}
	}

	// Now try to associate the signature with the committer, if present
	if committer.ID != 0 {
		keys, err := db.Find[GPGKey](ctx, FindGPGKeyOptions{
			OwnerID: committer.ID,
		})
		if err != nil { // Skipping failed to get gpg keys of user
			log.Error("ListGPGKeys: %v", err)
			return &CommitVerification{
				CommittingUser: committer,
				Verified:       false,
				Reason:         "gpg.error.failed_retrieval_gpg_keys",
			}
		}

		if err := GPGKeyList(keys).LoadSubKeys(ctx); err != nil {
			log.Error("LoadSubKeys: %v", err)
			return &CommitVerification{
				CommittingUser: committer,
				Verified:       false,
				Reason:         "gpg.error.failed_retrieval_gpg_keys",
			}
		}

		committerEmailAddresses, _ := user_model.GetEmailAddresses(ctx, committer.ID)
		activated := false
		for _, e := range committerEmailAddresses {
			if e.IsActivated && strings.EqualFold(e.Email, c.Committer.Email) {
				activated = true
				break
			}
		}

		for _, k := range keys {
			// Pre-check (& optimization) that emails attached to key can be attached to the committer email and can validate
			canValidate := false
			email := ""
			if k.Verified && activated {
				canValidate = true
				email = c.Committer.Email
			}
			if !canValidate {
				for _, e := range k.Emails {
					if e.IsActivated && strings.EqualFold(e.Email, c.Committer.Email) {
						canValidate = true
						email = e.Email
						break
					}
				}
			}
			if !canValidate {
				continue // Skip this key
			}

			commitVerification := hashAndVerifyWithSubKeysCommitVerification(sig, c.Signature.Payload, k, committer, committer, email)
			if commitVerification != nil {
				return commitVerification
			}
		}
	}

	if setting.Repository.Signing.SigningKey != "" && setting.Repository.Signing.SigningKey != "default" && setting.Repository.Signing.SigningKey != "none" {
		// OK we should try the default key
		gpgSettings := git.GPGSettings{
			Sign:  true,
			KeyID: setting.Repository.Signing.SigningKey,
			Name:  setting.Repository.Signing.SigningName,
			Email: setting.Repository.Signing.SigningEmail,
		}
		if err := gpgSettings.LoadPublicKeyContent(); err != nil {
			log.Error("Error getting default signing key: %s %v", gpgSettings.KeyID, err)
		} else if commitVerification := verifyWithGPGSettings(ctx, &gpgSettings, sig, c.Signature.Payload, committer, keyID); commitVerification != nil {
			if commitVerification.Reason == BadSignature {
				defaultReason = BadSignature
			} else {
				return commitVerification
			}
		}
	}

	defaultGPGSettings, err := c.GetRepositoryDefaultPublicGPGKey(false)
	if err != nil {
		log.Error("Error getting default public gpg key: %v", err)
	} else if defaultGPGSettings == nil {
		log.Warn("Unable to get defaultGPGSettings for unattached commit: %s", c.ID.String())
	} else if defaultGPGSettings.Sign {
		if commitVerification := verifyWithGPGSettings(ctx, defaultGPGSettings, sig, c.Signature.Payload, committer, keyID); commitVerification != nil {
			if commitVerification.Reason == BadSignature {
				defaultReason = BadSignature
			} else {
				return commitVerification
			}
		}
	}

	return &CommitVerification{ // Default at this stage
		CommittingUser: committer,
		Verified:       false,
		Warning:        defaultReason != NoKeyFound,
		Reason:         defaultReason,
		SigningKey: &GPGKey{
			KeyID: keyID,
		},
	}
}

func verifyWithGPGSettings(ctx context.Context, gpgSettings *git.GPGSettings, sig *packet.Signature, payload string, committer *user_model.User, keyID string) *CommitVerification {
	// First try to find the key in the db
	if commitVerification := hashAndVerifyForKeyID(ctx, sig, payload, committer, gpgSettings.KeyID, gpgSettings.Name, gpgSettings.Email); commitVerification != nil {
		return commitVerification
	}

	// Otherwise we have to parse the key
	ekeys, err := checkArmoredGPGKeyString(gpgSettings.PublicKeyContent)
	if err != nil {
		log.Error("Unable to get default signing key: %v", err)
		return &CommitVerification{
			CommittingUser: committer,
			Verified:       false,
			Reason:         "gpg.error.generate_hash",
		}
	}
	for _, ekey := range ekeys {
		pubkey := ekey.PrimaryKey
		content, err := base64EncPubKey(pubkey)
		if err != nil {
			return &CommitVerification{
				CommittingUser: committer,
				Verified:       false,
				Reason:         "gpg.error.generate_hash",
			}
		}
		k := &GPGKey{
			Content: content,
			CanSign: pubkey.CanSign(),
			KeyID:   pubkey.KeyIdString(),
		}
		for _, subKey := range ekey.Subkeys {
			content, err := base64EncPubKey(subKey.PublicKey)
			if err != nil {
				return &CommitVerification{
					CommittingUser: committer,
					Verified:       false,
					Reason:         "gpg.error.generate_hash",
				}
			}
			k.SubsKey = append(k.SubsKey, &GPGKey{
				Content: content,
				CanSign: subKey.PublicKey.CanSign(),
				KeyID:   subKey.PublicKey.KeyIdString(),
			})
		}
		if commitVerification := hashAndVerifyWithSubKeysCommitVerification(sig, payload, k, committer, &user_model.User{
			Name:  gpgSettings.Name,
			Email: gpgSettings.Email,
		}, gpgSettings.Email); commitVerification != nil {
			return commitVerification
		}
		if keyID == k.KeyID {
			// This is a bad situation ... We have a key id that matches our default key but the signature doesn't match.
			return &CommitVerification{
				CommittingUser: committer,
				Verified:       false,
				Warning:        true,
				Reason:         BadSignature,
			}
		}
	}
	return nil
}

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

func hashAndVerifyWithSubKeysCommitVerification(sig *packet.Signature, payload string, k *GPGKey, committer, signer *user_model.User, email string) *CommitVerification {
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

func hashAndVerifyForKeyID(ctx context.Context, sig *packet.Signature, payload string, committer *user_model.User, keyID, name, email string) *CommitVerification {
	if keyID == "" {
		return nil
	}
	keys, err := db.Find[GPGKey](ctx, FindGPGKeyOptions{
		KeyID:          keyID,
		IncludeSubKeys: true,
	})
	if err != nil {
		log.Error("GetGPGKeysByKeyID: %v", err)
		return &CommitVerification{
			CommittingUser: committer,
			Verified:       false,
			Reason:         "gpg.error.failed_retrieval_gpg_keys",
		}
	}
	if len(keys) == 0 {
		return nil
	}
	for _, key := range keys {
		var primaryKeys []*GPGKey
		if key.PrimaryKeyID != "" {
			primaryKeys, err = db.Find[GPGKey](ctx, FindGPGKeyOptions{
				KeyID:          key.PrimaryKeyID,
				IncludeSubKeys: true,
			})
			if err != nil {
				log.Error("GetGPGKeysByKeyID: %v", err)
				return &CommitVerification{
					CommittingUser: committer,
					Verified:       false,
					Reason:         "gpg.error.failed_retrieval_gpg_keys",
				}
			}
		}

		activated, email := checkKeyEmails(ctx, email, append([]*GPGKey{key}, primaryKeys...)...)
		if !activated {
			continue
		}

		signer := &user_model.User{
			Name:  name,
			Email: email,
		}
		if key.OwnerID != 0 {
			owner, err := user_model.GetUserByID(ctx, key.OwnerID)
			if err == nil {
				signer = owner
			} else if !user_model.IsErrUserNotExist(err) {
				log.Error("Failed to user_model.GetUserByID: %d for key ID: %d (%s) %v", key.OwnerID, key.ID, key.KeyID, err)
				return &CommitVerification{
					CommittingUser: committer,
					Verified:       false,
					Reason:         "gpg.error.no_committer_account",
				}
			}
		}
		commitVerification := hashAndVerifyWithSubKeysCommitVerification(sig, payload, key, committer, signer, email)
		if commitVerification != nil {
			return commitVerification
		}
	}
	// This is a bad situation ... We have a key id that is in our database but the signature doesn't match.
	return &CommitVerification{
		CommittingUser: committer,
		Verified:       false,
		Warning:        true,
		Reason:         BadSignature,
	}
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
