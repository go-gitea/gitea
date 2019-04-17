// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"bytes"
	"container/list"
	"crypto"
	"encoding/base64"
	"fmt"
	"hash"
	"io"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/git"
	"code.gitea.io/gitea/modules/log"
	"code.gitea.io/gitea/modules/util"

	"github.com/go-xorm/xorm"
	"github.com/keybase/go-crypto/openpgp"
	"github.com/keybase/go-crypto/openpgp/armor"
	"github.com/keybase/go-crypto/openpgp/packet"
)

// GPGKey represents a GPG key.
type GPGKey struct {
	ID                int64          `xorm:"pk autoincr"`
	OwnerID           int64          `xorm:"INDEX NOT NULL"`
	KeyID             string         `xorm:"INDEX CHAR(16) NOT NULL"`
	PrimaryKeyID      string         `xorm:"CHAR(16)"`
	Content           string         `xorm:"TEXT NOT NULL"`
	CreatedUnix       util.TimeStamp `xorm:"created"`
	ExpiredUnix       util.TimeStamp
	AddedUnix         util.TimeStamp
	SubsKey           []*GPGKey `xorm:"-"`
	Emails            []*EmailAddress
	CanSign           bool
	CanEncryptComms   bool
	CanEncryptStorage bool
	CanCertify        bool
}

//GPGKeyImport the original import of key
type GPGKeyImport struct {
	KeyID   string `xorm:"pk CHAR(16) NOT NULL"`
	Content string `xorm:"TEXT NOT NULL"`
}

// BeforeInsert will be invoked by XORM before inserting a record
func (key *GPGKey) BeforeInsert() {
	key.AddedUnix = util.TimeStampNow()
}

// AfterLoad is invoked from XORM after setting the values of all fields of this object.
func (key *GPGKey) AfterLoad(session *xorm.Session) {
	err := session.Where("primary_key_id=?", key.KeyID).Find(&key.SubsKey)
	if err != nil {
		log.Error("Find Sub GPGkeys[%s]: %v", key.KeyID, err)
	}
}

// ListGPGKeys returns a list of public keys belongs to given user.
func ListGPGKeys(uid int64) ([]*GPGKey, error) {
	keys := make([]*GPGKey, 0, 5)
	return keys, x.Where("owner_id=? AND primary_key_id=''", uid).Find(&keys)
}

// GetGPGKeyByID returns public key by given ID.
func GetGPGKeyByID(keyID int64) (*GPGKey, error) {
	key := new(GPGKey)
	has, err := x.ID(keyID).Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrGPGKeyNotExist{keyID}
	}
	return key, nil
}

// GetGPGImportByKeyID returns the import public armored key by given KeyID.
func GetGPGImportByKeyID(keyID string) (*GPGKeyImport, error) {
	key := new(GPGKeyImport)
	has, err := x.ID(keyID).Get(key)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrGPGKeyImportNotExist{keyID}
	}
	return key, nil
}

// checkArmoredGPGKeyString checks if the given key string is a valid GPG armored key.
// The function returns the actual public key on success
func checkArmoredGPGKeyString(content string) (*openpgp.Entity, error) {
	list, err := openpgp.ReadArmoredKeyRing(strings.NewReader(content))
	if err != nil {
		return nil, ErrGPGKeyParsing{err}
	}
	return list[0], nil
}

//addGPGKey add key, import and subkeys to database
func addGPGKey(e Engine, key *GPGKey, content string) (err error) {
	//Add GPGKeyImport
	if _, err = e.Insert(GPGKeyImport{
		KeyID:   key.KeyID,
		Content: content,
	}); err != nil {
		return err
	}
	// Save GPG primary key.
	if _, err = e.Insert(key); err != nil {
		return err
	}
	// Save GPG subs key.
	for _, subkey := range key.SubsKey {
		if err := addGPGSubKey(e, subkey); err != nil {
			return err
		}
	}
	return nil
}

//addGPGSubKey add subkeys to database
func addGPGSubKey(e Engine, key *GPGKey) (err error) {
	// Save GPG primary key.
	if _, err = e.Insert(key); err != nil {
		return err
	}
	// Save GPG subs key.
	for _, subkey := range key.SubsKey {
		if err := addGPGSubKey(e, subkey); err != nil {
			return err
		}
	}
	return nil
}

