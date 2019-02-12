package models

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"code.gitea.io/gitea/modules/util"
)

// LFSMetaObject stores metadata for LFS tracked files.
type LFSMetaObject struct {
	ID           int64          `xorm:"pk autoincr"`
	Oid          string         `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Size         int64          `xorm:"NOT NULL"`
	RepositoryID int64          `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Existing     bool           `xorm:"-"`
	CreatedUnix  util.TimeStamp `xorm:"created"`
}

// Pointer returns the string representation of an LFS pointer file
func (m *LFSMetaObject) Pointer() string {
	return fmt.Sprintf("%s\n%s%s\nsize %d\n", LFSMetaFileIdentifier, LFSMetaFileOidPrefix, m.Oid, m.Size)
}

// LFSTokenResponse defines the JSON structure in which the JWT token is stored.
// This structure is fetched via SSH and passed by the Git LFS client to the server
// endpoint for authorization.
type LFSTokenResponse struct {
	Header map[string]string `json:"header"`
	Href   string            `json:"href"`
}

var (
	// ErrLFSObjectNotExist is returned from lfs models functions in order
	// to differentiate between database and missing object errors.
	ErrLFSObjectNotExist = errors.New("LFS Meta object does not exist")
)

const (
	// LFSMetaFileIdentifier is the string appearing at the first line of LFS pointer files.
	// https://github.com/git-lfs/git-lfs/blob/master/docs/spec.md
	LFSMetaFileIdentifier = "version https://git-lfs.github.com/spec/v1"

	// LFSMetaFileOidPrefix appears in LFS pointer files on a line before the sha256 hash.
	LFSMetaFileOidPrefix = "oid sha256:"
)

// NewLFSMetaObject stores a given populated LFSMetaObject structure in the database
// if it is not already present.
func NewLFSMetaObject(m *LFSMetaObject) (*LFSMetaObject, error) {
	var err error

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return nil, err
	}

	has, err := sess.Get(m)
	if err != nil {
		return nil, err
	}

	if has {
		m.Existing = true
		return m, sess.Commit()
	}

	if _, err = sess.Insert(m); err != nil {
		return nil, err
	}

	return m, sess.Commit()
}

// GenerateLFSOid generates a Sha256Sum to represent an oid for arbitrary content
func GenerateLFSOid(content io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, content); err != nil {
		return "", err
	}
	sum := h.Sum(nil)
	return hex.EncodeToString(sum), nil
}

// GetLFSMetaObjectByOid selects a LFSMetaObject entry from database by its OID.
// It may return ErrLFSObjectNotExist or a database error. If the error is nil,
// the returned pointer is a valid LFSMetaObject.
func (repo *Repository) GetLFSMetaObjectByOid(oid string) (*LFSMetaObject, error) {
	if len(oid) == 0 {
		return nil, ErrLFSObjectNotExist
	}

	m := &LFSMetaObject{Oid: oid, RepositoryID: repo.ID}
	has, err := x.Get(m)
	if err != nil {
		return nil, err
	} else if !has {
		return nil, ErrLFSObjectNotExist
	}
	return m, nil
}

// RemoveLFSMetaObjectByOid removes a LFSMetaObject entry from database by its OID.
// It may return ErrLFSObjectNotExist or a database error.
func (repo *Repository) RemoveLFSMetaObjectByOid(oid string) error {
	if len(oid) == 0 {
		return ErrLFSObjectNotExist
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	m := &LFSMetaObject{Oid: oid, RepositoryID: repo.ID}
	if _, err := sess.Delete(m); err != nil {
		return err
	}

	return sess.Commit()
}
