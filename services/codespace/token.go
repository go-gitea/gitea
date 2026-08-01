// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	auth_model "gitea.dev/models/auth"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	perm_model "gitea.dev/models/perm"
	unit_model "gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/globallock"
	secret_module "gitea.dev/modules/secret"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/util"
)

const codespaceTokenPrefix = "gcs_"

// GiteaTokenScope is the fixed PAT category scope represented by a Codespace Token.
const GiteaTokenScope = "write:issue,write:repository,read:user"

var (
	// ErrRequestRuntimeAccessNotFound is returned when the Codespace no longer exists.
	ErrRequestRuntimeAccessNotFound = errors.New("codespace not found")
	// ErrRequestRuntimeAccessManagerMismatch is returned when the Codespace is bound to another Manager.
	ErrRequestRuntimeAccessManagerMismatch = errors.New("codespace belongs to another manager")
	// ErrRequestRuntimeAccessStateUnavailable is returned when the lifecycle state cannot receive runtime access material.
	ErrRequestRuntimeAccessStateUnavailable = errors.New("codespace state does not allow requesting runtime access")
	// ErrRequestRuntimeAccessManagerOffline is returned when the authenticated Manager is not usable.
	ErrRequestRuntimeAccessManagerOffline = errors.New("manager is not online")
	// ErrRequestRuntimeAccessUserNotFound is returned when the Codespace creator row is missing.
	ErrRequestRuntimeAccessUserNotFound = errors.New("codespace user not found")
	// ErrResolveGiteaTokenUnmatched is returned when the plaintext token is not a Codespace Token.
	ErrResolveGiteaTokenUnmatched = errors.New("codespace gitea token unmatched")
	// ErrResolveGiteaTokenRejected is returned when the Codespace Token is malformed or not current.
	ErrResolveGiteaTokenRejected = errors.New("codespace gitea token rejected")
	// ErrResolveGiteaTokenForbidden is returned when a current Codespace Token cannot be used now.
	ErrResolveGiteaTokenForbidden = errors.New("codespace gitea token forbidden")
)

// RequestRuntimeAccessOptions identifies the Codespace operation preparing runtime access.
type RequestRuntimeAccessOptions struct {
	CodespaceUUID     string
	OperationRVersion int64
	GitSSHPublicKey   []byte
}

// RequestRuntimeAccessResult contains the short-lived runtime inputs returned to a Manager.
type RequestRuntimeAccessResult struct {
	Token            string
	ServerURL        string
	Secrets          []RuntimeSecret
	GitSSHKnownHosts []string
}

type requestRuntimeCredentialsOptions struct {
	CodespaceUUID     string
	OperationRVersion int64
}

type requestRuntimeCredentialsResult struct {
	Token     string
	ServerURL string
	Secrets   []RuntimeSecret
}

// GiteaTokenAuthSnapshot contains the current Codespace Token authentication result for one request.
type GiteaTokenAuthSnapshot struct {
	User                  *user_model.User
	CodespaceUUID         string
	RepoID                int64
	Scope                 auth_model.AccessTokenScope
	RepositoryPermissions map[int64]map[unit_model.Type]perm_model.AccessMode
}

// CodespaceTokenAllowsRepository reports whether the Codespace authorization snapshot includes a repository unit permission.
func (s *GiteaTokenAuthSnapshot) CodespaceTokenAllowsRepository(repoID int64, unitType unit_model.Type, mode perm_model.AccessMode) bool {
	if s == nil || repoID <= 0 || mode < perm_model.AccessModeRead || mode > perm_model.AccessModeWrite {
		return false
	}
	if repoID == s.RepoID {
		return true
	}
	return s.RepositoryPermissions[repoID][unitType] >= mode
}

// CodespaceTokenAllowsAnyRepository reports whether the snapshot has any authenticated access to a repository.
func (s *GiteaTokenAuthSnapshot) CodespaceTokenAllowsAnyRepository(repoID int64) bool {
	if s == nil || repoID <= 0 {
		return false
	}
	return repoID == s.RepoID || len(s.RepositoryPermissions[repoID]) > 0
}

type giteaTokenAuthCandidate struct {
	Token     *codespace_model.GiteaToken    `xorm:"extends"`
	Codespace *codespace_model.Codespace     `xorm:"extends"`
	User      *user_model.User               `xorm:"extends"`
	TwoFactor *auth_model.TwoFactor          `xorm:"extends"`
	WebAuthn  *auth_model.WebAuthnCredential `xorm:"extends"`
}

