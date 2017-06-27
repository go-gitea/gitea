package models

import (
	"errors"
	"github.com/go-xorm/xorm"
	"time"
)

// LFSMetaObject stores metadata for LFS tracked files.
type LFSMetaObject struct {
	ID           int64     `xorm:"pk autoincr"`
	Oid          string    `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Size         int64     `xorm:"NOT NULL"`
	RepositoryID int64     `xorm:"UNIQUE(s) INDEX NOT NULL"`
	Existing     bool      `xorm:"-"`
	Created      time.Time `xorm:"-"`
	CreatedUnix  int64
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

	has, err := x.Get(m)
	if err != nil {
		return nil, err
	}

	if has {
		m.Existing = true
		return m, nil
	}

	sess := x.NewSession()
	defer sess.Close()
	if err = sess.Begin(); err != nil {
		return nil, err
	}

	if _, err = sess.Insert(m); err != nil {
		return nil, err
	}

	return m, sess.Commit()
}

// GetLFSMetaObjectByOid selects a LFSMetaObject entry from database by its OID.
// It may return ErrLFSObjectNotExist or a database error. If the error is nil,
// the returned pointer is a valid LFSMetaObject.
func GetLFSMetaObjectByOid(oid string) (*LFSMetaObject, error) {
	if len(oid) == 0 {
		return nil, ErrLFSObjectNotExist
	}

	m := &LFSMetaObject{Oid: oid}
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
func RemoveLFSMetaObjectByOid(oid string) error {
	if len(oid) == 0 {
		return ErrLFSObjectNotExist
	}

	sess := x.NewSession()
	defer sess.Close()
	if err := sess.Begin(); err != nil {
		return err
	}

	m := &LFSMetaObject{Oid: oid}

	if _, err := sess.Delete(m); err != nil {
		return err
	}

	return sess.Commit()
}

// BeforeInsert sets the time at which the LFSMetaObject was created.
func (m *LFSMetaObject) BeforeInsert() {
	m.CreatedUnix = time.Now().Unix()
}

// AfterSet stores the LFSMetaObject creation time in the database as local time.
func (m *LFSMetaObject) AfterSet(colName string, _ xorm.Cell) {
	switch colName {
	case "created_unix":
		m.Created = time.Unix(m.CreatedUnix, 0).Local()
	}
}
