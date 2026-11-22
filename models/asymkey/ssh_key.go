// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gitea.dev/models/auth"
	"gitea.dev/models/db"
	"gitea.dev/models/perm"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
	"gitea.dev/modules/timeutil"

	"xorm.io/builder"
)

// KeyType specifies the key type
type KeyType int

const (
	// KeyTypeUser specifies the user key
	KeyTypeUser = iota + 1
	// KeyTypeDeploy specifies the deploy key
	KeyTypeDeploy
	// KeyTypePrincipal specifies the authorized principal key
	KeyTypePrincipal
)

// PublicKey represents a user or deploy SSH public key.
type PublicKey struct {
	ID            int64           `xorm:"pk autoincr"`
	OwnerID       int64           `xorm:"INDEX NOT NULL"` // deploy-key doesn't have owner
	Name          string          `xorm:"NOT NULL"`
	Fingerprint   string          `xorm:"INDEX NOT NULL"`
	Content       string          `xorm:"MEDIUMTEXT NOT NULL"`
	Mode          perm.AccessMode `xorm:"NOT NULL DEFAULT 2"`
	Type          KeyType         `xorm:"NOT NULL DEFAULT 1"`
	LoginSourceID int64           `xorm:"NOT NULL DEFAULT 0"`

	CreatedUnix       timeutil.TimeStamp `xorm:"created"`
	UpdatedUnix       timeutil.TimeStamp `xorm:"updated"`
	HasRecentActivity bool               `xorm:"-"`
	HasUsed           bool               `xorm:"-"`
	Verified          bool               `xorm:"NOT NULL DEFAULT false"`
}

func init() {
	db.RegisterModel(new(PublicKey))
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (key *PublicKey) AfterLoad() {
	key.HasUsed = key.UpdatedUnix > key.CreatedUnix
	key.HasRecentActivity = key.UpdatedUnix.AddDuration(7*24*time.Hour) > timeutil.TimeStampNow()
}

// OmitEmail returns content of public key without email address.
func (key *PublicKey) OmitEmail() string {
	fields := strings.Split(key.Content, " ") // format: ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC... comment
	if len(fields) < 2 {
		setting.PanicInDevOrTesting("invalid public key %d content: %s", key.ID, key.Content)
		return "" // not a valid public key, it shouldn't really happen, the value is managed internally
	}
	return strings.Join(fields[:2], " ")
}

func addPublicKey(ctx context.Context, key *PublicKey) (err error) {
	if len(key.Fingerprint) == 0 {
		key.Fingerprint, err = CalcFingerprint(key.Content)
		if err != nil {
			return err
		}
	}

	// Save SSH key.
	if err = db.Insert(ctx, key); err != nil {
		return err
	}

	return appendAuthorizedKeysToFile(key)
}

// FindOrAddDeployPublicKey returns the shared public key that deploy keys of the given content link to, adding it on first use.
func FindOrAddDeployPublicKey(ctx context.Context, content string) (*PublicKey, error) {
	fingerprint, err := CalcFingerprint(content)
	if err != nil {
		return nil, err
	}

	pkey, exist, err := db.Get[PublicKey](ctx, builder.Eq{"fingerprint": fingerprint})
	if err != nil {
		return nil, err
	} else if exist {
		if pkey.Type != KeyTypeDeploy {
			return nil, ErrKeyAlreadyExist{0, fingerprint, ""}
		}
		return pkey, nil
	}

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
	return pkey, nil
}

// AddPublicKey adds new public key to database and authorized_keys file.
func AddPublicKey(ctx context.Context, ownerID int64, name, content string, authSourceID int64, verified bool) (*PublicKey, error) {
	log.Trace(content)

	fingerprint, err := CalcFingerprint(content)
	if err != nil {
		return nil, err
	}

	return db.WithTx2(ctx, func(ctx context.Context) (*PublicKey, error) {
		if err := checkKeyFingerprint(ctx, fingerprint); err != nil {
			return nil, err
		}

		// Key name of same user cannot be duplicated.
		has, err := db.GetEngine(ctx).
			Where("owner_id = ? AND name = ?", ownerID, name).
			Get(new(PublicKey))
		if err != nil {
			return nil, err
		} else if has {
			return nil, ErrKeyNameAlreadyUsed{ownerID, name}
		}

		key := &PublicKey{
			OwnerID:       ownerID,
			Name:          name,
			Fingerprint:   fingerprint,
			Content:       content,
			Mode:          perm.AccessModeWrite,
			Type:          KeyTypeUser,
			LoginSourceID: authSourceID,
			Verified:      verified,
		}
		if err = addPublicKey(ctx, key); err != nil {
			return nil, fmt.Errorf("addKey: %w", err)
		}

		return key, nil
	})
}

// GetPublicKeyByID returns public key by given ID.
func GetPublicKeyByID(ctx context.Context, keyID int64) (*PublicKey, error) {
	key := new(PublicKey)
	has, err := db.GetEngine(ctx).
		ID(keyID).
		Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrKeyNotExist{keyID}
	}
	return key, nil
}

