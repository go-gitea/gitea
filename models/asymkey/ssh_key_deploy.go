// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"fmt"
	"time"

	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

// DeployKey represents deploy key information and its relation with repository.
type DeployKey struct {
	ID          int64 `xorm:"pk autoincr"`
	KeyID       int64 `xorm:"UNIQUE(s) INDEX"`
	RepoID      int64 `xorm:"UNIQUE(s) INDEX"`
	Name        string
	Fingerprint string

	Mode perm.AccessMode `xorm:"NOT NULL DEFAULT 1"`

	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`

	PublicKey *PublicKey `xorm:"-"`
}

func (key *DeployKey) HasUsed() bool {
	return key.UpdatedUnix > key.CreatedUnix
}

func (key *DeployKey) HasRecentActivity() bool {
	return key.UpdatedUnix.AddDuration(7*24*time.Hour) > timeutil.TimeStampNow()
}

func (key *DeployKey) LoadPublicKey(ctx context.Context) (err error) {
	if key.PublicKey != nil {
		return nil
	}
	key.PublicKey, err = GetPublicKeyByID(ctx, key.KeyID)
	return err
}

// IsReadOnly checks if the key can only be used for read operations, used by template
func (key *DeployKey) IsReadOnly() bool {
	return key.Mode == perm.AccessModeRead
}

func init() {
	db.RegisterModel(new(DeployKey))
}

func checkDeployKey(ctx context.Context, repoID, publicKeyID int64, name string) error {
	// Note: We want error detail, not just true or false here.
	has, err := db.GetEngine(ctx).
		Where("repo_id=? AND (key_id=? OR name=?)", repoID, publicKeyID, name).
		Get(new(DeployKey))
	if err != nil {
		return err
	} else if has {
		return ErrDeployKeyAlreadyExist{publicKeyID, repoID}
	}
	return nil
}

// addDeployKey adds new key-repo relation.
func addDeployKey(ctx context.Context, repoID, publicKeyID int64, name, fingerprint string, mode perm.AccessMode) (*DeployKey, error) {
	if err := checkDeployKey(ctx, repoID, publicKeyID, name); err != nil {
		return nil, err
	}

	key := &DeployKey{KeyID: publicKeyID, RepoID: repoID, Name: name, Fingerprint: fingerprint, Mode: mode}
	return key, db.Insert(ctx, key)
}

// AddDeployKey add new deploy key to database and authorized_keys file.
func AddDeployKey(ctx context.Context, repoID int64, name, content string, accessMode perm.AccessMode) (*DeployKey, error) {
	fingerprint, err := CalcFingerprint(content)
	if err != nil {
		return nil, err
	}

	if accessMode != perm.AccessModeRead && accessMode != perm.AccessModeWrite {
		return nil, util.NewInvalidArgumentErrorf("invalid access mode")
	}
	return db.WithTx2(ctx, func(ctx context.Context) (*DeployKey, error) {
		pkey, exist, err := db.Get[PublicKey](ctx, builder.Eq{"fingerprint": fingerprint})
		if err != nil {
			return nil, err
		} else if exist {
			if pkey.Type != KeyTypeDeploy {
				return nil, ErrKeyAlreadyExist{0, fingerprint, ""}
			}
		} else {
			// First time use this deploy key, add a shared public key
			pkey = &PublicKey{
				Mode:        perm.AccessModeNone,
				Type:        KeyTypeDeploy,
				Name:        "(DeployKey)",
				Content:     content,
				Fingerprint: fingerprint,
			}
			if err = addPublicKey(ctx, pkey); err != nil {
				return nil, fmt.Errorf("addPublicKey: %w", err)
			}
		}
		return addDeployKey(ctx, repoID, pkey.ID, name, fingerprint, accessMode)
	})
}

// GetDeployKeyByID returns deploy key by given ID.
func GetDeployKeyByID(ctx context.Context, repoID, deployKeyID int64) (*DeployKey, error) {
	key, exist, err := db.Get[DeployKey](ctx, builder.Eq{"id": deployKeyID, "repo_id": repoID})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrDeployKeyNotExist{deployKeyID, 0, repoID}
	}
	return key, nil
}

// GetDeployKeyByRepoPublicKey returns deploy key by given public key ID and repository ID.
func GetDeployKeyByRepoPublicKey(ctx context.Context, repoID, publicKeyID int64) (*DeployKey, error) {
	key, exist, err := db.Get[DeployKey](ctx, builder.Eq{"key_id": publicKeyID, "repo_id": repoID})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrDeployKeyNotExist{0, publicKeyID, repoID}
	}
	return key, nil
}

// IsDeployKeyExistByPublicKeyID return true if there is at least one deploy-key with the key id
func IsDeployKeyExistByPublicKeyID(ctx context.Context, keyID int64) (bool, error) {
	return db.GetEngine(ctx).
		Where("key_id = ?", keyID).
		Get(new(DeployKey))
}

// UpdateDeployKeyCols updates deploy key information in the specified columns.
func UpdateDeployKeyCols(ctx context.Context, key *DeployKey, cols ...string) error {
	_, err := db.GetEngine(ctx).ID(key.ID).Cols(cols...).Update(key)
	return err
}

// ListDeployKeysOptions are options for ListDeployKeys
type ListDeployKeysOptions struct {
	db.ListOptions
	RepoID      int64
	KeyID       int64
	Fingerprint string
}

func (opt ListDeployKeysOptions) ToOrders() string {
	return "name"
}

func (opt ListDeployKeysOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	cond = cond.And(builder.Eq{"repo_id": opt.RepoID}) // repo ID must be used
	if opt.KeyID != 0 {
		cond = cond.And(builder.Eq{"key_id": opt.KeyID})
	}
	if opt.Fingerprint != "" {
		cond = cond.And(builder.Eq{"fingerprint": opt.Fingerprint})
	}
	return cond
}
