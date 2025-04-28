// Copyright 2014 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"context"
	"fmt"
	"strings"
	"time"

	"code.gitea.io/gitea/models/auth"
	"code.gitea.io/gitea/models/db"
	"code.gitea.io/gitea/models/perm"
	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/timeutil"
	"code.gitea.io/gitea/modules/util"

	"golang.org/x/crypto/ssh"
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
	OwnerID       int64           `xorm:"INDEX NOT NULL"`
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
	return strings.Join(strings.Split(key.Content, " ")[:2], " ")
}

// AuthorizedString returns formatted public key string for authorized_keys file.
//
// TODO: Consider dropping this function
func (key *PublicKey) AuthorizedString() string {
	return AuthorizedStringForKey(key)
}

func addKey(ctx context.Context, key *PublicKey) (err error) {
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

// AddPublicKey adds new public key to database and authorized_keys file.
func AddPublicKey(ctx context.Context, ownerID int64, name, content string, authSourceID int64) (*PublicKey, error) {
	log.Trace(content)

	fingerprint, err := CalcFingerprint(content)
	if err != nil {
		return nil, err
	}

	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return nil, err
	}
	defer committer.Close()

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
	}
	if err = addKey(ctx, key); err != nil {
		return nil, fmt.Errorf("addKey: %w", err)
	}

	return key, committer.Commit()
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

// SearchPublicKeyByContent searches content as prefix (leak e-mail part)
// and returns public key found.
func SearchPublicKeyByContent(ctx context.Context, content string) (*PublicKey, error) {
	key := new(PublicKey)
	has, err := db.GetEngine(ctx).
		Where("content like ?", content+"%").
		Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrKeyNotExist{}
	}
	return key, nil
}

