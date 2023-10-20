// Copyright 2021 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"fmt"
	"time"

	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	"code.gitea.io/gitea/modules/timeutil"

	"xorm.io/builder"
)

// ________                .__                 ____  __.
// \______ \   ____ ______ |  |   ____ ___.__.|    |/ _|____ ___.__.
//  |    |  \_/ __ \\____ \|  |  /  _ <   |  ||      <_/ __ <   |  |
//  |    `   \  ___/|  |_> >  |_(  <_> )___  ||    |  \  ___/\___  |
// /_______  /\___  >   __/|____/\____// ____||____|__ \___  > ____|
//         \/     \/|__|               \/             \/   \/\/
//
// This file contains functions specific to DeployKeys

// DeployKey represents deploy key information and its relation with repository.
type DeployKey struct {
	ID          int64 `xorm:"pk autoincr"`
	KeyID       int64 `xorm:"UNIQUE(s) INDEX"`
	RepoID      int64 `xorm:"UNIQUE(s) INDEX"`
	Name        string
	Fingerprint string
	Content     string `xorm:"-"`

	Mode perm.AccessMode `xorm:"NOT NULL DEFAULT 1"`

	CreatedUnix       timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix       timeutil.TimeStamp `xorm:"updated"`
	HasRecentActivity bool               `xorm:"-"`
	HasUsed           bool               `xorm:"-"`
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (key *DeployKey) AfterLoad() {
	key.HasUsed = key.UpdatedUnix > key.CreatedUnix
	key.HasRecentActivity = key.UpdatedUnix.AddDuration(7*24*time.Hour) > timeutil.TimeStampNow()
}

// GetContent gets associated public key content.
func (key *DeployKey) GetContent(ctx context.Context) error {
	pkey, err := GetPublicKeyByID(ctx, key.KeyID)
	if err != nil {
		return err
	}
	key.Content = pkey.Content
	return nil
}

// IsReadOnly checks if the key can only be used for read operations, used by template
func (key *DeployKey) IsReadOnly() bool {
	return key.Mode == perm.AccessModeRead
}

func init() {
	db.RegisterModel(new(DeployKey))
}

func checkDeployKey(ctx context.Context, keyID, repoID int64, name string) error {
	// Note: We want error detail, not just true or false here.
	has, err := db.GetEngine(ctx).
		Where("key_id = ? AND repo_id = ?", keyID, repoID).
		Get(new(DeployKey))
	if err != nil {
		return err
	} else if has {
		return ErrDeployKeyAlreadyExist{keyID, repoID}
	}

	has, err = db.GetEngine(ctx).
		Where("repo_id = ? AND name = ?", repoID, name).
		Get(new(DeployKey))
	if err != nil {
		return err
	} else if has {
		return ErrDeployKeyNameAlreadyUsed{repoID, name}
	}

	return nil
}

// addDeployKey adds new key-repo relation.
func addDeployKey(ctx context.Context, keyID, repoID int64, name, fingerprint string, mode perm.AccessMode) (*DeployKey, error) {
	if err := checkDeployKey(ctx, keyID, repoID, name); err != nil {
		return nil, err
	}

	key := &DeployKey{
		KeyID:       keyID,
		RepoID:      repoID,
		Name:        name,
		Fingerprint: fingerprint,
		Mode:        mode,
	}
	return key, db.Insert(ctx, key)
}

// HasDeployKey returns true if public key is a deploy key of given repository.
func HasDeployKey(ctx context.Context, keyID, repoID int64) bool {
	has, _ := db.GetEngine(ctx).
		Where("key_id = ? AND repo_id = ?", keyID, repoID).
		Get(new(DeployKey))
	return has
}

// AddDeployKey add new deploy key to database and authorized_keys file.
func AddDeployKey(ctx context.Context, repoID int64, name, content string, readOnly bool) (*DeployKey, error) {
	fingerprint, err := CalcFingerprint(content)
	if err != nil {
		return nil, err
	}

	accessMode := perm.AccessModeRead
	if !readOnly {
		accessMode = perm.AccessModeWrite
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer committer.Close()

	pkey := &PublicKey{
		Fingerprint: fingerprint,
	}
	has, err := db.GetByBean(ctx, pkey)
	if err != nil {
		return nil, err
	}

	if has {
		if pkey.Type != KeyTypeDeploy {
			return nil, ErrKeyAlreadyExist{0, fingerprint, ""}
		}
	} else {
		// First time use this deploy key.
		pkey.Mode = accessMode
		pkey.Type = KeyTypeDeploy
		pkey.Content = content
		pkey.Name = name
		if err = addKey(ctx, pkey); err != nil {
			return nil, fmt.Errorf("addKey: %w", err)
		}
	}

	key, err := addDeployKey(ctx, pkey.ID, repoID, name, pkey.Fingerprint, accessMode)
	if err != nil {
		return nil, err
	}

	return key, committer.Commit()
}

// GetDeployKeyByID returns deploy key by given ID.
func GetDeployKeyByID(ctx context.Context, id int64) (*DeployKey, error) {
	key := new(DeployKey)
	has, err := db.GetEngine(ctx).ID(id).Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrDeployKeyNotExist{id, 0, 0}
	}
	return key, nil
}

// GetDeployKeyByRepo returns deploy key by given public key ID and repository ID.
func GetDeployKeyByRepo(ctx context.Context, keyID, repoID int64) (*DeployKey, error) {
	key := &DeployKey{
		KeyID:  keyID,
		RepoID: repoID,
	}
	has, err := db.GetByBean(ctx, key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrDeployKeyNotExist{0, keyID, repoID}
	}
	return key, nil
}

// IsDeployKeyExistByKeyID return true if there is at least one deploykey with the key id
func IsDeployKeyExistByKeyID(ctx context.Context, keyID int64) (bool, error) {
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

func (opt ListDeployKeysOptions) toCond() builder.Cond {
	cond := builder.NewCond()
	if opt.RepoID != 0 {
		cond = cond.And(builder.Eq{"repo_id": opt.RepoID})
	}
	if opt.KeyID != 0 {
		cond = cond.And(builder.Eq{"key_id": opt.KeyID})
	}
	if opt.Fingerprint != "" {
		cond = cond.And(builder.Eq{"fingerprint": opt.Fingerprint})
	}
	return cond
}

// ListDeployKeys returns a list of deploy keys matching the provided arguments.
func ListDeployKeys(ctx context.Context, opts *ListDeployKeysOptions) ([]*DeployKey, error) {
	sess := db.GetEngine(ctx).Where(opts.toCond())

	if opts.Page != 0 {
		sess = db.SetSessionPagination(sess, opts)

		keys := make([]*DeployKey, 0, opts.PageSize)
		return keys, sess.Find(&keys)
	}

	keys := make([]*DeployKey, 0, 5)
	return keys, sess.Find(&keys)
}

// CountDeployKeys returns count deploy keys matching the provided arguments.
func CountDeployKeys(ctx context.Context, opts *ListDeployKeysOptions) (int64, error) {
	return db.GetEngine(ctx).Where(opts.toCond()).Count(&DeployKey{})
}
