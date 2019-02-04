// +build !windows

package mdb

import (
	"github.com/siddontang/ledisdb/store/driver"
	mdb "github.com/szferi/gomdb"
)

type Snapshot struct {
	db mdb.DBI
	tx *mdb.Txn
}

func newSnapshot(db MDB) (*Snapshot, error) {
	tx, err := db.env.BeginTxn(nil, mdb.RDONLY)
	if err != nil {
		return nil, err
	}

	return &Snapshot{db.db, tx}, nil
}

func (s *Snapshot) Get(key []byte) ([]byte, error) {
	v, err := s.tx.Get(s.db, key)
	if err == mdb.NotFound {
		return nil, nil
	}
	return v, err
}

func (s *Snapshot) NewIterator() driver.IIterator {
	c, err := s.tx.CursorOpen(s.db)
	if err != nil {
		return &MDBIterator{nil, nil, nil, nil, false, err, false}
	}

	return &MDBIterator{nil, nil, c, s.tx, true, nil, false}
}

func (s *Snapshot) Close() {
	s.tx.Commit()
}
