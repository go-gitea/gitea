// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	asymkey_model "gitea.dev/models/asymkey"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/setting"
	asymkey_service "gitea.dev/services/asymkey"

	"golang.org/x/crypto/ssh"
	"xorm.io/builder"
)

var (
	// ErrEnsureGitSSHKeyNotFound is returned when the Codespace no longer exists.
	ErrEnsureGitSSHKeyNotFound = errors.New("codespace not found")
	// ErrEnsureGitSSHKeyManagerMismatch is returned when the Codespace is bound to another Manager.
	ErrEnsureGitSSHKeyManagerMismatch = errors.New("codespace belongs to another manager")
	// ErrEnsureGitSSHKeyStateUnavailable is returned when the lifecycle state cannot register a Git SSH key.
	ErrEnsureGitSSHKeyStateUnavailable = errors.New("codespace state does not allow git ssh key")
	// ErrEnsureGitSSHKeyManagerOffline is returned when the authenticated Manager is not usable.
	ErrEnsureGitSSHKeyManagerOffline = errors.New("manager is not online")
	// ErrEnsureGitSSHKeyUserNotFound is returned when the Codespace creator row is missing.
	ErrEnsureGitSSHKeyUserNotFound = errors.New("codespace user not found")
	// ErrEnsureGitSSHKeyLoginRestricted is returned when the Codespace creator cannot currently log in.
	ErrEnsureGitSSHKeyLoginRestricted = errors.New("codespace user login restricted")
	// ErrEnsureGitSSHKeyInvalidPublicKey is returned when the Runtime submitted an invalid key.
	ErrEnsureGitSSHKeyInvalidPublicKey = errors.New("invalid codespace git ssh public key")
	// ErrEnsureGitSSHKeyConflict is returned when an existing key binding cannot be reused.
	ErrEnsureGitSSHKeyConflict = errors.New("codespace git ssh key conflict")
	// ErrEnsureGitSSHKeyIntegrity is returned when persisted key rows are internally inconsistent.
	ErrEnsureGitSSHKeyIntegrity = errors.New("codespace git ssh key data integrity error")
	// ErrResolveGitSSHKeyBindingNotFound is returned when a Codespace key row has no Codespace binding.
	ErrResolveGitSSHKeyBindingNotFound = errors.New("codespace git ssh key binding not found")
	// ErrResolveGitSSHKeyBindingInvalid is returned when a Codespace key binding is internally inconsistent.
	ErrResolveGitSSHKeyBindingInvalid = errors.New("codespace git ssh key binding invalid")
	// ErrResolveGitSSHKeyRepoMismatch is returned when the Codespace key is used for another repository.
	ErrResolveGitSSHKeyRepoMismatch = errors.New("codespace git ssh key repository mismatch")
	// ErrResolveGitSSHKeyStateUnavailable is returned when the Codespace lifecycle cannot use Git SSH.
	ErrResolveGitSSHKeyStateUnavailable = errors.New("codespace git ssh key state unavailable")
	// ErrResolveGitSSHKeyUserNotFound is returned when the Codespace creator row is missing.
	ErrResolveGitSSHKeyUserNotFound = errors.New("codespace git ssh key user not found")
	// ErrResolveGitSSHKeyLoginRestricted is returned when the Codespace creator cannot currently log in.
	ErrResolveGitSSHKeyLoginRestricted = errors.New("codespace git ssh key user login restricted")
)

// EnsureGitSSHKeyOptions contains a Runtime Git SSH public key submitted by Manager.
type EnsureGitSSHKeyOptions struct {
	CodespaceUUID string
	PublicKey     []byte
}

// EnsureGitSSHKeyResult contains known_hosts material for the public Gitea SSH endpoint.
type EnsureGitSSHKeyResult struct {
	KnownHostsLines []string
}

type normalizedGitSSHKey struct {
	Content     string
	Fingerprint string
}

