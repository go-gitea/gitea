package goleveldb

import (
	"github.com/syndtr/goleveldb/leveldb"
)

type WriteBatch struct {
	db     *DB
	wbatch *leveldb.Batch
}

func (w *WriteBatch) Put(key, value []byte) {
	w.wbatch.Put(key, value)
}

func (w *WriteBatch) Delete(key []byte) {
	w.wbatch.Delete(key)
}

func (w *WriteBatch) Commit() error {
	return w.db.db.Write(w.wbatch, nil)
}

func (w *WriteBatch) Rollback() error {
	w.wbatch.Reset()
	return nil
}
