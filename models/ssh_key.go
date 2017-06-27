// Copyright 2014 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bufio"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Unknwon/com"
	"github.com/go-xorm/xorm"
	"golang.org/x/crypto/ssh"

	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/process"
	"code.gitea.io/gitea/modules/setting"
)

const (
	tplCommentPrefix = `# gitea public key`
	tplPublicKey     = tplCommentPrefix + "\n" + `command="%s serv key-%d --config='%s'",no-port-forwarding,no-X11-forwarding,no-agent-forwarding,no-pty %s` + "\n"
)

var sshOpLocker sync.Mutex

// KeyType specifies the key type
type KeyType int

const (
	// KeyTypeUser specifies the user key
	KeyTypeUser = iota + 1
	// KeyTypeDeploy specifies the deploy key
	KeyTypeDeploy
)

// PublicKey represents a user or deploy SSH public key.
type PublicKey struct {
	ID          int64      `xorm:"pk autoincr"`
	OwnerID     int64      `xorm:"INDEX NOT NULL"`
	Name        string     `xorm:"NOT NULL"`
	Fingerprint string     `xorm:"NOT NULL"`
	Content     string     `xorm:"TEXT NOT NULL"`
	Mode        AccessMode `xorm:"NOT NULL DEFAULT 2"`
	Type        KeyType    `xorm:"NOT NULL DEFAULT 1"`

	Created           time.Time `xorm:"-"`
	CreatedUnix       int64
	Updated           time.Time `xorm:"-"` // Note: Updated must below Created for AfterSet.
	UpdatedUnix       int64
	HasRecentActivity bool `xorm:"-"`
	HasUsed           bool `xorm:"-"`
}

// BeforeInsert will be invoked by XORM before inserting a record
func (key *PublicKey) BeforeInsert() {
	key.CreatedUnix = time.Now().Unix()
}

// BeforeUpdate is invoked from XORM before updating this object.
func (key *PublicKey) BeforeUpdate() {
	key.UpdatedUnix = time.Now().Unix()
}

// AfterSet is invoked from XORM after setting the value of a field of this object.
func (key *PublicKey) AfterSet(colName string, _ xorm.Cell) {
	switch colName {
	case "created_unix":
		key.Created = time.Unix(key.CreatedUnix, 0).Local()
	case "updated_unix":
		key.Updated = time.Unix(key.UpdatedUnix, 0).Local()
		key.HasUsed = key.Updated.After(key.Created)
		key.HasRecentActivity = key.Updated.Add(7 * 24 * time.Hour).After(time.Now())
	}
}

// OmitEmail returns content of public key without email address.
func (key *PublicKey) OmitEmail() string {
	return strings.Join(strings.Split(key.Content, " ")[:2], " ")
}

// AuthorizedString returns formatted public key string for authorized_keys file.
func (key *PublicKey) AuthorizedString() string {
	return fmt.Sprintf(tplPublicKey, setting.AppPath, key.ID, setting.CustomConf, key.Content)
}

func extractTypeFromBase64Key(key string) (string, error) {
	b, err := base64.StdEncoding.DecodeString(key)
	if err != nil || len(b) < 4 {
		return "", fmt.Errorf("invalid key format: %v", err)
	}

	keyLength := int(binary.BigEndian.Uint32(b))
	if len(b) < 4+keyLength {
		return "", fmt.Errorf("invalid key format: not enough length %d", keyLength)
	}

	return string(b[4 : 4+keyLength]), nil
}

