// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package repo

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"code.gitea.io/gitea/models/db"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/secret"
	"code.gitea.io/gitea/modules/setting"
	"code.gitea.io/gitea/modules/util"

	"golang.org/x/crypto/ssh"
)

// UserSSHKeypair represents an SSH keypair for repository mirroring
type UserSSHKeypair struct {
	OwnerID             int64
	PrivateKeyEncrypted string
	PublicKey           string
	Fingerprint         string
}

// GetUserSSHKeypairByOwner gets the SSH keypair for the given owner
func GetUserSSHKeypairByOwner(ctx context.Context, ownerID int64) (*UserSSHKeypair, error) {
	settings, err := user_model.GetSettings(ctx, ownerID, []string{
		user_model.UserSSHMirrorPrivPem,
		user_model.UserSSHMirrorPubPem,
		user_model.UserSSHMirrorFingerprint,
	})
	if err != nil {
		return nil, err
	}
	if len(settings) == 0 {
		return nil, util.NewNotExistErrorf("SSH keypair does not exist for owner %d", ownerID)
	}

	keypair := &UserSSHKeypair{
		OwnerID: ownerID,
	}

	if privSetting, exists := settings[user_model.UserSSHMirrorPrivPem]; exists {
		keypair.PrivateKeyEncrypted = privSetting.SettingValue
	}
	if pubSetting, exists := settings[user_model.UserSSHMirrorPubPem]; exists {
		keypair.PublicKey = pubSetting.SettingValue
	}
	if fpSetting, exists := settings[user_model.UserSSHMirrorFingerprint]; exists {
		keypair.Fingerprint = fpSetting.SettingValue
	}

	if keypair.PrivateKeyEncrypted == "" || keypair.PublicKey == "" || keypair.Fingerprint == "" {
		return nil, util.NewNotExistErrorf("SSH keypair incomplete for owner %d", ownerID)
	}

	return keypair, nil
}

// CreateUserSSHKeypair creates a new SSH keypair for mirroring
func CreateUserSSHKeypair(ctx context.Context, ownerID int64) (*UserSSHKeypair, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Ed25519 keypair: %w", err)
	}

	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to convert public key to SSH format: %w", err)
	}

	publicKeyStr := string(ssh.MarshalAuthorizedKey(sshPublicKey))

	fingerprint := sha256.Sum256(sshPublicKey.Marshal())
	fingerprintStr := hex.EncodeToString(fingerprint[:])

	privateKeyEncrypted, err := secret.EncryptSecret(setting.SecretKey, string(privateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt private key: %w", err)
	}

	err = db.WithTx(ctx, func(ctx context.Context) error {
		if err := user_model.SetUserSetting(ctx, ownerID, user_model.UserSSHMirrorPrivPem, privateKeyEncrypted); err != nil {
			return fmt.Errorf("failed to save private key: %w", err)
		}
		if err := user_model.SetUserSetting(ctx, ownerID, user_model.UserSSHMirrorPubPem, publicKeyStr); err != nil {
			return fmt.Errorf("failed to save public key: %w", err)
		}
		if err := user_model.SetUserSetting(ctx, ownerID, user_model.UserSSHMirrorFingerprint, fingerprintStr); err != nil {
			return fmt.Errorf("failed to save fingerprint: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	keypair := &UserSSHKeypair{
		OwnerID:             ownerID,
		PrivateKeyEncrypted: privateKeyEncrypted,
		PublicKey:           publicKeyStr,
		Fingerprint:         fingerprintStr,
	}

	return keypair, nil
}

// GetDecryptedPrivateKey returns the decrypted private key
func (k *UserSSHKeypair) GetDecryptedPrivateKey() (ed25519.PrivateKey, error) {
	decrypted, err := secret.DecryptSecret(setting.SecretKey, k.PrivateKeyEncrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt private key: %w", err)
	}
	return ed25519.PrivateKey(decrypted), nil
}

// GetPublicKeyWithComment returns the public key with a descriptive comment (namespace-fingerprint@domain)
func (k *UserSSHKeypair) GetPublicKeyWithComment(ctx context.Context) (string, error) {
	owner, err := user_model.GetUserByID(ctx, k.OwnerID)
	if err != nil {
		return k.PublicKey, nil
	}

	domain := setting.Domain
	if domain == "" {
		domain = "gitea"
	}

	keyID := k.Fingerprint
	if len(keyID) > 8 {
		keyID = keyID[:8]
	}

	comment := fmt.Sprintf("%s-%s@%s", owner.Name, keyID, domain)
	return strings.TrimSpace(k.PublicKey) + " " + comment, nil
}

// DeleteUserSSHKeypair deletes an SSH keypair
func DeleteUserSSHKeypair(ctx context.Context, ownerID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := user_model.DeleteUserSetting(ctx, ownerID, user_model.UserSSHMirrorPrivPem); err != nil {
			return err
		}
		if err := user_model.DeleteUserSetting(ctx, ownerID, user_model.UserSSHMirrorPubPem); err != nil {
			return err
		}
		return user_model.DeleteUserSetting(ctx, ownerID, user_model.UserSSHMirrorFingerprint)
	})
}

// RegenerateUserSSHKeypair regenerates an SSH keypair for the given owner
func RegenerateUserSSHKeypair(ctx context.Context, ownerID int64) (*UserSSHKeypair, error) {
	return db.WithTx2(ctx, func(ctx context.Context) (*UserSSHKeypair, error) {
		_ = DeleteUserSSHKeypair(ctx, ownerID)

		newKeypair, err := CreateUserSSHKeypair(ctx, ownerID)
		if err != nil {
			return nil, err
		}
		return newKeypair, nil
	})
}
