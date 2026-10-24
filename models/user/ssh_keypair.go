// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package user

import (
	"context"
	"encoding/pem"
	"strings"

	"gitea.dev/modules/generate"
	"gitea.dev/modules/secret"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"

	"golang.org/x/crypto/ssh"
)

// SSHKeypair is a Gitea-managed SSH keypair that authenticates SSH mirrors and migrations
type SSHKeypair struct {
	PublicKey           string `json:"public_key"`
	PrivateKeyEncrypted string `json:"private_key_encrypted"`
	Fingerprint         string `json:"-"`
}

// GetSSHKeypairByOwner returns the owner's managed SSH keypair, a NotExist error if it was never generated
func GetSSHKeypairByOwner(ctx context.Context, ownerID int64) (*SSHKeypair, error) {
	keypair, err := GetUserSettingJSON(ctx, ownerID, SettingsKeyManagedSSHKey, &SSHKeypair{})
	if err != nil {
		return nil, err
	} else if keypair.PublicKey == "" {
		return nil, util.NewNotExistErrorf("managed SSH key of %d does not exist", ownerID)
	}
	publicKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(keypair.PublicKey))
	if err != nil {
		return nil, err
	}
	keypair.Fingerprint = ssh.FingerprintSHA256(publicKey)
	return keypair, nil
}

// CreateSSHKeypair generates the owner's managed SSH keypair, replacing the existing one
func CreateSSHKeypair(ctx context.Context, ownerID int64) (*SSHKeypair, error) {
	publicKey, privateKeyPEM, err := generate.NewSSHKey(generate.SSHKeyED25519, 0)
	if err != nil {
		return nil, err
	}
	keypair := &SSHKeypair{
		PublicKey:   strings.TrimSpace(string(ssh.MarshalAuthorizedKey(publicKey))),
		Fingerprint: ssh.FingerprintSHA256(publicKey),
	}
	if keypair.PrivateKeyEncrypted, err = secret.EncryptSecret(setting.SecretKey, string(pem.EncodeToMemory(privateKeyPEM))); err != nil {
		return nil, err
	}
	return keypair, SetUserSettingJSON(ctx, ownerID, SettingsKeyManagedSSHKey, keypair)
}

// PrivateKey returns the private key in OpenSSH format
func (k *SSHKeypair) PrivateKey() ([]byte, error) {
	privateKey, err := secret.DecryptSecret(setting.SecretKey, k.PrivateKeyEncrypted)
	return []byte(privateKey), err
}
