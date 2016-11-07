package models

import (
	"errors"
	"github.com/go-xorm/xorm"
	"time"
)

type LFSMetaObject struct {
	Oid          string    `xorm:"pk"`
	Size         int64     `xorm:"NOT NULL"`
	RepositoryID int64     `xorm:"NOT NULL"`
	Existing     bool      `xorm:"-"`
	Created      time.Time `xorm:"-"`
	CreatedUnix  int64
}

var (
	ErrLFSObjectNotExist = errors.New("LFS Meta object does not exist")
)

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
	defer sessionRelease(sess)
	if err = sess.Begin(); err != nil {
		return nil, err
	}

	if _, err = sess.Insert(m); err != nil {
		return nil, err
	}

	return m, sess.Commit()
}

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

func RemoveLFSMetaObjectByOid(oid string) error {
	if len(oid) == 0 {
		return ErrLFSObjectNotExist
	}

	sess := x.NewSession()
	defer sessionRelease(sess)
	if err := sess.Begin(); err != nil {
		return err
	}

	m := &LFSMetaObject{Oid: oid}

	if _, err := sess.Delete(m); err != nil {
		return err
	}

	return sess.Commit()
}

func (m *LFSMetaObject) BeforeInsert() {
	m.CreatedUnix = time.Now().Unix()
}

func (m *LFSMetaObject) AfterSet(colName string, _ xorm.Cell) {
	switch colName {
	case "created_unix":
		m.Created = time.Unix(m.CreatedUnix, 0).Local()
	}
}