// AddGPGKey adds new public key to database.
func AddGPGKey(ownerID int64, content string) (*GPGKey, error) {
	ekey, err := checkArmoredGPGKeyString(content)
	if err != nil {
		return nil, err
	}

	// Key ID cannot be duplicated.
	has, err := x.Where("key_id=?", ekey.PrimaryKey.KeyIdString()).
		Get(new(GPGKey))
	if err != nil {
		return nil, err
	} else if has {
		return nil, ErrGPGKeyIDAlreadyUsed{ekey.PrimaryKey.KeyIdString()}
	}

	//Get DB session
	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return nil, err
	}

	key, err := parseGPGKey(ownerID, ekey)
	if err != nil {
		return nil, err
	}

	if err = addGPGKey(sess, key, content); err != nil {
		return nil, err
	}

	return key, sess.Commit()
}

//base64EncPubKey encode public key content to base 64
func base64EncPubKey(pubkey *packet.PublicKey) (string, error) {
	var w bytes.Buffer
	err := pubkey.Serialize(&w)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(w.Bytes()), nil
}

//base64DecPubKey decode public key content from base 64
func base64DecPubKey(content string) (*packet.PublicKey, error) {
	b, err := readerFromBase64(content)
	if err != nil {
		return nil, err
	}
	//Read key
	p, err := packet.Read(b)
	if err != nil {
		return nil, err
	}
	//Check type
	pkey, ok := p.(*packet.PublicKey)
	if !ok {
		return nil, fmt.Errorf("key is not a public key")
	}
	return pkey, nil
}

//GPGKeyToEntity retrieve the imported key and the traducted entity
func GPGKeyToEntity(k *GPGKey) (*openpgp.Entity, error) {
	impKey, err := GetGPGImportByKeyID(k.KeyID)
	if err != nil {
		return nil, err
	}
	return checkArmoredGPGKeyString(impKey.Content)
}

//parseSubGPGKey parse a sub Key
func parseSubGPGKey(ownerID int64, primaryID string, pubkey *packet.PublicKey, expiry time.Time) (*GPGKey, error) {
	content, err := base64EncPubKey(pubkey)
	if err != nil {
		return nil, err
	}
	return &GPGKey{
		OwnerID:           ownerID,
		KeyID:             pubkey.KeyIdString(),
		PrimaryKeyID:      primaryID,
		Content:           content,
		CreatedUnix:       util.TimeStamp(pubkey.CreationTime.Unix()),
		ExpiredUnix:       util.TimeStamp(expiry.Unix()),
		CanSign:           pubkey.CanSign(),
		CanEncryptComms:   pubkey.PubKeyAlgo.CanEncrypt(),
		CanEncryptStorage: pubkey.PubKeyAlgo.CanEncrypt(),
		CanCertify:        pubkey.PubKeyAlgo.CanSign(),
	}, nil
}

//getExpiryTime extract the expire time of primary key based on sig
func getExpiryTime(e *openpgp.Entity) time.Time {
	expiry := time.Time{}
	//Extract self-sign for expire date based on : https://github.com/golang/crypto/blob/master/openpgp/keys.go#L165
	var selfSig *packet.Signature
	for _, ident := range e.Identities {
		if selfSig == nil {
			selfSig = ident.SelfSignature
		} else if ident.SelfSignature.IsPrimaryId != nil && *ident.SelfSignature.IsPrimaryId {
			selfSig = ident.SelfSignature
			break
		}
	}
	if selfSig.KeyLifetimeSecs != nil {
		expiry = e.PrimaryKey.CreationTime.Add(time.Duration(*selfSig.KeyLifetimeSecs) * time.Second)
	}
	return expiry
}

