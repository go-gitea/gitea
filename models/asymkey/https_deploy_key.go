// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"crypto/subtle"
	"encoding/hex"
	"time"

	auth_model "code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"xorm.io/builder"
)

// HTTPSDeployKeyTokenLength is the expected length of a hex-encoded deploy
// token (20 random bytes → 40 hex chars).
const HTTPSDeployKeyTokenLength = 40

// HTTPSDeployKey is a per-repository credential that authenticates Git
// operations over HTTPS without being tied to a user account. It mirrors the
// semantics of the SSH DeployKey (RepoID + Mode) but carries a hashed bearer
// token instead of a public-key fingerprint.
type HTTPSDeployKey struct {
	ID             int64           `xorm:"pk autoincr"`
	RepoID         int64           `xorm:"INDEX UNIQUE(s) NOT NULL"`
	Name           string          `xorm:"UNIQUE(s) NOT NULL"`
	TokenHash      string          `xorm:"UNIQUE NOT NULL"`
	TokenSalt      string          `xorm:"NOT NULL"`
	TokenLastEight string          `xorm:"INDEX"`
	Mode           perm.AccessMode `xorm:"NOT NULL DEFAULT 1"`

	// Token holds the plaintext token only on the row returned from a creation
	// call. It is never read from or written to the database.
	Token string `xorm:"-"`

	CreatedUnix       timeutil.TimeStamp `xorm:"INDEX created"`
	UpdatedUnix       timeutil.TimeStamp `xorm:"INDEX updated"`
	HasRecentActivity bool               `xorm:"-"`
	HasUsed           bool               `xorm:"-"`
}

// AfterLoad populates derived display fields after XORM reads a row.
func (k *HTTPSDeployKey) AfterLoad() {
	k.HasUsed = k.UpdatedUnix > k.CreatedUnix
	k.HasRecentActivity = k.UpdatedUnix.AddDuration(7*24*time.Hour) > timeutil.TimeStampNow()
}

// IsReadOnly reports whether the key grants only read access to its
// repository.
func (k *HTTPSDeployKey) IsReadOnly() bool {
	return k.Mode == perm.AccessModeRead
}

func init() {
	db.RegisterModel(new(HTTPSDeployKey))
}

// tokenIsValidFormat reports whether s looks like a serialized deploy token
// (40 lowercase hex chars). We reject everything else early so that an
// incidental basic-auth password can never collide with the token lookup.
func tokenIsValidFormat(s string) bool {
	if len(s) != HTTPSDeployKeyTokenLength {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

// AddHTTPSDeployKey creates a new HTTPS deploy key for the given repository
// and returns both the stored row and the plaintext token. The plaintext is
// only returned here; callers must surface it to the user exactly once.
func AddHTTPSDeployKey(ctx context.Context, repoID int64, name string, readOnly bool) (*HTTPSDeployKey, string, error) {
	if name == "" {
		return nil, "", util.NewInvalidArgumentErrorf("deploy key name must not be empty")
	}

	salt := util.CryptoRandomString(10)
	tokenBytes := util.CryptoRandomBytes(20)
	token := hex.EncodeToString(tokenBytes)

	mode := perm.AccessModeRead
	if !readOnly {
		mode = perm.AccessModeWrite
	}

	now := timeutil.TimeStampNow()
	key := &HTTPSDeployKey{
		RepoID:         repoID,
		Name:           name,
		TokenHash:      auth_model.HashToken(token, salt),
		TokenSalt:      salt,
		TokenLastEight: token[len(token)-8:],
		Mode:           mode,
		CreatedUnix:    now,
		UpdatedUnix:    now,
	}

	insertErr := db.WithTx(ctx, func(ctx context.Context) error {
		has, err := db.GetEngine(ctx).Where("repo_id = ? AND name = ?", repoID, name).Exist(new(HTTPSDeployKey))
		if err != nil {
			return err
		}
		if has {
			return ErrHTTPSDeployKeyNameAlreadyUsed{RepoID: repoID, Name: name}
		}

		_, err = db.GetEngine(ctx).NoAutoTime().Insert(key)
		if err != nil {
			return ErrHTTPSDeployKeyNameAlreadyUsed{RepoID: repoID, Name: name}
		}
		return nil
	})
	if insertErr != nil {
		return nil, "", insertErr
	}

	key.Token = token
	return key, token, nil
}

// GetHTTPSDeployKeyByID loads a single HTTPS deploy key by its primary key.
func GetHTTPSDeployKeyByID(ctx context.Context, id int64) (*HTTPSDeployKey, error) {
	key, exist, err := db.GetByID[HTTPSDeployKey](ctx, id)
	if err != nil {
		return nil, err
	}
	if !exist {
		return nil, ErrHTTPSDeployKeyNotExist{ID: id}
	}
	return key, nil
}

// ListHTTPSDeployKeysOptions filters a list query.
type ListHTTPSDeployKeysOptions struct {
	db.ListOptions
	RepoID int64
}

// ToConds implements db.FindOptions.
func (opt ListHTTPSDeployKeysOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opt.RepoID != 0 {
		cond = cond.And(builder.Eq{"repo_id": opt.RepoID})
	}
	return cond
}

// DeleteHTTPSDeployKey removes the key identified by (repoID, id). The repo
// scope is required so that a caller in one repository cannot drop a token
// belonging to another.
func DeleteHTTPSDeployKey(ctx context.Context, repoID, id int64) error {
	cnt, err := db.GetEngine(ctx).Where("repo_id = ?", repoID).ID(id).Delete(new(HTTPSDeployKey))
	if err != nil {
		return err
	}
	if cnt == 0 {
		return ErrHTTPSDeployKeyNotExist{ID: id, RepoID: repoID}
	}
	return nil
}

// VerifyHTTPSDeployToken returns the key that the given plaintext token
// authenticates, or ErrHTTPSDeployKeyNotExist if no key matches.
func VerifyHTTPSDeployToken(ctx context.Context, token string) (*HTTPSDeployKey, error) {
	if !tokenIsValidFormat(token) {
		_ = auth_model.HashToken(token, util.CryptoRandomString(10)) // dummy to prevent timing side-channel
		return nil, ErrHTTPSDeployKeyNotExist{}
	}

	lastEight := token[len(token)-8:]
	var candidates []HTTPSDeployKey
	if err := db.GetEngine(ctx).Where("token_last_eight = ?", lastEight).Find(&candidates); err != nil {
		return nil, err
	}

	for i := range candidates {
		expected := auth_model.HashToken(token, candidates[i].TokenSalt)
		if subtle.ConstantTimeCompare([]byte(candidates[i].TokenHash), []byte(expected)) == 1 {
			k := candidates[i]
			k.UpdatedUnix = timeutil.TimeStampNow()
			if _, err := db.GetEngine(ctx).ID(k.ID).Cols("updated_unix").Update(&k); err != nil {
				return nil, err
			}
			return &k, nil
		}
	}
	return nil, ErrHTTPSDeployKeyNotExist{}
}