// parseKeyString parses any key string in OpenSSH or SSH2 format to clean OpenSSH string (RFC4253).
func parseKeyString(content string) (string, error) {
	// Transform all legal line endings to a single "\n".
	content = strings.NewReplacer("\r\n", "\n", "\r", "\n").Replace(content)
	// remove trailing newline (and beginning spaces too)
	content = strings.TrimSpace(content)
	lines := strings.Split(content, "\n")

	var keyType, keyContent, keyComment string

	if len(lines) == 1 {
		// Parse OpenSSH format.
		parts := strings.SplitN(lines[0], " ", 3)
		switch len(parts) {
		case 0:
			return "", errors.New("empty key")
		case 1:
			keyContent = parts[0]
		case 2:
			keyType = parts[0]
			keyContent = parts[1]
		default:
			keyType = parts[0]
			keyContent = parts[1]
			keyComment = parts[2]
		}

		// If keyType is not given, extract it from content. If given, validate it.
		t, err := extractTypeFromBase64Key(keyContent)
		if err != nil {
			return "", fmt.Errorf("extractTypeFromBase64Key: %v", err)
		}
		if len(keyType) == 0 {
			keyType = t
		} else if keyType != t {
			return "", fmt.Errorf("key type and content does not match: %s - %s", keyType, t)
		}
	} else {
		// Parse SSH2 file format.
		continuationLine := false

		for _, line := range lines {
			// Skip lines that:
			// 1) are a continuation of the previous line,
			// 2) contain ":" as that are comment lines
			// 3) contain "-" as that are begin and end tags
			if continuationLine || strings.ContainsAny(line, ":-") {
				continuationLine = strings.HasSuffix(line, "\\")
			} else {
				keyContent = keyContent + line
			}
		}

		t, err := extractTypeFromBase64Key(keyContent)
		if err != nil {
			return "", fmt.Errorf("extractTypeFromBase64Key: %v", err)
		}
		keyType = t
	}
	return keyType + " " + keyContent + " " + keyComment, nil
}

// writeTmpKeyFile writes key content to a temporary file
// and returns the name of that file, along with any possible errors.
func writeTmpKeyFile(content string) (string, error) {
	tmpFile, err := ioutil.TempFile(setting.SSH.KeyTestPath, "gitea_keytest")
	if err != nil {
		return "", fmt.Errorf("TempFile: %v", err)
	}
	defer tmpFile.Close()

	if _, err = tmpFile.WriteString(content); err != nil {
		return "", fmt.Errorf("WriteString: %v", err)
	}
	return tmpFile.Name(), nil
}

// SSHKeyGenParsePublicKey extracts key type and length using ssh-keygen.
func SSHKeyGenParsePublicKey(key string) (string, int, error) {
	// The ssh-keygen in Windows does not print key type, so no need go further.
	if setting.IsWindows {
		return "", 0, nil
	}

	tmpName, err := writeTmpKeyFile(key)
	if err != nil {
		return "", 0, fmt.Errorf("writeTmpKeyFile: %v", err)
	}
	defer os.Remove(tmpName)

	stdout, stderr, err := process.GetManager().Exec("SSHKeyGenParsePublicKey", setting.SSH.KeygenPath, "-lf", tmpName)
	if err != nil {
		return "", 0, fmt.Errorf("fail to parse public key: %s - %s", err, stderr)
	}
	if strings.Contains(stdout, "is not a public key file") {
		return "", 0, ErrKeyUnableVerify{stdout}
	}

	fields := strings.Split(stdout, " ")
	if len(fields) < 4 {
		return "", 0, fmt.Errorf("invalid public key line: %s", stdout)
	}

	keyType := strings.Trim(fields[len(fields)-1], "()\r\n")
	return strings.ToLower(keyType), com.StrTo(fields[0]).MustInt(), nil
}

// SSHNativeParsePublicKey extracts the key type and length using the golang SSH library.
// NOTE: ed25519 is not supported.
func SSHNativeParsePublicKey(keyLine string) (string, int, error) {
	fields := strings.Fields(keyLine)
	if len(fields) < 2 {
		return "", 0, fmt.Errorf("not enough fields in public key line: %s", keyLine)
	}

	raw, err := base64.StdEncoding.DecodeString(fields[1])
	if err != nil {
		return "", 0, err
	}

	pkey, err := ssh.ParsePublicKey(raw)
	if err != nil {
		if strings.Contains(err.Error(), "ssh: unknown key algorithm") {
			return "", 0, ErrKeyUnableVerify{err.Error()}
		}
		return "", 0, fmt.Errorf("ParsePublicKey: %v", err)
	}

	// The ssh library can parse the key, so next we find out what key exactly we have.
	switch pkey.Type() {
	case ssh.KeyAlgoDSA:
		rawPub := struct {
			Name       string
			P, Q, G, Y *big.Int
		}{}
		if err := ssh.Unmarshal(pkey.Marshal(), &rawPub); err != nil {
			return "", 0, err
		}
		// as per https://bugzilla.mindrot.org/show_bug.cgi?id=1647 we should never
		// see dsa keys != 1024 bit, but as it seems to work, we will not check here
		return "dsa", rawPub.P.BitLen(), nil // use P as per crypto/dsa/dsa.go (is L)
	case ssh.KeyAlgoRSA:
		rawPub := struct {
			Name string
			E    *big.Int
			N    *big.Int
		}{}
		if err := ssh.Unmarshal(pkey.Marshal(), &rawPub); err != nil {
			return "", 0, err
		}
		return "rsa", rawPub.N.BitLen(), nil // use N as per crypto/rsa/rsa.go (is bits)
	case ssh.KeyAlgoECDSA256:
		return "ecdsa", 256, nil
	case ssh.KeyAlgoECDSA384:
		return "ecdsa", 384, nil
	case ssh.KeyAlgoECDSA521:
		return "ecdsa", 521, nil
	case "ssh-ed25519": // TODO: replace with ssh constant when available
		return "ed25519", 256, nil
	}
	return "", 0, fmt.Errorf("unsupported key length detection for type: %s", pkey.Type())
}

