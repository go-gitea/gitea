// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/modules/secret"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"golang.org/x/crypto/ssh"
)

// SSHKeypair represents an SSH keypair for repository mirroring
type SSHKeypair struct {
	OwnerID             int64
	PrivateKeyEncrypted string
	PublicKey           string
	Fingerprint         string
}

// GetSSHKeypairByOwner gets the SSH keypair for the given owner
func GetSSHKeypairByOwner(ctx context.Context, ownerID int64) (*SSHKeypair, error) {
	settings, err := GetSettings(ctx, ownerID, []string{
		UserSSHMirrorPrivPem,
		UserSSHMirrorPubPem,
		UserSSHMirrorFingerprint,
	})
	if err != nil {
		return nil, err
	}
	if len(settings) == 0 {
		return nil, util.NewNotExistErrorf("SSH keypair does not exist for owner %d", ownerID)
	}

	keypair := &SSHKeypair{
		OwnerID: ownerID,
	}

	if privSetting, exists := settings[UserSSHMirrorPrivPem]; exists {
		keypair.PrivateKeyEncrypted = privSetting.SettingValue
	}
	if pubSetting, exists := settings[UserSSHMirrorPubPem]; exists {
		keypair.PublicKey = pubSetting.SettingValue
	}
	if fpSetting, exists := settings[UserSSHMirrorFingerprint]; exists {
		keypair.Fingerprint = fpSetting.SettingValue
	}

	if keypair.PrivateKeyEncrypted == "" || keypair.PublicKey == "" || keypair.Fingerprint == "" {
		return nil, util.NewNotExistErrorf("SSH keypair incomplete for owner %d", ownerID)
	}

	return keypair, nil
}

// CreateSSHKeypair creates a new SSH keypair for mirroring
func CreateSSHKeypair(ctx context.Context, ownerID int64) (*SSHKeypair, error) {
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Ed25519 keypair: %w", err)
	}

	sshPublicKey, err := ssh.NewPublicKey(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to convert public key to SSH format: %w", err)
	}

	publicKeyStr := string(ssh.MarshalAuthorizedKey(sshPublicKey))

	fingerprintStr := ssh.FingerprintSHA256(sshPublicKey)

	privateKeyEncrypted, err := secret.EncryptSecret(setting.SecretKey, string(privateKey))
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt private key: %w", err)
	}

	err = db.WithTx(ctx, func(ctx context.Context) error {
		if err := SetUserSetting(ctx, ownerID, UserSSHMirrorPrivPem, privateKeyEncrypted); err != nil {
			return fmt.Errorf("failed to save private key: %w", err)
		}
		if err := SetUserSetting(ctx, ownerID, UserSSHMirrorPubPem, publicKeyStr); err != nil {
			return fmt.Errorf("failed to save public key: %w", err)
		}
		if err := SetUserSetting(ctx, ownerID, UserSSHMirrorFingerprint, fingerprintStr); err != nil {
			return fmt.Errorf("failed to save fingerprint: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	keypair := &SSHKeypair{
		OwnerID:             ownerID,
		PrivateKeyEncrypted: privateKeyEncrypted,
		PublicKey:           publicKeyStr,
		Fingerprint:         fingerprintStr,
	}

	return keypair, nil
}

// GetDecryptedPrivateKey returns the decrypted private key
func (k *SSHKeypair) GetDecryptedPrivateKey() (ed25519.PrivateKey, error) {
	decrypted, err := secret.DecryptSecret(setting.SecretKey, k.PrivateKeyEncrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt private key: %w", err)
	}
	return ed25519.PrivateKey(decrypted), nil
}

// GetPublicKeyWithComment returns the public key with a descriptive comment (namespace-fingerprint@domain)
func (k *SSHKeypair) GetPublicKeyWithComment(ctx context.Context) (string, error) {
	owner, err := GetUserByID(ctx, k.OwnerID)
	if err != nil {
		return k.PublicKey, nil
	}

	domain := setting.Domain
	if domain == "" {
		domain = "gitea"
	}

	keyID := strings.TrimPrefix(k.Fingerprint, "SHA256:")
	if len(keyID) > 8 {
		keyID = keyID[:8]
	}

	comment := fmt.Sprintf("%s-%s@%s", owner.Name, keyID, domain)
	return strings.TrimSpace(k.PublicKey) + " " + comment, nil
}

// DeleteSSHKeypair deletes an SSH keypair
func DeleteSSHKeypair(ctx context.Context, ownerID int64) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		if err := DeleteUserSetting(ctx, ownerID, UserSSHMirrorPrivPem); err != nil {
			return err
		}
		if err := DeleteUserSetting(ctx, ownerID, UserSSHMirrorPubPem); err != nil {
			return err
		}
		return DeleteUserSetting(ctx, ownerID, UserSSHMirrorFingerprint)
	})
}

// RegenerateSSHKeypair regenerates an SSH keypair for the given owner
func RegenerateSSHKeypair(ctx context.Context, ownerID int64) (*SSHKeypair, error) {
	return db.WithTx2(ctx, func(ctx context.Context) (*SSHKeypair, error) {
		if err := DeleteSSHKeypair(ctx, ownerID); err != nil {
			return nil, fmt.Errorf("failed to delete existing keypair: %w", err)
		}

		newKeypair, err := CreateSSHKeypair(ctx, ownerID)
		if err != nil {
			return nil, err
		}
		return newKeypair, nil
	})
}