// EnsureGitSSHKey creates or confirms the Codespace-lifetime Git SSH public key.
func EnsureGitSSHKey(ctx context.Context, manager *codespace_model.Manager, opts EnsureGitSSHKeyOptions) (*EnsureGitSSHKeyResult, error) {
	if !setting.Codespace.Enabled {
		return nil, ErrEnsureGitSSHKeyStateUnavailable
	}
	if manager == nil || manager.ID <= 0 {
		return nil, errors.New("manager is required")
	}
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return nil, err
	}
	allowed, err := currentManagerAllowsOnlineOrRecovering(ctx, manager.ID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrEnsureGitSSHKeyManagerOffline
	}
	key, err := normalizeGitSSHPublicKey(opts.PublicKey)
	if err != nil {
		return nil, err
	}
	knownHostsLines, err := gitSSHCloneKnownHostsLines()
	if err != nil {
		return nil, err
	}

	err = globallock.LockAndDo(ctx, ensureGitSSHKeyCodespaceLockKey(opts.CodespaceUUID), func(ctx context.Context) error {
		allowed, err = currentManagerAllowsOnlineOrRecovering(ctx, manager.ID)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrEnsureGitSSHKeyManagerOffline
		}
		codespace, err := loadEnsureGitSSHKeyCodespace(ctx, manager.ID, opts.CodespaceUUID)
		if err != nil {
			return err
		}
		return globallock.LockAndDo(ctx, asymkey_model.PublicKeyFingerprintLockKey(key.Fingerprint), func(ctx context.Context) error {
			return ensureGitSSHKeyBinding(ctx, codespace, key)
		})
	})
	if err != nil {
		return nil, err
	}
	if err := asymkey_service.RewriteAllPublicKeys(ctx); err != nil {
		return nil, fmt.Errorf("%w: rewrite authorized keys: %v", ErrEnsureGitSSHKeyIntegrity, err)
	}
	return &EnsureGitSSHKeyResult{KnownHostsLines: knownHostsLines}, nil
}

// ResolveGitSSHKeyUser returns the Codespace creator allowed to use one Git SSH key for a repository.
func ResolveGitSSHKeyUser(ctx context.Context, key *asymkey_model.PublicKey, repoID int64) (*user_model.User, error) {
	if key == nil || key.ID <= 0 || key.Type != asymkey_model.KeyTypeCodespace {
		return nil, ErrResolveGitSSHKeyBindingInvalid
	}
	relation := new(codespace_model.SSHKey)
	has, err := db.GetEngine(ctx).Where("key_id = ?", key.ID).Get(relation)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrResolveGitSSHKeyBindingNotFound
	}

	codespace := new(codespace_model.Codespace)
	has, err = db.GetEngine(ctx).ID(relation.CodespaceUUID).Get(codespace)
	if err != nil {
		return nil, err
	}
	if !has || codespace.UserID != key.OwnerID {
		return nil, ErrResolveGitSSHKeyBindingInvalid
	}
	if repoID <= 0 || codespace.RepoID <= 0 || codespace.RepoID != repoID {
		return nil, ErrResolveGitSSHKeyRepoMismatch
	}
	if !codespaceGitSSHCommandAllowed(codespace, time.Now().Unix()) {
		return nil, ErrResolveGitSSHKeyStateUnavailable
	}

	user, err := user_model.GetUserByID(ctx, codespace.UserID)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			return nil, ErrResolveGitSSHKeyUserNotFound
		}
		return nil, err
	}
	if !user.IsActive || user.ProhibitLogin {
		return nil, ErrResolveGitSSHKeyLoginRestricted
	}
	return user, nil
}

func loadEnsureGitSSHKeyCodespace(ctx context.Context, managerID int64, codespaceUUID string) (*codespace_model.Codespace, error) {
	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(ctx).ID(codespaceUUID).Get(codespace)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrEnsureGitSSHKeyNotFound
	}
	if codespace.ManagerID != managerID {
		return nil, ErrEnsureGitSSHKeyManagerMismatch
	}
	if !ensureGitSSHKeyLifecycleAllows(codespace, time.Now().Unix()) {
		return nil, ErrEnsureGitSSHKeyStateUnavailable
	}
	user, err := user_model.GetUserByID(ctx, codespace.UserID)
	if err != nil {
		if user_model.IsErrUserNotExist(err) {
			return nil, ErrEnsureGitSSHKeyUserNotFound
		}
		return nil, err
	}
	canUseCodespace, err := codespaceUserSatisfiesLogin(ctx, user)
	if err != nil {
		return nil, err
	}
	if !canUseCodespace {
		return nil, ErrEnsureGitSSHKeyLoginRestricted
	}
	return codespace, nil
}