// SearchPublicKeyByContentExact searches content
// and returns public key found.
func SearchPublicKeyByContentExact(ctx context.Context, content string) (*PublicKey, error) {
	key := new(PublicKey)
	has, err := db.GetEngine(ctx).
		Where("content = ?", content).
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

func (opts FindPublicKeyOptions) ToConds() builder.Cond {
	cond := builder.NewCond()
	if opts.OwnerID > 0 {
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

// deleteKeysMarkedForDeletion returns true if ssh keys needs update
func deleteKeysMarkedForDeletion(ctx context.Context, keys []string) (bool, error) {
	// Start session
	ctx, committer, err := db.TxContext(ctx)
	if err != nil {
		return false, err
	}
	defer committer.Close()

	// Delete keys marked for deletion
	var sshKeysNeedUpdate bool
	for _, KeyToDelete := range keys {
		key, err := SearchPublicKeyByContent(ctx, KeyToDelete)
		if err != nil {
			log.Error("SearchPublicKeyByContent: %v", err)
			continue
		}
		if _, err = db.DeleteByID[PublicKey](ctx, key.ID); err != nil {
			log.Error("DeleteByID[PublicKey]: %v", err)
			continue
		}
		sshKeysNeedUpdate = true
	}

	if err := committer.Commit(); err != nil {
		return false, err
	}

	return sshKeysNeedUpdate, nil
}

// AddPublicKeysBySource add a users public keys. Returns true if there are changes.
func AddPublicKeysBySource(ctx context.Context, usr *user_model.User, s *auth.Source, sshPublicKeys []string) bool {
	var sshKeysNeedUpdate bool
	for _, sshKey := range sshPublicKeys {
		var err error
		found := false
		keys := []byte(sshKey)
	loop:
		for len(keys) > 0 && err == nil {
			var out ssh.PublicKey
			// We ignore options as they are not relevant to Gitea
			out, _, _, keys, err = ssh.ParseAuthorizedKey(keys)
			if err != nil {
				break loop
			}
			found = true
			marshalled := string(ssh.MarshalAuthorizedKey(out))
			marshalled = marshalled[:len(marshalled)-1]
			sshKeyName := fmt.Sprintf("%s-%s", s.Name, ssh.FingerprintSHA256(out))

			if _, err := AddPublicKey(ctx, usr.ID, sshKeyName, marshalled, s.ID); err != nil {
				if IsErrKeyAlreadyExist(err) {
					log.Trace("AddPublicKeysBySource[%s]: Public SSH Key %s already exists for user", sshKeyName, usr.Name)
				} else {
					log.Error("AddPublicKeysBySource[%s]: Error adding Public SSH Key for user %s: %v", sshKeyName, usr.Name, err)
				}
			} else {
				log.Trace("AddPublicKeysBySource[%s]: Added Public SSH Key for user %s", sshKeyName, usr.Name)
				sshKeysNeedUpdate = true
			}
		}
		if !found && err != nil {
			log.Warn("AddPublicKeysBySource[%s]: Skipping invalid Public SSH Key for user %s: %v", s.Name, usr.Name, sshKey)
		}
	}
	return sshKeysNeedUpdate
}

// SynchronizePublicKeys updates a users public keys. Returns true if there are changes.
func SynchronizePublicKeys(ctx context.Context, usr *user_model.User, s *auth.Source, sshPublicKeys []string) bool {
	var sshKeysNeedUpdate bool

	log.Trace("synchronizePublicKeys[%s]: Handling Public SSH Key synchronization for user %s", s.Name, usr.Name)

	// Get Public Keys from DB with current LDAP source
	var giteaKeys []string
	keys, err := db.Find[PublicKey](ctx, FindPublicKeyOptions{
		OwnerID:       usr.ID,
		LoginSourceID: s.ID,
	})
	if err != nil {
		log.Error("synchronizePublicKeys[%s]: Error listing Public SSH Keys for user %s: %v", s.Name, usr.Name, err)
	}

	for _, v := range keys {
		giteaKeys = append(giteaKeys, v.OmitEmail())
	}

	// Process the provided keys to remove duplicates and name part
	var providedKeys []string
	for _, v := range sshPublicKeys {
		sshKeySplit := strings.Split(v, " ")
		if len(sshKeySplit) > 1 {
			key := strings.Join(sshKeySplit[:2], " ")
			if !util.SliceContainsString(providedKeys, key) {
				providedKeys = append(providedKeys, key)
			}
		}
	}

	// Check if Public Key sync is needed
	if util.SliceSortedEqual(giteaKeys, providedKeys) {
		log.Trace("synchronizePublicKeys[%s]: Public Keys are already in sync for %s (Source:%v/DB:%v)", s.Name, usr.Name, len(providedKeys), len(giteaKeys))
		return false
	}
	log.Trace("synchronizePublicKeys[%s]: Public Key needs update for user %s (Source:%v/DB:%v)", s.Name, usr.Name, len(providedKeys), len(giteaKeys))

	// Add new Public SSH Keys that doesn't already exist in DB
	var newKeys []string
	for _, key := range providedKeys {
		if !util.SliceContainsString(giteaKeys, key) {
			newKeys = append(newKeys, key)
		}
	}
	if AddPublicKeysBySource(ctx, usr, s, newKeys) {
		sshKeysNeedUpdate = true
	}

	// Mark keys from DB that no longer exist in the source for deletion
	var giteaKeysToDelete []string
	for _, giteaKey := range giteaKeys {
		if !util.SliceContainsString(providedKeys, giteaKey) {
			log.Trace("synchronizePublicKeys[%s]: Marking Public SSH Key for deletion for user %s: %v", s.Name, usr.Name, giteaKey)
			giteaKeysToDelete = append(giteaKeysToDelete, giteaKey)
		}
	}

	// Delete keys from DB that no longer exist in the source
	needUpd, err := deleteKeysMarkedForDeletion(ctx, giteaKeysToDelete)
	if err != nil {
		log.Error("synchronizePublicKeys[%s]: Error deleting Public Keys marked for deletion for user %s: %v", s.Name, usr.Name, err)
	}
	if needUpd {
		sshKeysNeedUpdate = true
	}

	return sshKeysNeedUpdate
}
