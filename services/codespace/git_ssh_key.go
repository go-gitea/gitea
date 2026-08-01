// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"crypto/rsa"
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
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/setting"
	asymkey_service "gitea.dev/services/asymkey"

	"golang.org/x/crypto/ssh"
	"xorm.io/builder"
)

var (
	// ErrRuntimeGitSSHKeyLoginRestricted is returned when the Codespace creator cannot currently log in.
	ErrRuntimeGitSSHKeyLoginRestricted = errors.New("codespace user login restricted")
	// ErrRuntimeGitSSHKeyInvalidPublicKey is returned when the Runtime submitted an invalid key.
	ErrRuntimeGitSSHKeyInvalidPublicKey = errors.New("invalid codespace git ssh public key")
	// ErrRuntimeGitSSHKeyConflict is returned when an existing key binding cannot be reused.
	ErrRuntimeGitSSHKeyConflict = errors.New("codespace git ssh key conflict")
	// ErrRuntimeGitSSHKeyIntegrity is returned when persisted key rows are internally inconsistent.
	ErrRuntimeGitSSHKeyIntegrity = errors.New("codespace git ssh key data integrity error")
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

type normalizedGitSSHKey struct {
	Content     string
	Fingerprint string
}

type runtimeGitSSHKeyOptions struct {
	CodespaceUUID     string
	OperationRVersion int64
	PublicKey         []byte
}

func ensureRuntimeGitSSHKey(ctx context.Context, manager *codespace_model.Manager, opts runtimeGitSSHKeyOptions) ([]string, error) {
	if !setting.Codespace.Enabled {
		return nil, ErrRequestRuntimeAccessStateUnavailable
	}
	if manager == nil || manager.ID <= 0 {
		return nil, errors.New("manager is required")
	}
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return nil, err
	}
	if opts.OperationRVersion <= 0 {
		return nil, errors.New("operation_rversion must be positive")
	}
	allowed, err := currentManagerAllowsOnlineOrRecovering(ctx, manager.ID)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrRequestRuntimeAccessManagerOffline
	}
	key, err := normalizeGitSSHPublicKey(opts.PublicKey)
	if err != nil {
		return nil, err
	}
	knownHostsLines, err := availableGitSSHKnownHostsLines()
	if err != nil {
		return nil, err
	}

	err = globallock.LockAndDo(ctx, runtimeGitSSHKeyLockKey(opts.CodespaceUUID), func(ctx context.Context) error {
		allowed, err = currentManagerAllowsOnlineOrRecovering(ctx, manager.ID)
		if err != nil {
			return err
		}
		if !allowed {
			return ErrRequestRuntimeAccessManagerOffline
		}
		codespace, err := loadRuntimeAccessCodespace(ctx, manager.ID, opts.CodespaceUUID, opts.OperationRVersion)
		if err != nil {
			return err
		}
		user, err := user_model.GetUserByID(ctx, codespace.UserID)
		if err != nil {
			if user_model.IsErrUserNotExist(err) {
				return ErrRequestRuntimeAccessUserNotFound
			}
			return err
		}
		canUseCodespace, err := codespaceUserCanLogIn(ctx, user)
		if err != nil {
			return err
		}
		if !canUseCodespace {
			return ErrRuntimeGitSSHKeyLoginRestricted
		}
		// Share the fingerprint lock with user and deploy key creation so their global uniqueness checks cannot race.
		return globallock.LockAndDo(ctx, asymkey_model.PublicKeyFingerprintLockKey(key.Fingerprint), func(ctx context.Context) error {
			return ensureGitSSHKeyBinding(ctx, codespace, key)
		})
	})
	if err != nil {
		return nil, err
	}
	if err := asymkey_service.RewriteAllPublicKeys(ctx); err != nil {
		return nil, fmt.Errorf("%w: rewrite authorized keys: %v", ErrRuntimeGitSSHKeyIntegrity, err)
	}
	return knownHostsLines, nil
}

// ResolveGitSSHKeyUser returns the Codespace creator allowed to use one Git SSH key for a repository.
func ResolveGitSSHKeyUser(ctx context.Context, key *asymkey_model.PublicKey, repoID int64, unitType unit.Type, mode perm.AccessMode) (*user_model.User, error) {
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
	if repoID <= 0 || codespace.RepoID <= 0 {
		return nil, ErrResolveGitSSHKeyRepoMismatch
	}
	if codespace.RepoID != repoID {
		authorization := new(codespace_model.PermissionAuthorization)
		has, err := db.GetEngine(ctx).ID(codespace.PermissionAuthorizationID).Where("user_id = ? AND source_repo_id = ? AND revoked_unix = 0", codespace.UserID, codespace.RepoID).Get(authorization)
		if err != nil {
			return nil, err
		}
		if !has {
			return nil, ErrResolveGitSSHKeyRepoMismatch
		}
		rule := new(codespace_model.PermissionRepository)
		has, err = db.GetEngine(ctx).
			Where("authorization_id = ? AND target_repo_id = ? AND unit_type = ? AND granted_mode >= ?", authorization.ID, repoID, unitType, mode).
			Get(rule)
		if err != nil {
			return nil, err
		}
		if !has {
			return nil, ErrResolveGitSSHKeyRepoMismatch
		}
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
			return ErrRuntimeGitSSHKeyIntegrity
		}
		if len(keys) == 1 {
			return ensureOrphanedGitSSHKeyBinding(ctx, codespace, keys[0], key)
		}

		publicKey := newCodespaceGitPublicKey(codespace, key)
		if _, err := db.GetEngine(ctx).Insert(publicKey); err != nil {
			return err
		}
		return insertCodespaceGitSSHKeyBinding(ctx, codespace.UUID, publicKey.ID)
	})
}

