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

type KeyType int

const (
	KeyTypeSSH KeyType = iota + 1
	KeyTypeToken
)

type DeployKey struct {
	ID     int64 `xorm:"pk autoincr"`
	KeyID  int64 `xorm:"INDEX"`
	RepoID int64 `xorm:"INDEX"`
	Name   string

	KeyType KeyType `xorm:"NOT NULL DEFAULT 1"`

	Fingerprint string
	PublicKey   *asymkey.PublicKey `xorm:"-"`

	TokenHash string `xorm:"INDEX"` // sha256 of the token, which carries enough entropy to need no salt
	Token     string `xorm:"-"`     // only set when the token is created

	Mode perm.AccessMode `xorm:"NOT NULL DEFAULT 1"`

	CreatedUnix timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix timeutil.TimeStamp `xorm:"updated"`
}

func (key *DeployKey) HasUsed() bool {
	return key.UpdatedUnix > key.CreatedUnix
}

func (key *DeployKey) HasRecentActivity() bool {
	return key.UpdatedUnix.AddDuration(7*24*time.Hour) > timeutil.TimeStampNow()
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

// AddDeployKeySSH add new deploy-key to database and authorized_keys file.
func AddDeployKeySSH(ctx context.Context, repoID int64, name, content string, accessMode perm.AccessMode) (*DeployKey, error) {
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

		key := &DeployKey{KeyID: pkey.ID, RepoID: repoID, KeyType: KeyTypeSSH, Name: name, Fingerprint: pkey.Fingerprint, Mode: accessMode}
		return key, db.Insert(ctx, key)
	})
}

// UpdateDeployKeyLastUsed marks the key as used now.
func UpdateDeployKeyLastUsed(ctx context.Context, id int64) error {
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