func (c *giteaTokenAuthCandidate) hasTwoFactorOrWebAuthn() bool {
	return c != nil &&
		(c.TwoFactor != nil && c.TwoFactor.ID > 0 ||
			c.WebAuthn != nil && c.WebAuthn.ID > 0)
}

// CodespaceTokenRepoID returns the repository bound to this Codespace Token snapshot.
func (s *GiteaTokenAuthSnapshot) CodespaceTokenRepoID() int64 {
	if s == nil {
		return 0
	}
	return s.RepoID
}

// RequestRuntimeAccess returns the current token, user secrets, and Git SSH trust material.
func RequestRuntimeAccess(ctx context.Context, manager *codespace_model.Manager, opts RequestRuntimeAccessOptions) (*RequestRuntimeAccessResult, error) {
	if opts.OperationRVersion <= 0 {
		return nil, errors.New("operation_rversion must be positive")
	}
	credentials, err := requestRuntimeCredentials(ctx, manager, requestRuntimeCredentialsOptions{
		CodespaceUUID:     opts.CodespaceUUID,
		OperationRVersion: opts.OperationRVersion,
	})
	if err != nil {
		return nil, err
	}
	knownHosts, err := ensureRuntimeGitSSHKey(ctx, manager, runtimeGitSSHKeyOptions{
		CodespaceUUID:     opts.CodespaceUUID,
		OperationRVersion: opts.OperationRVersion,
		PublicKey:         opts.GitSSHPublicKey,
	})
	if err != nil {
		return nil, err
	}
	return &RequestRuntimeAccessResult{
		Token:            credentials.Token,
		ServerURL:        credentials.ServerURL,
		Secrets:          credentials.Secrets,
		GitSSHKnownHosts: knownHosts,
	}, nil
}

