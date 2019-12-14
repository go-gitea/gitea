package nodb

import (
	"sync"

	"github.com/lunny/nodb/store"
)

type batch struct {
	l *Nodb

	store.WriteBatch

	sync.Locker

	logs [][]byte

	tx *Tx
}

func (b *batch) Commit() error {
	b.l.commitLock.Lock()
	defer b.l.commitLock.Unlock()

	err := b.WriteBatch.Commit()

	if b.l.binlog != nil {
		if err == nil {
			if b.tx == nil {
				b.l.binlog.Log(b.logs...)
			} else {
				b.tx.logs = append(b.tx.logs, b.logs...)
			}
		}
		b.logs = [][]byte{}
	}

	return err
}

func (b *batch) Lock() {
	b.Locker.Lock()
}

func (b *batch) Unlock() {
	if b.l.binlog != nil {
		b.logs = [][]byte{}
	}
	b.WriteBatch.Rollback()
	b.Locker.Unlock()
}

func (b *batch) Put(key []byte, value []byte) {
	if b.l.binlog != nil {
		buf := encodeBinLogPut(key, value)
		b.logs = append(b.logs, buf)
	}
	b.WriteBatch.Put(key, value)
}

func (b *batch) Delete(key []byte) {
	if b.l.binlog != nil {
		buf := encodeBinLogDelete(key)
		b.logs = append(b.logs, buf)
	}
	b.WriteBatch.Delete(key)
}

type dbBatchLocker struct {
	l      *sync.Mutex
	wrLock *sync.RWMutex
}

func (l *dbBatchLocker) Lock() {
	l.wrLock.RLock()
	l.l.Lock()
}

func (l *dbBatchLocker) Unlock() {
	l.l.Unlock()
	l.wrLock.RUnlock()
}

type txBatchLocker struct {
}

func (l *txBatchLocker) Lock()   {}
func (l *txBatchLocker) Unlock() {}

type multiBatchLocker struct {
}

func (l *multiBatchLocker) Lock()   {}
func (l *multiBatchLocker) Unlock() {}

func (l *Nodb) newBatch(wb store.WriteBatch, locker sync.Locker, tx *Tx) *batch {
	b := new(batch)
	b.l = l
	b.WriteBatch = wb

	b.tx = tx
	b.Locker = locker

	b.logs = [][]byte{}
	return b
}
