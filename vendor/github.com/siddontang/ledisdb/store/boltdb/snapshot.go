package boltdb

import (
	"github.com/boltdb/bolt"
	"github.com/siddontang/ledisdb/store/driver"
)

type Snapshot struct {
	tx *bolt.Tx
	b  *bolt.Bucket
}

func newSnapshot(db *DB) (*Snapshot, error) {
	tx, err := db.db.Begin(false)
	if err != nil {
		return nil, err
	}

	return &Snapshot{
		tx: tx,
		b:  tx.Bucket(bucketName)}, nil
}

func (s *Snapshot) Get(key []byte) ([]byte, error) {
	return s.b.Get(key), nil
}

func (s *Snapshot) NewIterator() driver.IIterator {
	return &Iterator{
		tx: nil,
		it: s.b.Cursor(),
	}
}

func (s *Snapshot) Close() {
	s.tx.Rollback()
}