//parseGPGKey parse a PrimaryKey entity (primary key + subs keys + self-signature)
func parseGPGKey(ownerID int64, e *openpgp.Entity) (*GPGKey, error) {
	pubkey := e.PrimaryKey
	expiry := getExpiryTime(e)

	//Parse Subkeys
	subkeys := make([]*GPGKey, len(e.Subkeys))
	for i, k := range e.Subkeys {
		subs, err := parseSubGPGKey(ownerID, pubkey.KeyIdString(), k.PublicKey, expiry)
		if err != nil {
			return nil, err
		}
		subkeys[i] = subs
	}

	//Check emails
	userEmails, err := GetEmailAddresses(ownerID)
	if err != nil {
		return nil, err
	}

	emails := make([]*EmailAddress, 0, len(e.Identities))
	for _, ident := range e.Identities {
		email := strings.ToLower(strings.TrimSpace(ident.UserId.Email))
		for _, e := range userEmails {
			if e.Email == email {
				emails = append(emails, e)
				break
			}
		}
	}

	//In the case no email as been found
	if len(emails) == 0 {
		failedEmails := make([]string, 0, len(e.Identities))
		for _, ident := range e.Identities {
			failedEmails = append(failedEmails, ident.UserId.Email)
		}
		return nil, ErrGPGNoEmailFound{failedEmails}
	}

	content, err := base64EncPubKey(pubkey)
	if err != nil {
		return nil, err
	}
	return &GPGKey{
		OwnerID:           ownerID,
		KeyID:             pubkey.KeyIdString(),
		PrimaryKeyID:      "",
		Content:           content,
		CreatedUnix:       util.TimeStamp(pubkey.CreationTime.Unix()),
		ExpiredUnix:       util.TimeStamp(expiry.Unix()),
		Emails:            emails,
		SubsKey:           subkeys,
		CanSign:           pubkey.CanSign(),
		CanEncryptComms:   pubkey.PubKeyAlgo.CanEncrypt(),
		CanEncryptStorage: pubkey.PubKeyAlgo.CanEncrypt(),
		CanCertify:        pubkey.PubKeyAlgo.CanSign(),
	}, nil
}

// deleteGPGKey does the actual key deletion
func deleteGPGKey(e *xorm.Session, keyID string) (int64, error) {
	if keyID == "" {
		return 0, fmt.Errorf("empty KeyId forbidden") //Should never happen but just to be sure
	}
	//Delete imported key
	n, err := e.Where("key_id=?", keyID).Delete(new(GPGKeyImport))
	if err != nil {
		return n, err
	}
	return e.Where("key_id=?", keyID).Or("primary_key_id=?", keyID).Delete(new(GPGKey))
}

// DeleteGPGKey deletes GPG key information in database.
func DeleteGPGKey(doer *User, id int64) (err error) {
	key, err := GetGPGKeyByID(id)
	if err != nil {
		if IsErrGPGKeyNotExist(err) {
			return nil
		}
		return fmt.Errorf("GetPublicKeyByID: %v", err)
	}

	// Check if user has access to delete this key.
	if !doer.IsAdmin && doer.ID != key.OwnerID {
		return ErrGPGKeyAccessDenied{doer.ID, key.ID}
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return err
	}

	if _, err = deleteGPGKey(sess, key.KeyID); err != nil {
		return err
	}

	return sess.Commit()
}

// CommitVerification represents a commit validation of signature
type CommitVerification struct {
	Verified    bool
	Reason      string
	SigningUser *User
	SigningKey  *GPGKey
}

// SignCommit represents a commit with validation of signature.
type SignCommit struct {
	Verification *CommitVerification
	*UserCommit
}

func readerFromBase64(s string) (io.Reader, error) {
	bs, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	return bytes.NewBuffer(bs), nil
}

func populateHash(hashFunc crypto.Hash, msg []byte) (hash.Hash, error) {
	h := hashFunc.New()
	if _, err := h.Write(msg); err != nil {
		return nil, err
	}
	return h, nil
}

// readArmoredSign read an armored signature block with the given type. https://sourcegraph.com/github.com/golang/crypto/-/blob/openpgp/read.go#L24:6-24:17
func readArmoredSign(r io.Reader) (body io.Reader, err error) {
	block, err := armor.Decode(r)
	if err != nil {
		return
	}
	if block.Type != openpgp.SignatureType {
		return nil, fmt.Errorf("expected '" + openpgp.SignatureType + "', got: " + block.Type)
	}
	return block.Body, nil
}

func extractSignature(s string) (*packet.Signature, error) {
	r, err := readArmoredSign(strings.NewReader(s))
	if err != nil {
		return nil, fmt.Errorf("Failed to read signature armor")
	}
	p, err := packet.Read(r)
	if err != nil {
		return nil, fmt.Errorf("Failed to read signature packet")
	}
	sig, ok := p.(*packet.Signature)
	if !ok {
		return nil, fmt.Errorf("Packet is not a signature")
	}
	return sig, nil
}

