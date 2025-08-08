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
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"golang.org/x/crypto/ssh"
)

// UserSSHKeypair represents an SSH keypair for repository mirroring
type UserSSHKeypair struct {
	ID                  int64              `xorm:"pk autoincr"`
	OwnerID             int64              `xorm:"INDEX NOT NULL"`
	PrivateKeyEncrypted string             `xorm:"TEXT NOT NULL"`
	PublicKey           string             `xorm:"TEXT NOT NULL"`
	Fingerprint         string             `xorm:"VARCHAR(255) UNIQUE NOT NULL"`
	CreatedUnix         timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix         timeutil.TimeStamp `xorm:"updated"`
}

func init() {
	db.RegisterModel(new(UserSSHKeypair))
}

// GetUserSSHKeypairByOwner gets the most recent SSH keypair for the given owner
func GetUserSSHKeypairByOwner(ctx context.Context, ownerID int64) (*UserSSHKeypair, error) {
	keypair := &UserSSHKeypair{}
	has, err := db.GetEngine(ctx).Where("owner_id = ?", ownerID).
		Desc("created_unix").Get(keypair)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, util.NewNotExistErrorf("SSH keypair does not exist for owner %d", ownerID)
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

	keypair := &UserSSHKeypair{
		OwnerID:             ownerID,
		PrivateKeyEncrypted: privateKeyEncrypted,
		PublicKey:           publicKeyStr,
		Fingerprint:         fingerprintStr,
	}

	return keypair, db.Insert(ctx, keypair)
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
	_, err := db.GetEngine(ctx).Where("owner_id = ?", ownerID).Delete(&UserSSHKeypair{})
	return err
}

// RegenerateUserSSHKeypair regenerates an SSH keypair for the given owner
func RegenerateUserSSHKeypair(ctx context.Context, ownerID int64) (*UserSSHKeypair, error) {
	// TODO: This creates a new one old ones will be garbage collected later, as the user may accidentally regenerate
	return CreateUserSSHKeypair(ctx, ownerID)
}