func newCodespaceGitPublicKey(codespace *codespace_model.Codespace, key normalizedGitSSHKey) *asymkey_model.PublicKey {
	return &asymkey_model.PublicKey{
		OwnerID:     codespace.UserID,
		Name:        codespaceGitSSHKeyName(codespace.UUID),
		Fingerprint: key.Fingerprint,
		Content:     key.Content,
		Mode:        perm.AccessModeWrite,
		Type:        asymkey_model.KeyTypeCodespace,
		Verified:    false,
	}
}

func ensureOrphanedGitSSHKeyBinding(ctx context.Context, codespace *codespace_model.Codespace, publicKey *asymkey_model.PublicKey, key normalizedGitSSHKey) error {
	expectedKey := newCodespaceGitPublicKey(codespace, key)
	if publicKey.Type != expectedKey.Type ||
		publicKey.OwnerID != expectedKey.OwnerID ||
		publicKey.Name != expectedKey.Name ||
		publicKey.Content != expectedKey.Content ||
		publicKey.Fingerprint != expectedKey.Fingerprint {
		return ErrRuntimeGitSSHKeyConflict
	}

	relation := new(codespace_model.SSHKey)
	has, err := db.GetEngine(ctx).Where("key_id = ?", publicKey.ID).Get(relation)
	if err != nil {
		return err
	}
	if has {
		return ErrRuntimeGitSSHKeyConflict
	}

	return insertCodespaceGitSSHKeyBinding(ctx, codespace.UUID, publicKey.ID)
}

func insertCodespaceGitSSHKeyBinding(ctx context.Context, codespaceUUID string, keyID int64) error {
	_, err := db.GetEngine(ctx).Insert(&codespace_model.SSHKey{
		CodespaceUUID: codespaceUUID,
		KeyID:         keyID,
	})
	return err
}

func codespaceGitSSHKeyName(codespaceUUID string) string {
	return "codespace-" + codespaceUUID
}

func ensureExistingGitSSHKey(ctx context.Context, relation *codespace_model.SSHKey, key normalizedGitSSHKey) error {
	publicKey := new(asymkey_model.PublicKey)
	has, err := db.GetEngine(ctx).ID(relation.KeyID).Get(publicKey)
	if err != nil {
		return err
	}
	if !has || publicKey.Type != asymkey_model.KeyTypeCodespace {
		return ErrRuntimeGitSSHKeyIntegrity
	}
	if publicKey.Content != key.Content || publicKey.Fingerprint != key.Fingerprint {
		return ErrRuntimeGitSSHKeyConflict
	}
	return nil
}

func normalizeGitSSHPublicKey(raw []byte) (normalizedGitSSHKey, error) {
	publicKey, err := ssh.ParsePublicKey(raw)
	if err != nil {
		return normalizedGitSSHKey{}, fmt.Errorf("%w: %v", ErrRuntimeGitSSHKeyInvalidPublicKey, err)
	}
	if err := validateGitSSHPublicKeyType(publicKey); err != nil {
		return normalizedGitSSHKey{}, err
	}
	content := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(publicKey)))
	return normalizedGitSSHKey{
		Content:     content,
		Fingerprint: ssh.FingerprintSHA256(publicKey),
	}, nil
}

func validateGitSSHPublicKeyType(publicKey ssh.PublicKey) error {
	switch publicKey.Type() {
	case ssh.KeyAlgoED25519:
		return nil
	case ssh.KeyAlgoRSA:
		cryptoKey, ok := publicKey.(ssh.CryptoPublicKey)
		if !ok {
			return fmt.Errorf("%w: ssh-rsa key cannot be inspected", ErrRuntimeGitSSHKeyInvalidPublicKey)
		}
		rsaKey, ok := cryptoKey.CryptoPublicKey().(*rsa.PublicKey)
		if !ok || rsaKey.N == nil || rsaKey.N.BitLen() != 4096 {
			return fmt.Errorf("%w: ssh-rsa key must be 4096 bits", ErrRuntimeGitSSHKeyInvalidPublicKey)
		}
		return nil
	default:
		return fmt.Errorf("%w: key type must be ssh-ed25519 or rsa-4096", ErrRuntimeGitSSHKeyInvalidPublicKey)
	}
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
		return nil, fmt.Errorf("%w: [server] DISABLE_SSH=true", ErrRequestRuntimeAccessStateUnavailable)
	}
	lines, err := gitSSHKnownHostsLines()
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrRequestRuntimeAccessStateUnavailable, err.Error())
	}
	return lines, nil
}

func availableGitSSHKnownHostsLines() ([]string, error) {
	protocol, err := createGitProtocol()
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestRuntimeAccessStateUnavailable, err)
	}
	capabilities, err := resolveGitTransportCapabilities(protocol)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrRequestRuntimeAccessStateUnavailable, err)
	}
	if !capabilities.SSHEnabled {
		return nil, nil
	}
	return gitSSHCloneKnownHostsLines()
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

func runtimeGitSSHKeyLockKey(codespaceUUID string) string {
	return "codespace_git_ssh_key_" + codespaceUUID
}