func verifySign(s *packet.Signature, h hash.Hash, k *GPGKey) error {
	//Check if key can sign
	if !k.CanSign {
		return fmt.Errorf("key can not sign")
	}
	//Decode key
	pkey, err := base64DecPubKey(k.Content)
	if err != nil {
		return err
	}
	return pkey.VerifySignature(h, s)
}

// ParseCommitWithSignature check if signature is good against keystore.
func ParseCommitWithSignature(c *git.Commit) *CommitVerification {
	if c.Signature != nil && c.Committer != nil {
		//Parsing signature
		sig, err := extractSignature(c.Signature.Signature)
		if err != nil { //Skipping failed to extract sign
			log.Error("SignatureRead err: %v", err)
			return &CommitVerification{
				Verified: false,
				Reason:   "gpg.error.extract_sign",
			}
		}

		//Find Committer account
		committer, err := GetUserByEmail(c.Committer.Email) //This find the user by primary email or activated email so commit will not be valid if email is not
		if err != nil {                                     //Skipping not user for commiter
			// We can expect this to often be an ErrUserNotExist. in the case
			// it is not, however, it is important to log it.
			if !IsErrUserNotExist(err) {
				log.Error("GetUserByEmail: %v", err)
			}
			return &CommitVerification{
				Verified: false,
				Reason:   "gpg.error.no_committer_account",
			}
		}

		keys, err := ListGPGKeys(committer.ID)
		if err != nil { //Skipping failed to get gpg keys of user
			log.Error("ListGPGKeys: %v", err)
			return &CommitVerification{
				Verified: false,
				Reason:   "gpg.error.failed_retrieval_gpg_keys",
			}
		}

		for _, k := range keys {
			//Pre-check (& optimization) that emails attached to key can be attached to the commiter email and can validate
			canValidate := false
			lowerCommiterEmail := strings.ToLower(c.Committer.Email)
			for _, e := range k.Emails {
				if e.IsActivated && strings.ToLower(e.Email) == lowerCommiterEmail {
					canValidate = true
					break
				}
			}
			if !canValidate {
				continue //Skip this key
			}

			//Generating hash of commit
			hash, err := populateHash(sig.Hash, []byte(c.Signature.Payload))
			if err != nil { //Skipping ailed to generate hash
				log.Error("PopulateHash: %v", err)
				return &CommitVerification{
					Verified: false,
					Reason:   "gpg.error.generate_hash",
				}
			}
			//We get PK
			if err := verifySign(sig, hash, k); err == nil {
				return &CommitVerification{ //Everything is ok
					Verified:    true,
					Reason:      fmt.Sprintf("%s <%s> / %s", c.Committer.Name, c.Committer.Email, k.KeyID),
					SigningUser: committer,
					SigningKey:  k,
				}
			}
			//And test also SubsKey
			for _, sk := range k.SubsKey {

				//Generating hash of commit
				hash, err := populateHash(sig.Hash, []byte(c.Signature.Payload))
				if err != nil { //Skipping ailed to generate hash
					log.Error("PopulateHash: %v", err)
					return &CommitVerification{
						Verified: false,
						Reason:   "gpg.error.generate_hash",
					}
				}
				if err := verifySign(sig, hash, sk); err == nil {
					return &CommitVerification{ //Everything is ok
						Verified:    true,
						Reason:      fmt.Sprintf("%s <%s> / %s", c.Committer.Name, c.Committer.Email, sk.KeyID),
						SigningUser: committer,
						SigningKey:  sk,
					}
				}
			}
		}
		return &CommitVerification{ //Default at this stage
			Verified: false,
			Reason:   "gpg.error.no_gpg_keys_found",
		}
	}

	return &CommitVerification{
		Verified: false,                         //Default value
		Reason:   "gpg.error.not_signed_commit", //Default value
	}
}

// ParseCommitsWithSignature checks if signaute of commits are corresponding to users gpg keys.
func ParseCommitsWithSignature(oldCommits *list.List) *list.List {
	var (
		newCommits = list.New()
		e          = oldCommits.Front()
	)
	for e != nil {
		c := e.Value.(UserCommit)
		newCommits.PushBack(SignCommit{
			UserCommit:   &c,
			Verification: ParseCommitWithSignature(c.Commit),
		})
		e = e.Next()
	}
	return newCommits
}