// CheckPublicKeyString checks if the given public key string is recognized by SSH.
// It returns the actual public key line on success.
func CheckPublicKeyString(content string) (_ string, err error) {
	if setting.SSH.Disabled {
		return "", errors.New("SSH is disabled")
	}

	content, err = parseKeyString(content)
	if err != nil {
		return "", err
	}

	content = strings.TrimRight(content, "\n\r")
	if strings.ContainsAny(content, "\n\r") {
		return "", errors.New("only a single line with a single key please")
	}

	// remove any unnecessary whitespace now
	content = strings.TrimSpace(content)

	var (
		fnName  string
		keyType string
		length  int
	)
	if setting.SSH.StartBuiltinServer {
		fnName = "SSHNativeParsePublicKey"
		keyType, length, err = SSHNativeParsePublicKey(content)
	} else {
		fnName = "SSHKeyGenParsePublicKey"
		keyType, length, err = SSHKeyGenParsePublicKey(content)
	}
	if err != nil {
		return "", fmt.Errorf("%s: %v", fnName, err)
	}
	log.Trace("Key info [native: %v]: %s-%d", setting.SSH.StartBuiltinServer, keyType, length)

	if !setting.SSH.MinimumKeySizeCheck {
		return content, nil
	}
	if minLen, found := setting.SSH.MinimumKeySizes[keyType]; found && length >= minLen {
		return content, nil
	} else if found && length < minLen {
		return "", fmt.Errorf("key length is not enough: got %d, needs %d", length, minLen)
	}
	return "", fmt.Errorf("key type is not allowed: %s", keyType)
}

// appendAuthorizedKeysToFile appends new SSH keys' content to authorized_keys file.
func appendAuthorizedKeysToFile(keys ...*PublicKey) error {
	sshOpLocker.Lock()
	defer sshOpLocker.Unlock()

	fPath := filepath.Join(setting.SSH.RootPath, "authorized_keys")
	f, err := os.OpenFile(fPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return err
	}
	defer f.Close()

	// Note: chmod command does not support in Windows.
	if !setting.IsWindows {
		fi, err := f.Stat()
		if err != nil {
			return err
		}

		// .ssh directory should have mode 700, and authorized_keys file should have mode 600.
		if fi.Mode().Perm() > 0600 {
			log.Error(4, "authorized_keys file has unusual permission flags: %s - setting to -rw-------", fi.Mode().Perm().String())
			if err = f.Chmod(0600); err != nil {
				return err
			}
		}
	}

	for _, key := range keys {
		if _, err = f.WriteString(key.AuthorizedString()); err != nil {
			return err
		}
	}
	return nil
}

// checkKeyFingerprint only checks if key fingerprint has been used as public key,
// it is OK to use same key as deploy key for multiple repositories/users.
func checkKeyFingerprint(e Engine, fingerprint string) error {
	has, err := e.Get(&PublicKey{
		Fingerprint: fingerprint,
		Type:        KeyTypeUser,
	})
	if err != nil {
		return err
	} else if has {
		return ErrKeyAlreadyExist{0, fingerprint, ""}
	}
	return nil
}