func codespaceGitSSHCommandAllowed(codespace *codespace_model.Codespace, now int64) bool {
	switch codespace.Status {
	case codespace_model.StatusCreating:
		return codespace.OperationType == codespace_model.OperationCreate &&
			codespace.OperationRVersion > 0 &&
			codespace.OperationDeadlineUnix > now
	case codespace_model.StatusRunning:
		return true
	case codespace_model.StatusStopped:
		return codespace.OperationType == codespace_model.OperationResume &&
			codespace.OperationRVersion > 0 &&
			codespace.OperationDeadlineUnix > now
	default:
		return false
	}
}

func ensureGitSSHKeyLifecycleAllows(codespace *codespace_model.Codespace, now int64) bool {
	return createOrResumeOperationActive(codespace, now)
}

func ensureGitSSHKeyBinding(ctx context.Context, codespace *codespace_model.Codespace, key normalizedGitSSHKey) error {
	return db.WithTx(ctx, func(ctx context.Context) error {
		relation := new(codespace_model.SSHKey)
		hasRelation, err := db.GetEngine(ctx).ID(codespace.UUID).Get(relation)
		if err != nil {
			return err
		}
		if hasRelation {
			return ensureExistingGitSSHKey(ctx, relation, key)
		}

		keys, err := db.Find[asymkey_model.PublicKey](ctx, asymkey_model.FindPublicKeyOptions{
			Fingerprint: key.Fingerprint,
		})
		if err != nil {
			return err
		}
		if len(keys) > 1 {
			return ErrEnsureGitSSHKeyIntegrity
		}
		if len(keys) == 1 {
			return ErrEnsureGitSSHKeyConflict
		}

		publicKey := &asymkey_model.PublicKey{
			OwnerID:     codespace.UserID,
			Name:        "codespace-" + codespace.UUID,
			Fingerprint: key.Fingerprint,
			Content:     key.Content,
			Mode:        perm.AccessModeWrite,
			Type:        asymkey_model.KeyTypeCodespace,
			Verified:    false,
		}
		if _, err := db.GetEngine(ctx).Insert(publicKey); err != nil {
			return err
		}
		_, err = db.GetEngine(ctx).Insert(&codespace_model.SSHKey{
			CodespaceUUID: codespace.UUID,
			KeyID:         publicKey.ID,
			CreatedUnix:   time.Now().Unix(),
		})
		return err
	})
}

func ensureExistingGitSSHKey(ctx context.Context, relation *codespace_model.SSHKey, key normalizedGitSSHKey) error {
	publicKey := new(asymkey_model.PublicKey)
	has, err := db.GetEngine(ctx).ID(relation.KeyID).Get(publicKey)
	if err != nil {
		return err
	}
	if !has || publicKey.Type != asymkey_model.KeyTypeCodespace {
		return ErrEnsureGitSSHKeyIntegrity
	}
	if publicKey.Content != key.Content || publicKey.Fingerprint != key.Fingerprint {
		return ErrEnsureGitSSHKeyConflict
	}
	return nil
}

func normalizeGitSSHPublicKey(raw []byte) (normalizedGitSSHKey, error) {
	publicKey, err := ssh.ParsePublicKey(raw)
	if err != nil {
		return normalizedGitSSHKey{}, fmt.Errorf("%w: %v", ErrEnsureGitSSHKeyInvalidPublicKey, err)
	}
	if publicKey.Type() != "ssh-ed25519" {
		return normalizedGitSSHKey{}, fmt.Errorf("%w: key type must be ssh-ed25519", ErrEnsureGitSSHKeyInvalidPublicKey)
	}
	content := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(publicKey)))
	return normalizedGitSSHKey{
		Content:     content,
		Fingerprint: ssh.FingerprintSHA256(publicKey),
	}, nil
}