func SearchPublicKeyForSSH(ctx context.Context, sshPubKey string) (*PublicKey, error) {
	// this function is designed to only accept SSH public keys,
	// because there might be different methods to calculate the fingerprint in the future (at the moment: "SHA256:...")
	fingerprint, err := CalcFingerprint(sshPubKey)
	if err != nil {
		return nil, err
	}
	key := new(PublicKey)
	has, err := db.GetEngine(ctx).
		Where("fingerprint = ?", fingerprint).
		Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrKeyNotExist{}
	}
	return key, nil
}

func SearchPrincipalKey(ctx context.Context, principalKey string) (*PublicKey, error) {
	// FIXME: this function is wrong, and there is no index on the content column
	// In the future, the existing principal keys should be migrated to use the fingerprint ("principal:{name}") instead of the content
	key := new(PublicKey)
	has, err := db.GetEngine(ctx).
		Where("content = ? AND type = ?", principalKey, KeyTypePrincipal).
		Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrKeyNotExist{}
	}
	return key, nil
}

type FindPublicKeyOptions struct {
	db.ListOptions
	OwnerID       int64
	Fingerprint   string
	KeyTypes      []KeyType
	NotKeytype    KeyType
	LoginSourceID int64
}

func (opts FindPublicKeyOptions) ToOrders() string {
	return "id"
}

func (opts FindPublicKeyOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.OwnerID != 0 {
		cond = cond.And(builder.Eq{"owner_id": opts.OwnerID})
	}
	if opts.Fingerprint != "" {
		cond = cond.And(builder.Eq{"fingerprint": opts.Fingerprint})
	}
	if len(opts.KeyTypes) > 0 {
		cond = cond.And(builder.In("`type`", opts.KeyTypes))
	}
	if opts.NotKeytype > 0 {
		cond = cond.And(builder.Neq{"`type`": opts.NotKeytype})
	}
	if opts.LoginSourceID > 0 {
		cond = cond.And(builder.Eq{"login_source_id": opts.LoginSourceID})
	}
	return cond
}

// UpdatePublicKeyUpdated updates public key use time.
func UpdatePublicKeyUpdated(ctx context.Context, id int64) error {
	// Check if key exists before update as affected rows count is unreliable
	//    and will return 0 affected rows if two updates are made at the same time
	if cnt, err := db.GetEngine(ctx).ID(id).Count(&PublicKey{}); err != nil {
		return err
	} else if cnt != 1 {
		return ErrKeyNotExist{id}
	}

	_, err := db.GetEngine(ctx).ID(id).Cols("updated_unix").Update(&PublicKey{
		UpdatedUnix: timeutil.TimeStampNow(),
	})
	if err != nil {
		return err
	}
	return nil
}

// PublicKeysAreExternallyManaged returns whether the provided KeyID represents an externally managed Key
func PublicKeysAreExternallyManaged(ctx context.Context, keys []*PublicKey) ([]bool, error) {
	sourceCache := make(map[int64]*auth.Source, len(keys))
	externals := make([]bool, len(keys))

	for i, key := range keys {
		if key.LoginSourceID == 0 {
			externals[i] = false
			continue
		}

		source, ok := sourceCache[key.LoginSourceID]
		if !ok {
			var err error
			source, err = auth.GetSourceByID(ctx, key.LoginSourceID)
			if err != nil {
				if auth.IsErrSourceNotExist(err) {
					externals[i] = false
					sourceCache[key.LoginSourceID] = &auth.Source{
						ID: key.LoginSourceID,
					}
					continue
				}
				return nil, err
			}
		}

		if sshKeyProvider, ok := source.Cfg.(auth.SSHKeyProvider); ok && sshKeyProvider.ProvidesSSHKeys() {
			// Disable setting SSH keys for this user
			externals[i] = true
		}
	}

	return externals, nil
}

// PublicKeyIsExternallyManaged returns whether the provided KeyID represents an externally managed Key
func PublicKeyIsExternallyManaged(ctx context.Context, id int64) (bool, error) {
	key, err := GetPublicKeyByID(ctx, id)
	if err != nil {
		return false, err
	}
	if key.LoginSourceID == 0 {
		return false, nil
	}
	source, err := auth.GetSourceByID(ctx, key.LoginSourceID)
	if err != nil {
		if auth.IsErrSourceNotExist(err) {
			return false, nil
		}
		return false, err
	}
	if sshKeyProvider, ok := source.Cfg.(auth.SSHKeyProvider); ok && sshKeyProvider.ProvidesSSHKeys() {
		// Disable setting SSH keys for this user
		return true, nil
	}
	return false, nil
}