func requestRuntimeCredentials(ctx context.Context, manager *codespace_model.Manager, opts requestRuntimeCredentialsOptions) (*requestRuntimeCredentialsResult, error) {
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

	var (
		token   string
		secrets []RuntimeSecret
	)
	err := globallock.LockAndDo(ctx, requestRuntimeCredentialsLockKey(opts.CodespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			allowed, err := currentManagerAllowsOnlineOrRecovering(ctx, manager.ID)
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

			secrets, err = resolveCodespaceRuntimeSecrets(ctx, user, codespace)
			if err != nil {
				return err
			}

			existingToken, ok, err := readCurrentGiteaToken(ctx, opts.CodespaceUUID)
			if err != nil {
				return err
			}
			if ok {
				token = existingToken
				return nil
			}
			generatedToken, err := insertNewGiteaToken(ctx, opts.CodespaceUUID)
			if err != nil {
				return err
			}
			token = generatedToken
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return &requestRuntimeCredentialsResult{
		Token:     token,
		ServerURL: setting.AppURL,
		Secrets:   secrets,
	}, nil
}

func runtimeAccessLifecycleAllows(codespace *codespace_model.Codespace, operationRVersion, now int64) bool {
	if codespace.OperationRVersion != operationRVersion {
		return false
	}
	if codespace.Status == codespace_model.StatusRunning {
		return !hasActiveOperation(codespace)
	}
	return createOrResumeOperationActive(codespace, now)
}

func loadRuntimeAccessCodespace(ctx context.Context, managerID int64, codespaceUUID string, operationRVersion int64) (*codespace_model.Codespace, error) {
	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(ctx).ID(codespaceUUID).Get(codespace)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrRequestRuntimeAccessNotFound
	}
	if codespace.ManagerID != managerID {
		return nil, ErrRequestRuntimeAccessManagerMismatch
	}
	if !runtimeAccessLifecycleAllows(codespace, operationRVersion, time.Now().Unix()) {
		return nil, ErrRequestRuntimeAccessStateUnavailable
	}
	return codespace, nil
}

// ResolveGiteaToken authenticates a plaintext Codespace Token and returns the request snapshot.
func ResolveGiteaToken(ctx context.Context, token string) (*GiteaTokenAuthSnapshot, error) {
	if !strings.HasPrefix(token, codespaceTokenPrefix) {
		return nil, ErrResolveGiteaTokenUnmatched
	}
	if !validCodespaceTokenPlaintext(token) {
		return nil, ErrResolveGiteaTokenRejected
	}
	if !setting.Codespace.Enabled {
		return nil, fmt.Errorf("%w: state_unavailable", ErrResolveGiteaTokenForbidden)
	}

	candidates, err := findGiteaTokenAuthCandidates(ctx, token[len(token)-8:])
	if err != nil {
		return nil, err
	}
	for _, candidate := range candidates {
		if candidate.Token == nil || !verifyCodespaceGiteaToken(candidate.Token, token) {
			continue
		}
		codespace := candidate.Codespace
		if codespace == nil || codespace.UUID == "" {
			return nil, ErrResolveGiteaTokenRejected
		}
		if !giteaTokenLifecycleAllows(codespace, time.Now().Unix()) {
			return nil, fmt.Errorf("%w: codespace_not_running", ErrResolveGiteaTokenForbidden)
		}
		user := candidate.User
		if user == nil || user.ID == 0 {
			return nil, ErrResolveGiteaTokenRejected
		}
		if err := checkCodespaceTokenUserAllowed(user, candidate.hasTwoFactorOrWebAuthn()); err != nil {
			return nil, err
		}
		scope, err := auth_model.AccessTokenScope(GiteaTokenScope).Normalize()
		if err != nil {
			return nil, err
		}
		permissions, err := loadTokenRepositoryPermissions(ctx, codespace)
		if err != nil {
			return nil, err
		}
		return &GiteaTokenAuthSnapshot{
			User:                  user,
			CodespaceUUID:         codespace.UUID,
			RepoID:                codespace.RepoID,
			Scope:                 scope,
			RepositoryPermissions: permissions,
		}, nil
	}
	return nil, ErrResolveGiteaTokenRejected
}

func loadTokenRepositoryPermissions(ctx context.Context, codespace *codespace_model.Codespace) (map[int64]map[unit_model.Type]perm_model.AccessMode, error) {
	permissions := make(map[int64]map[unit_model.Type]perm_model.AccessMode)
	if codespace.PermissionAuthorizationID == 0 {
		return permissions, nil
	}
	authorization := new(codespace_model.PermissionAuthorization)
	has, err := db.GetEngine(ctx).ID(codespace.PermissionAuthorizationID).Get(authorization)
	if err != nil {
		return nil, err
	}
	if !has || authorization.UserID != codespace.UserID || authorization.SourceRepoID != codespace.RepoID || authorization.RevokedUnix != 0 {
		return permissions, nil
	}
	var rules []*codespace_model.PermissionRepository
	if err := db.GetEngine(ctx).Where("authorization_id = ?", authorization.ID).Find(&rules); err != nil {
		return nil, err
	}
	for _, rule := range rules {
		if rule.GrantedMode < perm_model.AccessModeRead || rule.GrantedMode > perm_model.AccessModeWrite {
			continue
		}
		if permissions[rule.TargetRepoID] == nil {
			permissions[rule.TargetRepoID] = make(map[unit_model.Type]perm_model.AccessMode)
		}
		permissions[rule.TargetRepoID][rule.UnitType] = rule.GrantedMode
	}
	return permissions, nil
}

func findGiteaTokenAuthCandidates(ctx context.Context, tokenLastEight string) ([]*giteaTokenAuthCandidate, error) {
	rows, err := db.GetEngine(ctx).
		Table("codespace_gitea_token").
		Where("codespace_gitea_token.token_last_eight = ?", tokenLastEight).
		Join("INNER", "codespace", "codespace.uuid = codespace_gitea_token.codespace_uuid").
		Join("INNER", "`user`", "`user`.id = codespace.user_id").
		Join("LEFT", "two_factor", "two_factor.uid = `user`.id").
		Join("LEFT", "webauthn_credential", "webauthn_credential.user_id = `user`.id").
		Limit(20).
		Rows(new(giteaTokenAuthCandidate))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	candidates := make([]*giteaTokenAuthCandidate, 0, 1)
	for rows.Next() {
		candidate := new(giteaTokenAuthCandidate)
		if err := rows.Scan(candidate); err != nil {
			return nil, err
		}
		candidates = append(candidates, candidate)
	}
	return candidates, rows.Err()
}

func giteaTokenLifecycleAllows(codespace *codespace_model.Codespace, now int64) bool {
	if codespace.Status == codespace_model.StatusRunning {
		return true
	}
	return createOrResumeOperationActive(codespace, now)
}

func checkCodespaceTokenUserAllowed(user *user_model.User, hasTwoFactorOrWebAuthn bool) error {
	if user == nil || !user.IsActive || user.ProhibitLogin || user.MustChangePassword {
		return fmt.Errorf("%w: login_restricted", ErrResolveGiteaTokenForbidden)
	}
	if setting.TwoFactorAuthEnforced && !hasTwoFactorOrWebAuthn {
		return fmt.Errorf("%w: login_restricted", ErrResolveGiteaTokenForbidden)
	}
	return nil
}

func readCurrentGiteaToken(ctx context.Context, codespaceUUID string) (string, bool, error) {
	row := new(codespace_model.GiteaToken)
	has, err := db.GetEngine(ctx).ID(codespaceUUID).Get(row)
	if err != nil || !has {
		return "", false, err
	}
	token, err := secret_module.DecryptSecret(setting.SecretKey, row.TokenEncrypted)
	if err != nil || !validCodespaceTokenPlaintext(token) || !verifyCodespaceGiteaToken(row, token) {
		if _, deleteErr := db.GetEngine(ctx).ID(codespaceUUID).Delete(new(codespace_model.GiteaToken)); deleteErr != nil {
			return "", false, deleteErr
		}
		return "", false, nil
	}
	return token, true, nil
}

func hasValidCurrentGiteaToken(ctx context.Context, codespaceUUID string) (bool, error) {
	row := new(codespace_model.GiteaToken)
	has, err := db.GetEngine(ctx).ID(codespaceUUID).Get(row)
	if err != nil || !has {
		return false, err
	}
	token, err := secret_module.DecryptSecret(setting.SecretKey, row.TokenEncrypted)
	if err != nil {
		return false, nil
	}
	return validCodespaceTokenPlaintext(token) && verifyCodespaceGiteaToken(row, token), nil
}

func insertNewGiteaToken(ctx context.Context, codespaceUUID string) (string, error) {
	token := generateCodespaceGiteaToken()
	salt := util.CryptoRandomString(10)
	encrypted, err := secret_module.EncryptSecret(setting.SecretKey, token)
	if err != nil {
		return "", err
	}
	row := &codespace_model.GiteaToken{
		CodespaceUUID:  codespaceUUID,
		TokenHash:      auth_model.HashToken(token, salt),
		TokenSalt:      salt,
		TokenLastEight: token[len(token)-8:],
		TokenEncrypted: encrypted,
	}
	if _, err := db.GetEngine(ctx).Insert(row); err != nil {
		existing, ok, readErr := readCurrentGiteaToken(ctx, codespaceUUID)
		if readErr != nil {
			return "", readErr
		}
		if ok {
			return existing, nil
		}
		return "", err
	}
	return token, nil
}

func verifyCodespaceGiteaToken(row *codespace_model.GiteaToken, token string) bool {
	if row == nil || row.TokenHash == "" || row.TokenSalt == "" {
		return false
	}
	if !validCodespaceTokenPlaintext(token) || row.TokenLastEight != token[len(token)-8:] {
		return false
	}
	hash := auth_model.HashToken(token, row.TokenSalt)
	return subtle.ConstantTimeCompare([]byte(row.TokenHash), []byte(hash)) == 1
}

func generateCodespaceGiteaToken() string {
	return codespaceTokenPrefix + hex.EncodeToString(util.CryptoRandomBytes(32))
}

func validCodespaceTokenPlaintext(token string) bool {
	return IsGiteaTokenPlaintext(token)
}

// IsGiteaTokenPlaintext reports whether token has the Codespace Token plaintext format.
func IsGiteaTokenPlaintext(token string) bool {
	if !IsGiteaTokenCandidate(token) {
		return false
	}
	raw := strings.TrimPrefix(token, codespaceTokenPrefix)
	if len(raw) != 64 {
		return false
	}
	_, err := hex.DecodeString(raw)
	return err == nil
}

// IsGiteaTokenCandidate reports whether token uses the Codespace Token prefix.
func IsGiteaTokenCandidate(token string) bool {
	return strings.HasPrefix(token, codespaceTokenPrefix)
}

func requestRuntimeCredentialsLockKey(codespaceUUID string) string {
	return "codespace_gitea_token_" + codespaceUUID
}
