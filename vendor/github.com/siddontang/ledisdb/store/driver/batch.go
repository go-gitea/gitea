package driver

import (
	"github.com/syndtr/goleveldb/leveldb"
)

type BatchPuter interface {
	BatchPut([]Write) error
	SyncBatchPut([]Write) error
}

type Write struct {
	Key   []byte
	Value []byte
}

type WriteBatch struct {
	d *leveldb.Batch

	wb []Write
	w  BatchPuter
}

func (wb *WriteBatch) Close() {
	wb.d.Reset()
	wb.wb = wb.wb[0:0]
}

func (wb *WriteBatch) Put(key, value []byte) {
	if value == nil {
		value = []byte{}
	}
	wb.wb = append(wb.wb, Write{key, value})
}

func (wb *WriteBatch) Delete(key []byte) {
	wb.wb = append(wb.wb, Write{key, nil})
}

func (wb *WriteBatch) Commit() error {
	return wb.w.BatchPut(wb.wb)
}

func (wb *WriteBatch) SyncCommit() error {
	return wb.w.SyncBatchPut(wb.wb)
}

func (wb *WriteBatch) Rollback() error {
	wb.wb = wb.wb[0:0]
	return nil
}

func (wb *WriteBatch) Data() []byte {
	wb.d.Reset()
	for _, w := range wb.wb {
		if w.Value == nil {
			wb.d.Delete(w.Key)
		} else {
			wb.d.Put(w.Key, w.Value)
		}
	}
	return wb.d.Dump()
}

func NewWriteBatch(puter BatchPuter) *WriteBatch {
	return &WriteBatch{
		&leveldb.Batch{},
		[]Write{},
		puter}
}