func calcFingerprint(publicKeyContent string) (string, error) {
	// Calculate fingerprint.
	tmpPath, err := writeTmpKeyFile(publicKeyContent)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpPath)
	stdout, stderr, err := process.GetManager().Exec("AddPublicKey", "ssh-keygen", "-lf", tmpPath)
	if err != nil {
		return "", fmt.Errorf("'ssh-keygen -lf %s' failed with error '%s': %s", tmpPath, err, stderr)
	} else if len(stdout) < 2 {
		return "", errors.New("not enough output for calculating fingerprint: " + stdout)
	}
	return strings.Split(stdout, " ")[1], nil
}

func addKey(e Engine, key *PublicKey) (err error) {
	if len(key.Fingerprint) <= 0 {
		key.Fingerprint, err = calcFingerprint(key.Content)
		if err != nil {
			return err
		}
	}

	// Save SSH key.
	if _, err = e.Insert(key); err != nil {
		return err
	}

	// Don't need to rewrite this file if builtin SSH server is enabled.
	if setting.SSH.StartBuiltinServer {
		return nil
	}
	return appendAuthorizedKeysToFile(key)
}

// AddPublicKey adds new public key to database and authorized_keys file.
func AddPublicKey(ownerID int64, name, content string) (*PublicKey, error) {
	log.Trace(content)

	fingerprint, err := calcFingerprint(content)
	if err != nil {
		return nil, err
	}

	if err := checkKeyFingerprint(x, fingerprint); err != nil {
		return nil, err
	}

	// Key name of same user cannot be duplicated.
	has, err := x.
		Where("owner_id = ? AND name = ?", ownerID, name).
		Get(new(PublicKey))
	if err != nil {
		return nil, err
	} else if has {
		return nil, ErrKeyNameAlreadyUsed{ownerID, name}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return nil, err
	}

	key := &PublicKey{
		OwnerID:     ownerID,
		Name:        name,
		Fingerprint: fingerprint,
		Content:     content,
		Mode:        AccessModeWrite,
		Type:        KeyTypeUser,
	}
	if err = addKey(sess, key); err != nil {
		return nil, fmt.Errorf("addKey: %v", err)
	}

	return key, sess.Commit()
}

// GetPublicKeyByID returns public key by given ID.
func GetPublicKeyByID(keyID int64) (*PublicKey, error) {
	key := new(PublicKey)
	has, err := x.
		Id(keyID).
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
func SearchPublicKeyByContent(content string) (*PublicKey, error) {
	key := new(PublicKey)
	has, err := x.
		Where("content like ?", content+"%").
		Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrKeyNotExist{}
	}
	return key, nil
}

// ListPublicKeys returns a list of public keys belongs to given user.
func ListPublicKeys(uid int64) ([]*PublicKey, error) {
	keys := make([]*PublicKey, 0, 5)
	return keys, x.
		Where("owner_id = ?", uid).
		Find(&keys)
}

// UpdatePublicKey updates given public key.
func UpdatePublicKey(key *PublicKey) error {
	_, err := x.Id(key.ID).AllCols().Update(key)
	return err
}

// UpdatePublicKeyUpdated updates public key use time.
func UpdatePublicKeyUpdated(id int64) error {
	now := time.Now()
	cnt, err := x.ID(id).Cols("updated_unix").Update(&PublicKey{
		Updated:     now,
		UpdatedUnix: now.Unix(),
	})
	if err != nil {
		return err
	}
	if cnt != 1 {
		return ErrKeyNotExist{id}
	}
	return nil
}

// deletePublicKeys does the actual key deletion but does not update authorized_keys file.
func deletePublicKeys(e *xorm.Session, keyIDs ...int64) error {
	if len(keyIDs) == 0 {
		return nil
	}

	_, err := e.In("id", keyIDs).Delete(new(PublicKey))
	return err
}