func gitSSHKnownHostsLines() ([]string, error) {
	if len(setting.Codespace.GitSSHKnownHosts) > 0 {
		return configuredGitSSHKnownHostsLines(setting.Codespace.GitSSHKnownHosts)
	}
	if !setting.SSH.StartBuiltinServer {
		return nil, errors.New("codespace git ssh known hosts are required when builtin ssh server is disabled")
	}
	return builtinGitSSHKnownHostsLines()
}

func gitSSHCloneKnownHostsLines() ([]string, error) {
	if setting.SSH.Disabled {
		return nil, fmt.Errorf("%w: [server] DISABLE_SSH=true", ErrEnsureGitSSHKeyStateUnavailable)
	}
	lines, err := gitSSHKnownHostsLines()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrEnsureGitSSHKeyStateUnavailable, err.Error())
	}
	return lines, nil
}

func gitSSHCloneDisabledReason() string {
	if setting.SSH.Disabled {
		return "[server] DISABLE_SSH=true"
	}
	if _, err := gitSSHKnownHostsLines(); err != nil {
		return err.Error()
	}
	return ""
}

func configuredGitSSHKnownHostsLines(configured []string) ([]string, error) {
	hostPattern, err := gitSSHHostPattern()
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(configured))
	for _, line := range configured {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 3 {
			return nil, errors.New("invalid codespace git ssh known_hosts line")
		}
		if fields[0] != hostPattern {
			return nil, fmt.Errorf("codespace git ssh known_hosts host %q does not match %q", fields[0], hostPattern)
		}
		if _, _, _, _, err := ssh.ParseAuthorizedKey([]byte(strings.Join(fields[1:], " "))); err != nil {
			return nil, fmt.Errorf("invalid codespace git ssh known_hosts key for %q: %w", fields[0], err)
		}
		lines = append(lines, strings.Join(fields, " "))
	}
	if len(lines) == 0 {
		return nil, errors.New("codespace git ssh known hosts are required")
	}
	slices.Sort(lines)
	return lines, nil
}

func builtinGitSSHKnownHostsLines() ([]string, error) {
	hostPattern, err := gitSSHHostPattern()
	if err != nil {
		return nil, err
	}

	lines := make([]string, 0, len(setting.SSH.ServerHostKeys))
	for _, keyPath := range setting.SSH.ServerHostKeys {
		keyPath = strings.TrimSpace(keyPath)
		if keyPath == "" {
			continue
		}
		data, err := os.ReadFile(keyPath)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return nil, fmt.Errorf("read ssh host key %q: %w", keyPath, err)
		}
		signer, err := ssh.ParsePrivateKey(data)
		if err != nil {
			return nil, fmt.Errorf("parse ssh host key %q: %w", keyPath, err)
		}
		lines = append(lines, hostPattern+" "+strings.TrimSpace(string(ssh.MarshalAuthorizedKey(signer.PublicKey()))))
	}
	if len(lines) == 0 {
		return nil, errors.New("ssh host keys are required")
	}
	slices.Sort(lines)
	return lines, nil
}

func gitSSHHostPattern() (string, error) {
	host := strings.TrimSpace(setting.SSH.Domain)
	if host == "" {
		return "", errors.New("ssh domain is required")
	}
	hostPattern := host
	if setting.SSH.Port != 22 {
		hostPattern = fmt.Sprintf("[%s]:%d", host, setting.SSH.Port)
	}
	return hostPattern, nil
}

func deleteGitSSHKey(ctx context.Context, codespaceUUID string) error {
	relation := new(codespace_model.SSHKey)
	has, err := db.GetEngine(ctx).ID(codespaceUUID).Get(relation)
	if err != nil || !has {
		return err
	}
	if _, err := db.GetEngine(ctx).ID(codespaceUUID).Delete(new(codespace_model.SSHKey)); err != nil {
		return err
	}
	_, err = db.GetEngine(ctx).Where(builder.Eq{"id": relation.KeyID, "type": asymkey_model.KeyTypeCodespace}).Delete(new(asymkey_model.PublicKey))
	return err
}

func ensureGitSSHKeyCodespaceLockKey(codespaceUUID string) string {
	return "codespace_git_ssh_key_" + codespaceUUID
}
