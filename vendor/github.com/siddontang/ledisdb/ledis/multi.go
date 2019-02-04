package ledis

import (
	"errors"
	"fmt"
)

var (
	ErrNestMulti = errors.New("nest multi not supported")
	ErrMultiDone = errors.New("multi has been closed")
)

type Multi struct {
	*DB
}

func (db *DB) IsInMulti() bool {
	return db.status == DBInMulti
}

// begin a mutli to execute commands,
// it will block any other write operations before you close the multi, unlike transaction, mutli can not rollback
func (db *DB) Multi() (*Multi, error) {
	if db.IsInMulti() {
		return nil, ErrNestMulti
	}

	m := new(Multi)

	m.DB = new(DB)
	m.DB.status = DBInMulti

	m.DB.l = db.l

	m.l.wLock.Lock()

	m.DB.sdb = db.sdb

	m.DB.bucket = db.sdb

	m.DB.index = db.index

	m.DB.kvBatch = m.newBatch()
	m.DB.listBatch = m.newBatch()
	m.DB.hashBatch = m.newBatch()
	m.DB.zsetBatch = m.newBatch()
	m.DB.binBatch = m.newBatch()
	m.DB.setBatch = m.newBatch()

	m.DB.lbkeys = db.lbkeys

	return m, nil
}

func (m *Multi) newBatch() *batch {
	return m.l.newBatch(m.bucket.NewWriteBatch(), &multiBatchLocker{}, nil)
}

func (m *Multi) Close() error {
	if m.bucket == nil {
		return ErrMultiDone
	}
	m.l.wLock.Unlock()
	m.bucket = nil
	return nil
}

func (m *Multi) Select(index int) error {
	if index < 0 || index >= int(MaxDBNumber) {
		return fmt.Errorf("invalid db index %d", index)
	}

	m.DB.index = uint8(index)
	return nil
}