// DeletePublicKey deletes SSH key information both in database and authorized_keys file.
func DeletePublicKey(doer *User, id int64) (err error) {
	key, err := GetPublicKeyByID(id)
	if err != nil {
		if IsErrKeyNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetPublicKeyByID: %v", err)
	}

	// Check if user has access to delete this key.
	if !doer.IsAdmin && doer.ID != key.OwnerID {
		return ErrKeyAccessDenied{doer.ID, key.ID, "public"}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if err = deletePublicKeys(sess, id); err != nil {
		return err
	}

	if err = sess.Commit(); err != nil {
		return err
	}

	return RewriteAllPublicKeys()
}

// RewriteAllPublicKeys removes any authorized key and rewrite all keys from database again.
// Note: x.Iterate does not get latest data after insert/delete, so we have to call this function
// outside any session scope independently.
func RewriteAllPublicKeys() error {
	sshOpLocker.Lock()
	defer sshOpLocker.Unlock()

	fPath := filepath.Join(setting.SSH.RootPath, "authorized_keys")
	tmpPath := fPath + ".tmp"
	t, err := os.OpenFile(tmpPath, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer func() {
		t.Close()
		os.Remove(tmpPath)
	}()

	if setting.SSH.AuthorizedKeysBackup && com.IsExist(fPath) {
		bakPath := fmt.Sprintf("%s_%d.gitea_bak", fPath, time.Now().Unix())
		if err = com.Copy(fPath, bakPath); err != nil {
			return err
		}
	}

	err = x.Iterate(new(PublicKey), func(idx int, bean interface{}) (err error) {
		_, err = t.WriteString((bean.(*PublicKey)).AuthorizedString())
		return err
	})
	if err != nil {
		return err
	}

	if com.IsExist(fPath) {
		f, err := os.Open(fPath)
		if err != nil {
			return err
		}
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.HasPrefix(line, tplCommentPrefix) {
				scanner.Scan()
				continue
			}
			_, err = t.WriteString(line + "\n")
			if err != nil {
				return err
			}
		}
		defer f.Close()
	}

	if err = os.Rename(tmpPath, fPath); err != nil {
		return err
	}

	return nil
}

// ________                .__                 ____  __.
// \______ \   ____ ______ |  |   ____ ___.__.|    |/ _|____ ___.__.
//  |    |  \_/ __ \\____ \|  |  /  _ <   |  ||      <_/ __ <   |  |
//  |    `   \  ___/|  |_> >  |_(  <_> )___  ||    |  \  ___/\___  |
// /_______  /\___  >   __/|____/\____// ____||____|__ \___  > ____|
//         \/     \/|__|               \/             \/   \/\/

// DeployKey represents deploy key information and its relation with repository.
type DeployKey struct {
	ID          int64 `xorm:"pk autoincr"`
	KeyID       int64 `xorm:"UNIQUE(s) INDEX"`
	RepoID      int64 `xorm:"UNIQUE(s) INDEX"`
	Name        string
	Fingerprint string
	Content     string `xorm:"-"`

	Created           time.Time `xorm:"-"`
	CreatedUnix       int64
	Updated           time.Time `xorm:"-"` // Note: Updated must below Created for AfterSet.
	UpdatedUnix       int64
	HasRecentActivity bool `xorm:"-"`
	HasUsed           bool `xorm:"-"`
}

// BeforeInsert will be invoked by XORM before inserting a record
func (key *DeployKey) BeforeInsert() {
	key.CreatedUnix = time.Now().Unix()
}

// BeforeUpdate is invoked from XORM before updating this object.
func (key *DeployKey) BeforeUpdate() {
	key.UpdatedUnix = time.Now().Unix()
}

// AfterSet is invoked from XORM after setting the value of a field of this object.
func (key *DeployKey) AfterSet(colName string, _ xorm.Cell) {
	switch colName {
	case "created_unix":
		key.Created = time.Unix(key.CreatedUnix, 0).Local()
	case "updated_unix":
		key.Updated = time.Unix(key.UpdatedUnix, 0).Local()
		key.HasUsed = key.Updated.After(key.Created)
		key.HasRecentActivity = key.Updated.Add(7 * 24 * time.Hour).After(time.Now())
	}
}

// GetContent gets associated public key content.
func (key *DeployKey) GetContent() error {
	pkey, err := GetPublicKeyByID(key.KeyID)
	if err != nil {
		return err
	}
	key.Content = pkey.Content
	return nil
}

func checkDeployKey(e Engine, keyID, repoID int64, name string) error {
	// Note: We want error detail, not just true or false here.
	has, err := e.
		Where("key_id = ? AND repo_id = ?", keyID, repoID).
		Get(new(DeployKey))
	if err != nil {
		return err
	} else if has {
		return ErrDeployKeyAlreadyExist{keyID, repoID}
	}

	has, err = e.
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
func addDeployKey(e *xorm.Session, keyID, repoID int64, name, fingerprint string) (*DeployKey, error) {
	if err := checkDeployKey(e, keyID, repoID, name); err != nil {
		return nil, err
	}

	key := &DeployKey{
		KeyID:       keyID,
		RepoID:      repoID,
		Name:        name,
		Fingerprint: fingerprint,
	}
	_, err := e.Insert(key)
	return key, err
}

// HasDeployKey returns true if public key is a deploy key of given repository.
func HasDeployKey(keyID, repoID int64) bool {
	has, _ := x.
		Where("key_id = ? AND repo_id = ?", keyID, repoID).
		Get(new(DeployKey))
	return has
}

// AddDeployKey add new deploy key to database and authorized_keys file.
func AddDeployKey(repoID int64, name, content string) (*DeployKey, error) {
	fingerprint, err := calcFingerprint(content)
	if err != nil {
		return nil, err
	}

	pkey := &PublicKey{
		Fingerprint: fingerprint,
		Mode:        AccessModeRead,
		Type:        KeyTypeDeploy,
	}
	has, err := x.Get(pkey)
	if err != nil {
		return nil, err
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return nil, err
	}

	// First time use this deploy key.
	if !has {
		pkey.Content = content
		pkey.Name = name
		if err = addKey(sess, pkey); err != nil {
			return nil, fmt.Errorf("addKey: %v", err)
		}
	}

	key, err := addDeployKey(sess, pkey.ID, repoID, name, pkey.Fingerprint)
	if err != nil {
		return nil, fmt.Errorf("addDeployKey: %v", err)
	}

	return key, sess.Commit()
}

// GetDeployKeyByID returns deploy key by given ID.
func GetDeployKeyByID(id int64) (*DeployKey, error) {
	key := new(DeployKey)
	has, err := x.Id(id).Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrDeployKeyNotExist{id, 0, 0}
	}
	return key, nil
}

// GetDeployKeyByRepo returns deploy key by given public key ID and repository ID.
func GetDeployKeyByRepo(keyID, repoID int64) (*DeployKey, error) {
	key := &DeployKey{
		KeyID:  keyID,
		RepoID: repoID,
	}
	has, err := x.Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrDeployKeyNotExist{0, keyID, repoID}
	}
	return key, nil
}

