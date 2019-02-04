package ledis

import (
	"errors"
	"fmt"
	"github.com/siddontang/ledisdb/store"
)

var (
	ErrNestTx = errors.New("nest transaction not supported")
	ErrTxDone = errors.New("Transaction has already been committed or rolled back")
)

type Tx struct {
	*DB

	tx *store.Tx

	data *store.BatchData
}

func (db *DB) IsTransaction() bool {
	return db.status == DBInTransaction
}

// Begin a transaction, it will block all other write operations before calling Commit or Rollback.
// You must be very careful to prevent long-time transaction.
func (db *DB) Begin() (*Tx, error) {
	if db.IsTransaction() {
		return nil, ErrNestTx
	}

	tx := new(Tx)

	tx.data = &store.BatchData{}

	tx.DB = new(DB)
	tx.DB.l = db.l

	tx.l.wLock.Lock()

	tx.DB.sdb = db.sdb

	var err error
	tx.tx, err = db.sdb.Begin()
	if err != nil {
		tx.l.wLock.Unlock()
		return nil, err
	}

	tx.DB.bucket = tx.tx

	tx.DB.status = DBInTransaction

	tx.DB.index = db.index

	tx.DB.kvBatch = tx.newBatch()
	tx.DB.listBatch = tx.newBatch()
	tx.DB.hashBatch = tx.newBatch()
	tx.DB.zsetBatch = tx.newBatch()
	tx.DB.binBatch = tx.newBatch()
	tx.DB.setBatch = tx.newBatch()

	tx.DB.lbkeys = db.lbkeys

	return tx, nil
}

func (tx *Tx) Commit() error {
	if tx.tx == nil {
		return ErrTxDone
	}

	err := tx.l.handleCommit(tx.data, tx.tx)
	tx.data.Reset()

	tx.tx = nil

	tx.l.wLock.Unlock()

	tx.DB.bucket = nil

	return err
}

func (tx *Tx) Rollback() error {
	if tx.tx == nil {
		return ErrTxDone
	}

	err := tx.tx.Rollback()
	tx.data.Reset()
	tx.tx = nil

	tx.l.wLock.Unlock()
	tx.DB.bucket = nil

	return err
}

func (tx *Tx) newBatch() *batch {
	return tx.l.newBatch(tx.tx.NewWriteBatch(), &txBatchLocker{}, tx)
}

func (tx *Tx) Select(index int) error {
	if index < 0 || index >= int(MaxDBNumber) {
		return fmt.Errorf("invalid db index %d", index)
	}

	tx.DB.index = uint8(index)
	return nil
}
