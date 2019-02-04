// +build !windows

package mdb

import (
	"github.com/siddontang/ledisdb/store/driver"
	mdb "github.com/szferi/gomdb"
)

type Tx struct {
	db mdb.DBI
	tx *mdb.Txn
}

func newTx(db MDB) (*Tx, error) {
	tx, err := db.env.BeginTxn(nil, uint(0))
	if err != nil {
		return nil, err
	}

	return &Tx{db.db, tx}, nil
}

func (t *Tx) Get(key []byte) ([]byte, error) {
	v, err := t.tx.Get(t.db, key)
	if err == mdb.NotFound {
		return nil, nil
	}
	return v, err
}

func (t *Tx) Put(key []byte, value []byte) error {
	return t.tx.Put(t.db, key, value, mdb.NODUPDATA)
}

func (t *Tx) Delete(key []byte) error {
	return t.tx.Del(t.db, key, nil)
}

func (t *Tx) NewIterator() driver.IIterator {
	return t.newIterator()
}

func (t *Tx) newIterator() *MDBIterator {
	c, err := t.tx.CursorOpen(t.db)
	if err != nil {
		return &MDBIterator{nil, nil, nil, nil, false, err, false}
	}

	return &MDBIterator{nil, nil, c, t.tx, true, nil, false}
}

func (t *Tx) NewWriteBatch() driver.IWriteBatch {
	return driver.NewWriteBatch(t)
}

func (t *Tx) BatchPut(writes []driver.Write) error {
	itr := t.newIterator()

	for _, w := range writes {
		if w.Value == nil {
			itr.key, itr.value, itr.err = itr.c.Get(w.Key, nil, mdb.SET)
			if itr.err == nil {
				itr.err = itr.c.Del(0)
			}
		} else {
			itr.err = itr.c.Put(w.Key, w.Value, 0)
		}

		if itr.err != nil && itr.err != mdb.NotFound {
			break
		}
	}
	itr.setState()

	return itr.Close()
}

func (t *Tx) SyncBatchPut(writes []driver.Write) error {
	return t.BatchPut(writes)
}

func (t *Tx) Rollback() error {
	t.tx.Abort()
	return nil
}

func (t *Tx) Commit() error {
	return t.tx.Commit()
}
