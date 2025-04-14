// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"fmt"
	"strings"

	asymkey_model "code.gitea.io/gitea/models/asymkey"
	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/cache"
	"code.gitea.io/gitea/modules/cachegroup"
	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/setting"

	"github.com/42wim/sshsig"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
)

// ParseCommitWithSignature check if signature is good against keystore.
func ParseCommitWithSignature(ctx context.Context, c *git.Commit) *asymkey_model.CommitVerification {
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
				return &asymkey_model.CommitVerification{
					CommittingUser: committer,
					Verified:       false,
					Reason:         "gpg.error.no_committer_account",
				}
			}
		}
	}

	return ParseCommitWithSignatureCommitter(ctx, c, committer)
}

func ParseCommitWithSignatureCommitter(ctx context.Context, c *git.Commit, committer *user_model.User) *asymkey_model.CommitVerification {
	// If no signature just report the committer
	if c.Signature == nil {
		return &asymkey_model.CommitVerification{
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
	sig, err := asymkey_model.ExtractSignature(c.Signature.Signature)
	if err != nil { // Skipping failed to extract sign
		log.Error("SignatureRead err: %v", err)
		return &asymkey_model.CommitVerification{
			CommittingUser: committer,
			Verified:       false,
			Reason:         "gpg.error.extract_sign",
		}
	}

	keyID := asymkey_model.TryGetKeyIDFromSignature(sig)
	defaultReason := asymkey_model.NoKeyFound

	// First check if the sig has a keyID and if so just look at that
	if commitVerification := HashAndVerifyForKeyID(
		ctx,
		sig,
		c.Signature.Payload,
		committer,
		keyID,
		setting.AppName,
		""); commitVerification != nil {
		if commitVerification.Reason == asymkey_model.BadSignature {
			defaultReason = asymkey_model.BadSignature
		} else {
			return commitVerification
		}
	}

	// Now try to associate the signature with the committer, if present
	if committer.ID != 0 {
		keys, err := db.Find[asymkey_model.GPGKey](ctx, asymkey_model.FindGPGKeyOptions{
			OwnerID: committer.ID,
		})
		if err != nil { // Skipping failed to get gpg keys of user
			log.Error("ListGPGKeys: %v", err)
			return &asymkey_model.CommitVerification{
				CommittingUser: committer,
				Verified:       false,
				Reason:         "gpg.error.failed_retrieval_gpg_keys",
			}
		}

		if err := asymkey_model.GPGKeyList(keys).LoadSubKeys(ctx); err != nil {
			log.Error("LoadSubKeys: %v", err)
			return &asymkey_model.CommitVerification{
				CommittingUser: committer,
				Verified:       false,
				Reason:         "gpg.error.failed_retrieval_gpg_keys",
			}
		}

		committerEmailAddresses, _ := cache.GetWithContextCache(ctx, cachegroup.UserEmailAddresses, committer.ID, user_model.GetEmailAddresses)
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

			commitVerification := asymkey_model.HashAndVerifyWithSubKeysCommitVerification(sig, c.Signature.Payload, k, committer, committer, email)
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
		} else if commitVerification := VerifyWithGPGSettings(ctx, &gpgSettings, sig, c.Signature.Payload, committer, keyID); commitVerification != nil {
			if commitVerification.Reason == asymkey_model.BadSignature {
				defaultReason = asymkey_model.BadSignature
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
		if commitVerification := VerifyWithGPGSettings(ctx, defaultGPGSettings, sig, c.Signature.Payload, committer, keyID); commitVerification != nil {
			if commitVerification.Reason == asymkey_model.BadSignature {
				defaultReason = asymkey_model.BadSignature
			} else {
				return commitVerification
			}
		}
	}

	return &asymkey_model.CommitVerification{ // Default at this stage
		CommittingUser: committer,
		Verified:       false,
		Warning:        defaultReason != asymkey_model.NoKeyFound,
		Reason:         defaultReason,
		SigningKey: &asymkey_model.GPGKey{
			KeyID: keyID,
		},
	}
}

func checkKeyEmails(ctx context.Context, email string, keys ...*asymkey_model.GPGKey) (bool, string) {
	uid := int64(0)
	var userEmails []*user_model.EmailAddress
	var user *user_model.User
	for _, key := range keys {
		for _, e := range key.Emails {
			if e.IsActivated && (email == "" || strings.EqualFold(e.Email, email)) {
				return true, e.Email
			}
		}
		if key.Verified && key.OwnerID != 0 {
			if uid != key.OwnerID {
				userEmails, _ = cache.GetWithContextCache(ctx, cachegroup.UserEmailAddresses, key.OwnerID, user_model.GetEmailAddresses)
				uid = key.OwnerID
				user, _ = cache.GetWithContextCache(ctx, cachegroup.User, uid, user_model.GetUserByID)
			}
			for _, e := range userEmails {
				if e.IsActivated && (email == "" || strings.EqualFold(e.Email, email)) {
					return true, e.Email
				}
			}
			if user.KeepEmailPrivate && strings.EqualFold(email, user.GetEmail()) {
				return true, user.GetEmail()
			}
		}
	}
	return false, email
}

func HashAndVerifyForKeyID(ctx context.Context, sig *packet.Signature, payload string, committer *user_model.User, keyID, name, email string) *asymkey_model.CommitVerification {
	if keyID == "" {
		return nil
	}
	keys, err := cache.GetWithContextCache(ctx, cachegroup.GPGKeyWithSubKeys, keyID, asymkey_model.FindGPGKeyWithSubKeys)
	if err != nil {
		log.Error("GetGPGKeysByKeyID: %v", err)
		return &asymkey_model.CommitVerification{
			CommittingUser: committer,
			Verified:       false,
			Reason:         "gpg.error.failed_retrieval_gpg_keys",
		}
	}
	if len(keys) == 0 {
		return nil
	}
	for _, key := range keys {
		var primaryKeys []*asymkey_model.GPGKey
		if key.PrimaryKeyID != "" {
			primaryKeys, err = cache.GetWithContextCache(ctx, cachegroup.GPGKeyWithSubKeys, key.PrimaryKeyID, asymkey_model.FindGPGKeyWithSubKeys)
			if err != nil {
				log.Error("GetGPGKeysByKeyID: %v", err)
				return &asymkey_model.CommitVerification{
					CommittingUser: committer,
					Verified:       false,
					Reason:         "gpg.error.failed_retrieval_gpg_keys",
				}
			}
		}

		activated, email := checkKeyEmails(ctx, email, append([]*asymkey_model.GPGKey{key}, primaryKeys...)...)
		if !activated {
			continue
		}

		signer := &user_model.User{
			Name:  name,
			Email: email,
		}
		if key.OwnerID > 0 {
			owner, err := cache.GetWithContextCache(ctx, cachegroup.User, key.OwnerID, user_model.GetUserByID)
			if err == nil {
				signer = owner
			} else if !user_model.IsErrUserNotExist(err) {
				log.Error("Failed to user_model.GetUserByID: %d for key ID: %d (%s) %v", key.OwnerID, key.ID, key.KeyID, err)
				return &asymkey_model.CommitVerification{
					CommittingUser: committer,
					Verified:       false,
					Reason:         "gpg.error.no_committer_account",
				}
			}
		}
		commitVerification := asymkey_model.HashAndVerifyWithSubKeysCommitVerification(sig, payload, key, committer, signer, email)
		if commitVerification != nil {
			return commitVerification
		}
	}
	// This is a bad situation ... We have a key id that is in our database but the signature doesn't match.
	return &asymkey_model.CommitVerification{
		CommittingUser: committer,
		Verified:       false,
		Warning:        true,
		Reason:         asymkey_model.BadSignature,
	}
}

func VerifyWithGPGSettings(ctx context.Context, gpgSettings *git.GPGSettings, sig *packet.Signature, payload string, committer *user_model.User, keyID string) *asymkey_model.CommitVerification {
	// First try to find the key in the db
	if commitVerification := HashAndVerifyForKeyID(ctx, sig, payload, committer, gpgSettings.KeyID, gpgSettings.Name, gpgSettings.Email); commitVerification != nil {
		return commitVerification
	}

	// Otherwise we have to parse the key
	ekeys, err := asymkey_model.CheckArmoredGPGKeyString(gpgSettings.PublicKeyContent)
	if err != nil {
		log.Error("Unable to get default signing key: %v", err)
		return &asymkey_model.CommitVerification{
			CommittingUser: committer,
			Verified:       false,
			Reason:         "gpg.error.generate_hash",
		}
	}
	for _, ekey := range ekeys {
		pubkey := ekey.PrimaryKey
		content, err := asymkey_model.Base64EncPubKey(pubkey)
		if err != nil {
			return &asymkey_model.CommitVerification{
				CommittingUser: committer,
				Verified:       false,
				Reason:         "gpg.error.generate_hash",
			}
		}
		k := &asymkey_model.GPGKey{
			Content: content,
			CanSign: pubkey.CanSign(),
			KeyID:   pubkey.KeyIdString(),
		}
		for _, subKey := range ekey.Subkeys {
			content, err := asymkey_model.Base64EncPubKey(subKey.PublicKey)
			if err != nil {
				return &asymkey_model.CommitVerification{
					CommittingUser: committer,
					Verified:       false,
					Reason:         "gpg.error.generate_hash",
				}
			}
			k.SubsKey = append(k.SubsKey, &asymkey_model.GPGKey{
				Content: content,
				CanSign: subKey.PublicKey.CanSign(),
				KeyID:   subKey.PublicKey.KeyIdString(),
			})
		}
		if commitVerification := asymkey_model.HashAndVerifyWithSubKeysCommitVerification(sig, payload, k, committer, &user_model.User{
			Name:  gpgSettings.Name,
			Email: gpgSettings.Email,
		}, gpgSettings.Email); commitVerification != nil {
			return commitVerification
		}
		if keyID == k.KeyID {
			// This is a bad situation ... We have a key id that matches our default key but the signature doesn't match.
			return &asymkey_model.CommitVerification{
				CommittingUser: committer,
				Verified:       false,
				Warning:        true,
				Reason:         asymkey_model.BadSignature,
			}
		}
	}
	return nil
}

// ParseCommitWithSSHSignature check if signature is good against keystore.
func ParseCommitWithSSHSignature(ctx context.Context, c *git.Commit, committer *user_model.User) *asymkey_model.CommitVerification {
	// Now try to associate the signature with the committer, if present
	if committer.ID != 0 {
		keys, err := db.Find[asymkey_model.PublicKey](ctx, asymkey_model.FindPublicKeyOptions{
			OwnerID:    committer.ID,
			NotKeytype: asymkey_model.KeyTypePrincipal,
		})
		if err != nil { // Skipping failed to get ssh keys of user
			log.Error("ListPublicKeys: %v", err)
			return &asymkey_model.CommitVerification{
				CommittingUser: committer,
				Verified:       false,
				Reason:         "gpg.error.failed_retrieval_gpg_keys",
			}
		}

		committerEmailAddresses, err := cache.GetWithContextCache(ctx, cachegroup.UserEmailAddresses, committer.ID, user_model.GetEmailAddresses)
		if err != nil {
			log.Error("GetEmailAddresses: %v", err)
		}

		activated := false
		for _, e := range committerEmailAddresses {
			if e.IsActivated && strings.EqualFold(e.Email, c.Committer.Email) {
				activated = true
				break
			}
		}

		for _, k := range keys {
			if k.Verified && activated {
				commitVerification := verifySSHCommitVerification(c.Signature.Signature, c.Signature.Payload, k, committer, committer, c.Committer.Email)
				if commitVerification != nil {
					return commitVerification
				}
			}
		}
	}

	return &asymkey_model.CommitVerification{
		CommittingUser: committer,
		Verified:       false,
		Reason:         asymkey_model.NoKeyFound,
	}
}

func verifySSHCommitVerification(sig, payload string, k *asymkey_model.PublicKey, committer, signer *user_model.User, email string) *asymkey_model.CommitVerification {
	if err := sshsig.Verify(strings.NewReader(payload), []byte(sig), []byte(k.Content), "git"); err != nil {
		return nil
	}

	return &asymkey_model.CommitVerification{ // Everything is ok
		CommittingUser: committer,
		Verified:       true,
		Reason:         fmt.Sprintf("%s / %s", signer.Name, k.Fingerprint),
		SigningUser:    signer,
		SigningSSHKey:  k,
		SigningEmail:   email,
	}
}
