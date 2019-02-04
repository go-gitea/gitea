package boltdb

import (
	"github.com/boltdb/bolt"
	"github.com/siddontang/ledisdb/store/driver"
)

type Tx struct {
	tx *bolt.Tx
	b  *bolt.Bucket
}

func (t *Tx) Get(key []byte) ([]byte, error) {
	return t.b.Get(key), nil
}

func (t *Tx) Put(key []byte, value []byte) error {
	return t.b.Put(key, value)
}

func (t *Tx) Delete(key []byte) error {
	return t.b.Delete(key)
}

func (t *Tx) NewIterator() driver.IIterator {
	return &Iterator{
		tx: nil,
		it: t.b.Cursor(),
	}
}

func (t *Tx) NewWriteBatch() driver.IWriteBatch {
	return driver.NewWriteBatch(t)
}

func (t *Tx) BatchPut(writes []driver.Write) error {
	var err error
	for _, w := range writes {
		if w.Value == nil {
			err = t.b.Delete(w.Key)
		} else {
			err = t.b.Put(w.Key, w.Value)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (t *Tx) SyncBatchPut(writes []driver.Write) error {
	return t.BatchPut(writes)
}

func (t *Tx) Rollback() error {
	return t.tx.Rollback()
}

func (t *Tx) Commit() error {
	return t.tx.Commit()
}