// UpdateDeployKey updates deploy key information.
func UpdateDeployKey(key *DeployKey) error {
	_, err := x.Id(key.ID).AllCols().Update(key)
	return err
}

// DeleteDeployKey deletes deploy key from its repository authorized_keys file if needed.
func DeleteDeployKey(doer *User, id int64) error {
	key, err := GetDeployKeyByID(id)
	if err != nil {
		if IsErrDeployKeyNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetDeployKeyByID: %v", err)
	}

	// Check if user has access to delete this key.
	if !doer.IsAdmin {
		repo, err := GetRepositoryByID(key.RepoID)
		if err != nil {
			return fmt.Errorf("GetRepositoryByID: %v", err)
		}
		yes, err := HasAccess(doer.ID, repo, AccessModeAdmin)
		if err != nil {
			return fmt.Errorf("HasAccess: %v", err)
		} else if !yes {
			return ErrKeyAccessDenied{doer.ID, key.ID, "deploy"}
		}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = sess.Id(key.ID).Delete(new(DeployKey)); err != nil {
		return fmt.Errorf("delete deploy key [%d]: %v", key.ID, err)
	}

	// Check if this is the last reference to same key content.
	has, err := sess.
		Where("key_id = ?", key.KeyID).
		Get(new(DeployKey))
	if err != nil {
		return err
	} else if !has {
		if err = deletePublicKeys(sess, key.KeyID); err != nil {
			return err
		}
	}

	return sess.Commit()
}

// ListDeployKeys returns all deploy keys by given repository ID.
func ListDeployKeys(repoID int64) ([]*DeployKey, error) {
	keys := make([]*DeployKey, 0, 5)
	return keys, x.
		Where("repo_id = ?", repoID).
		Find(&keys)
}
