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
	// ErrRequestGiteaTokenNotFound is returned when the Codespace no longer exists.
	ErrRequestGiteaTokenNotFound = errors.New("codespace not found")
	// ErrRequestGiteaTokenManagerMismatch is returned when the Codespace is bound to another Manager.
	ErrRequestGiteaTokenManagerMismatch = errors.New("codespace belongs to another manager")
	// ErrRequestGiteaTokenStateUnavailable is returned when the lifecycle state cannot receive a Token.
	ErrRequestGiteaTokenStateUnavailable = errors.New("codespace state does not allow requesting gitea token")
	// ErrRequestGiteaTokenManagerOffline is returned when the authenticated Manager is not usable.
	ErrRequestGiteaTokenManagerOffline = errors.New("manager is not online")
	// ErrRequestGiteaTokenUserNotFound is returned when the Codespace creator row is missing.
	ErrRequestGiteaTokenUserNotFound = errors.New("codespace user not found")
	// ErrResolveGiteaTokenUnmatched is returned when the plaintext token is not a Codespace Token.
	ErrResolveGiteaTokenUnmatched = errors.New("codespace gitea token unmatched")
	// ErrResolveGiteaTokenRejected is returned when the Codespace Token is malformed or not current.
	ErrResolveGiteaTokenRejected = errors.New("codespace gitea token rejected")
	// ErrResolveGiteaTokenForbidden is returned when a current Codespace Token cannot be used now.
	ErrResolveGiteaTokenForbidden = errors.New("codespace gitea token forbidden")
)

// RequestGiteaTokenOptions identifies the Codespace requesting its current Gitea Token.
type RequestGiteaTokenOptions struct {
	CodespaceUUID string
}

// RequestGiteaTokenResult contains the plaintext Token and public Gitea URL for Runtime clients.
type RequestGiteaTokenResult struct {
	Token     string
	ServerURL string
}

// GiteaTokenAuthSnapshot contains the current Codespace Token authentication result for one request.
type GiteaTokenAuthSnapshot struct {
	User          *user_model.User
	CodespaceUUID string
	RepoID        int64
	Scope         auth_model.AccessTokenScope
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

// RequestGiteaToken returns or issues the current Codespace Gitea Token.
func RequestGiteaToken(ctx context.Context, manager *codespace_model.Manager, opts RequestGiteaTokenOptions) (*RequestGiteaTokenResult, error) {
	if !setting.Codespace.Enabled {
		return nil, ErrRequestGiteaTokenStateUnavailable
	}
	if manager == nil || manager.ID <= 0 {
		return nil, errors.New("manager is required")
	}
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return nil, err
	}

	var token string
	err := globallock.LockAndDo(ctx, requestGiteaTokenLockKey(opts.CodespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			allowed, err := currentManagerAllowsOnlineOrRecovering(ctx, manager.ID)
			if err != nil {
				return err
			}
			if !allowed {
				return ErrRequestGiteaTokenManagerOffline
			}
			codespace := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).ID(opts.CodespaceUUID).Get(codespace)
			if err != nil {
				return err
			}
			if !has {
				return ErrRequestGiteaTokenNotFound
			}
			if codespace.ManagerID != manager.ID {
				return ErrRequestGiteaTokenManagerMismatch
			}
			if !requestGiteaTokenLifecycleAllows(codespace, time.Now().Unix()) {
				return ErrRequestGiteaTokenStateUnavailable
			}
			if _, err := user_model.GetUserByID(ctx, codespace.UserID); err != nil {
				if user_model.IsErrUserNotExist(err) {
					return ErrRequestGiteaTokenUserNotFound
				}
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
	return &RequestGiteaTokenResult{
		Token:     token,
		ServerURL: setting.AppURL,
	}, nil
}

func requestGiteaTokenLifecycleAllows(codespace *codespace_model.Codespace, now int64) bool {
	if codespace.Status == codespace_model.StatusRunning {
		return !hasActiveOperation(codespace)
	}
	return createOrResumeOperationActive(codespace, now)
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
		return &GiteaTokenAuthSnapshot{
			User:          user,
			CodespaceUUID: codespace.UUID,
			RepoID:        codespace.RepoID,
			Scope:         scope,
		}, nil
	}
	return nil, ErrResolveGiteaTokenRejected
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
		CreatedUnix:    time.Now().Unix(),
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

func requestGiteaTokenLockKey(codespaceUUID string) string {
	return "codespace_gitea_token_" + codespaceUUID
}
