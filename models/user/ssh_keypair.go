// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"

	"gitea.dev/models/db"
	"gitea.dev/modules/log"
	"gitea.dev/modules/secret"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"golang.org/x/crypto/ssh"
)

// SSHKeypair is a Gitea-managed SSH keypair that authenticates the owner's SSH mirrors and migrations
type SSHKeypair struct {
	OwnerID             int64
	PrivateKeyEncrypted string
	PublicKey           string
	Fingerprint         string
}

// GetSSHKeypairByOwner returns the owner's managed SSH keypair, a NotExist error if it was never generated
func GetSSHKeypairByOwner(ctx context.Context, ownerID int64) (*SSHKeypair, error) {
	settings, err := GetSettings(ctx, ownerID, []string{UserSSHMirrorPrivPem, UserSSHMirrorPubPem})
	if err != nil {
		return nil, err
	}
	privSetting, pubSetting := settings[UserSSHMirrorPrivPem], settings[UserSSHMirrorPubPem]
	if privSetting == nil || pubSetting == nil || privSetting.SettingValue == "" || pubSetting.SettingValue == "" {
		return nil, util.NewNotExistErrorf("SSH keypair does not exist for owner %d", ownerID)
	}

	// the fingerprint is derived rather than stored, so its format can't drift
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(pubSetting.SettingValue))
	if err != nil {
		return nil, fmt.Errorf("failed to parse stored public key: %w", err)
	}
	return &SSHKeypair{
		OwnerID:             ownerID,
		PrivateKeyEncrypted: privSetting.SettingValue,
		PublicKey:           pubSetting.SettingValue,
		Fingerprint:         ssh.FingerprintSHA256(publicKey),
	}, nil
}

// GetOrCreateSSHKeypair returns the owner's managed SSH keypair, generating it on first use
func GetOrCreateSSHKeypair(ctx context.Context, ownerID int64) (*SSHKeypair, error) {
	keypair, err := GetSSHKeypairByOwner(ctx, ownerID)
	if err != nil {
		if errors.Is(err, util.ErrNotExist) {
			log.Debug("Creating new SSH keypair for owner %d", ownerID)
			return CreateSSHKeypair(ctx, ownerID)
		}
		return nil, fmt.Errorf("failed to get SSH keypair for owner %d: %w", ownerID, err)
	}
	return keypair, nil
}

// CreateSSHKeypair generates a new managed SSH keypair for the owner, replacing the existing one
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
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &SSHKeypair{
		OwnerID:             ownerID,
		PrivateKeyEncrypted: privateKeyEncrypted,
		PublicKey:           publicKeyStr,
		Fingerprint:         ssh.FingerprintSHA256(sshPublicKey),
	}, nil
}

// GetDecryptedPrivateKey returns the decrypted private key
func (k *SSHKeypair) GetDecryptedPrivateKey() (ed25519.PrivateKey, error) {
	decrypted, err := secret.DecryptSecret(setting.SecretKey, k.PrivateKeyEncrypted)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt private key: %w", err)
	}
	return ed25519.PrivateKey(decrypted), nil
}

// PublicKeyWithComment returns the public key commented as "owner-fingerprint@domain", so it is recognizable on the remote
func (k *SSHKeypair) PublicKeyWithComment(ownerName string) string {
	domain := setting.AppDomain
	if domain == "" {
		domain = "gitea"
	}

	keyID := strings.TrimPrefix(k.Fingerprint, "SHA256:")
	if len(keyID) > 8 {
		keyID = keyID[:8]
	}

	return fmt.Sprintf("%s %s-%s@%s", strings.TrimSpace(k.PublicKey), ownerName, keyID, domain)
}
