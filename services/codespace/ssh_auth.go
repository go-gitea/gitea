// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"bytes"
	"context"
	"errors"
	"time"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	asymkey_model "gitea.dev/models/asymkey"
	auth_model "gitea.dev/models/auth"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/setting"

	"golang.org/x/crypto/ssh"
)

const (
	// SSHAuthDeniedInvalidCredentials means the submitted public key is not an allowed user key.
	SSHAuthDeniedInvalidCredentials = "invalid_credentials"
	// SSHAuthDeniedLoginRestricted means the Codespace creator cannot currently log in.
	SSHAuthDeniedLoginRestricted = "login_restricted"
	// SSHAuthDeniedCodespaceNotFound means the Codespace no longer exists.
	SSHAuthDeniedCodespaceNotFound = "codespace_not_found"
	// SSHAuthDeniedCodespaceNotRunning means the Codespace is not running.
	SSHAuthDeniedCodespaceNotRunning = "codespace_not_running"
	// SSHAuthDeniedManagerMismatch means the Codespace is bound to another Manager.
	SSHAuthDeniedManagerMismatch = "manager_mismatch"
	// SSHAuthDeniedStateUnavailable means the lifecycle state cannot accept a new SSH connection.
	SSHAuthDeniedStateUnavailable = "state_unavailable"
	// SSHAuthDeniedMetadataRebuilding means Runtime Metadata is absent or not ready for SSH.
	SSHAuthDeniedMetadataRebuilding = "metadata_rebuilding"
	// SSHAuthDeniedVersionExhausted means interaction_generation cannot advance.
	SSHAuthDeniedVersionExhausted = "version_exhausted"
)

// VerifySSHPublicKeyOptions contains one Gateway SSH public key authentication request.
type VerifySSHPublicKeyOptions struct {
	CodespaceUUID string
	PublicKey     []byte
}

type normalizedUserSSHKey struct {
	Wire        []byte
	Fingerprint string
}

// VerifySSHPublicKey authorizes one new Gateway SSH transport using a Gitea user SSH key.
func VerifySSHPublicKey(ctx context.Context, manager *codespace_model.Manager, opts VerifySSHPublicKeyOptions) (*codespacev1.VerifySSHPublicKeyResponse, error) {
	if manager == nil || manager.ID <= 0 {
		return nil, errors.New("manager is required")
	}
	if !setting.Codespace.Enabled {
		return denySSHAuth(SSHAuthDeniedStateUnavailable), nil
	}
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return nil, err
	}
	key, err := normalizeUserSSHPublicKey(opts.PublicKey)
	if err != nil {
		return denySSHAuth(SSHAuthDeniedInvalidCredentials), nil
	}

	var result *codespacev1.VerifySSHPublicKeyResponse
	err = globallock.LockAndDo(ctx, codespaceStateLockKey(opts.CodespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			currentManager, err := loadCodespaceManager(ctx, manager.ID)
			if err != nil {
				return err
			}
			if currentManager.RuntimeState != codespace_model.ManagerRuntimeStateOnline || isManagerOffline(currentManager) {
				result = denySSHAuth(SSHAuthDeniedStateUnavailable)
				return nil
			}

			codespace := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).ID(opts.CodespaceUUID).Get(codespace)
			if err != nil {
				return err
			}
			if !has {
				result = denySSHAuth(SSHAuthDeniedCodespaceNotFound)
				return nil
			}
			if codespace.ManagerID != manager.ID {
				result = denySSHAuth(SSHAuthDeniedManagerMismatch)
				return nil
			}
			if codespace.Status != codespace_model.StatusRunning {
				result = denySSHAuth(SSHAuthDeniedCodespaceNotRunning)
				return nil
			}
			if hasActiveOperation(codespace) && !isQueuedIdleStop(codespace) {
				result = denySSHAuth(SSHAuthDeniedStateUnavailable)
				return nil
			}

			entry, hasEntry, err := getRuntimeMetadataEntry(opts.CodespaceUUID)
			if err != nil {
				return err
			}
			if !hasEntry || !runtimeMetadataReadyForRunning(codespace, entry.Metadata) {
				result = denySSHAuth(SSHAuthDeniedMetadataRebuilding)
				return nil
			}

			user, err := user_model.GetUserByID(ctx, codespace.UserID)
			if err != nil {
				if user_model.IsErrUserNotExist(err) {
					result = denySSHAuth(SSHAuthDeniedLoginRestricted)
					return nil
				}
				return err
			}
			canUseGateway, err := codespaceUserCanLogIn(ctx, user)
			if err != nil {
				return err
			}
			if !canUseGateway {
				result = denySSHAuth(SSHAuthDeniedLoginRestricted)
				return nil
			}
			verified, err := userOwnsSSHKey(ctx, codespace.UserID, key)
			if err != nil {
				return err
			}
			if !verified {
				result = denySSHAuth(SSHAuthDeniedInvalidCredentials)
				return nil
			}

			now := time.Now().Unix()
			nextGeneration, err := advanceCodespaceInteraction(ctx, codespace, now)
			if err != nil {
				if err == errInteractionVersionExhausted {
					result = denySSHAuth(SSHAuthDeniedVersionExhausted)
					return nil
				}
				return err
			}
			result = &codespacev1.VerifySSHPublicKeyResponse{
				Outcome: &codespacev1.VerifySSHPublicKeyResponse_Allowed{
					Allowed: &codespacev1.SSHAuthBinding{
						UserId:                codespace.UserID,
						InteractionGeneration: nextGeneration,
					},
				},
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func normalizeUserSSHPublicKey(raw []byte) (normalizedUserSSHKey, error) {
	publicKey, err := ssh.ParsePublicKey(raw)
	if err != nil {
		return normalizedUserSSHKey{}, err
	}
	return normalizedUserSSHKey{
		Wire:        publicKey.Marshal(),
		Fingerprint: ssh.FingerprintSHA256(publicKey),
	}, nil
}

func codespaceUserCanLogIn(ctx context.Context, user *user_model.User) (bool, error) {
	if user == nil || !user.IsActive || user.ProhibitLogin || user.MustChangePassword {
		return false, nil
	}
	if !setting.TwoFactorAuthEnforced {
		return true, nil
	}
	has, err := auth_model.HasTwoFactorOrWebAuthn(ctx, user.ID)
	if err != nil {
		return false, err
	}
	return has, nil
}

func userOwnsSSHKey(ctx context.Context, ownerID int64, submitted normalizedUserSSHKey) (bool, error) {
	keys, err := db.Find[asymkey_model.PublicKey](ctx, asymkey_model.FindPublicKeyOptions{
		OwnerID:     ownerID,
		Fingerprint: submitted.Fingerprint,
		KeyTypes:    []asymkey_model.KeyType{asymkey_model.KeyTypeUser},
	})
	if err != nil {
		return false, err
	}
	for _, key := range keys {
		dbKey, _, _, _, err := ssh.ParseAuthorizedKey([]byte(key.Content))
		if err != nil {
			return false, err
		}
		if bytes.Equal(dbKey.Marshal(), submitted.Wire) {
			return true, nil
		}
	}
	return false, nil
}

func denySSHAuth(category string) *codespacev1.VerifySSHPublicKeyResponse {
	return &codespacev1.VerifySSHPublicKeyResponse{
		Outcome: &codespacev1.VerifySSHPublicKeyResponse_Denied{
			Denied: &codespacev1.FailureDetail{Category: category},
		},
	}
}
