// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package deploykey

import (
	"context"
	"time"

	"gitea.dev/models/asymkey"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	"gitea.dev/modules/timeutil"
	"gitea.dev/modules/util"

	"xorm.io/builder"
)

type AuthType int

const (
	// AuthTypeSSH authenticates over SSH with the public key of KeyID.
	AuthTypeSSH AuthType = iota + 1
	// AuthTypeToken authenticates over HTTPS with the token of TokenHash.
	AuthTypeToken
)

func (t AuthType) String() string {
	return util.Iif(t == AuthTypeToken, "token", "ssh")
}

// DeployKey represents deploy key information and its relation with repository.
type DeployKey struct {
	ID          int64    `xorm:"pk autoincr"`
	KeyID       int64    `xorm:"INDEX"`
	RepoID      int64    `xorm:"INDEX"`
	Type        AuthType `xorm:"NOT NULL DEFAULT 1"`
	Name        string
	Fingerprint string

	// Neither of these can be UNIQUE: one row type always leaves the other's columns empty,
	// so both key_id and token_hash collide across rows. Uniqueness is checked in code instead.
	TokenHash string `xorm:"INDEX"` // sha256 of the token, which carries enough entropy to need no salt
	Token     string `xorm:"-"`     // only set when the token is created

	Mode perm.AccessMode `xorm:"NOT NULL DEFAULT 1"`

	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`

	PublicKey *asymkey.PublicKey `xorm:"-"`
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
	key.PublicKey, err = asymkey.GetPublicKeyByID(ctx, key.KeyID)
	return err
}

// IsReadOnly checks if the key can only be used for read operations, used by template
func (key *DeployKey) IsReadOnly() bool {
	return key.Mode == perm.AccessModeRead
}

func init() {
	db.RegisterModel(new(DeployKey))
}

func checkDeployKeyName(ctx context.Context, repoID int64, name string) error {
	has, err := db.Exist[DeployKey](ctx, builder.Eq{"repo_id": repoID, "name": name})
	if err != nil {
		return err
	} else if has {
		return ErrDeployKeyNameAlreadyUsed{repoID, name}
	}
	return nil
}

// AddDeployKey add new deploy key to database and authorized_keys file.
func AddDeployKey(ctx context.Context, repoID int64, name, content string, accessMode perm.AccessMode) (*DeployKey, error) {
	if accessMode != perm.AccessModeRead && accessMode != perm.AccessModeWrite {
		return nil, util.NewInvalidArgumentErrorf("invalid access mode")
	}
	return db.WithTx2(ctx, func(ctx context.Context) (*DeployKey, error) {
		pkey, err := asymkey.FindOrAddDeployPublicKey(ctx, content)
		if err != nil {
			return nil, err
		}
		if has, err := db.Exist[DeployKey](ctx, builder.Eq{"repo_id": repoID, "key_id": pkey.ID}); err != nil {
			return nil, err
		} else if has {
			return nil, ErrDeployKeyAlreadyExist{pkey.ID, repoID}
		}
		if err := checkDeployKeyName(ctx, repoID, name); err != nil {
			return nil, err
		}

		key := &DeployKey{KeyID: pkey.ID, RepoID: repoID, Type: AuthTypeSSH, Name: name, Fingerprint: pkey.Fingerprint, Mode: accessMode}
		return key, db.Insert(ctx, key)
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
	// the type is part of the condition because every token row carries key id 0
	key, exist, err := db.Get[DeployKey](ctx, builder.Eq{"key_id": publicKeyID, "repo_id": repoID, "type": AuthTypeSSH})
	if err != nil {
		return nil, err
	} else if !exist {
		return nil, ErrDeployKeyNotExist{0, publicKeyID, repoID}
	}
	return key, nil
}

// IsDeployKeyExistByPublicKeyID return true if there is at least one deploy-key with the key id
func IsDeployKeyExistByPublicKeyID(ctx context.Context, keyID int64) (bool, error) {
	return db.Exist[DeployKey](ctx, builder.Eq{"key_id": keyID, "type": AuthTypeSSH})
}

// UpdateDeployKeyUpdated marks the key as used now.
func UpdateDeployKeyUpdated(ctx context.Context, id int64) error {
	_, err := db.GetEngine(ctx).ID(id).Cols("updated_unix").Update(&DeployKey{UpdatedUnix: timeutil.TimeStampNow()})
	return err
}

// ListDeployKeysOptions are options for ListDeployKeys
type ListDeployKeysOptions struct {
	db.ListOptions
	RepoID      int64
	KeyID       int64
	Fingerprint string
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
