package store

import (
	"github.com/siddontang/ledisdb/store/driver"
)

type Tx struct {
	tx driver.Tx
	st *Stat
}

func (tx *Tx) NewIterator() *Iterator {
	it := new(Iterator)
	it.it = tx.tx.NewIterator()
	it.st = tx.st

	tx.st.IterNum.Add(1)

	return it
}

func (tx *Tx) NewWriteBatch() *WriteBatch {
	tx.st.BatchNum.Add(1)

	wb := new(WriteBatch)
	wb.wb = tx.tx.NewWriteBatch()
	wb.st = tx.st
	return wb
}

func (tx *Tx) RangeIterator(min []byte, max []byte, rangeType uint8) *RangeLimitIterator {
	return NewRangeLimitIterator(tx.NewIterator(), &Range{min, max, rangeType}, &Limit{0, -1})
}

func (tx *Tx) RevRangeIterator(min []byte, max []byte, rangeType uint8) *RangeLimitIterator {
	return NewRevRangeLimitIterator(tx.NewIterator(), &Range{min, max, rangeType}, &Limit{0, -1})
}

//count < 0, unlimit.
//
//offset must >= 0, if < 0, will get nothing.
func (tx *Tx) RangeLimitIterator(min []byte, max []byte, rangeType uint8, offset int, count int) *RangeLimitIterator {
	return NewRangeLimitIterator(tx.NewIterator(), &Range{min, max, rangeType}, &Limit{offset, count})
}

//count < 0, unlimit.
//
//offset must >= 0, if < 0, will get nothing.
func (tx *Tx) RevRangeLimitIterator(min []byte, max []byte, rangeType uint8, offset int, count int) *RangeLimitIterator {
	return NewRevRangeLimitIterator(tx.NewIterator(), &Range{min, max, rangeType}, &Limit{offset, count})
}

func (tx *Tx) Get(key []byte) ([]byte, error) {
	v, err := tx.tx.Get(key)
	tx.st.statGet(v, err)
	return v, err
}

func (tx *Tx) GetSlice(key []byte) (Slice, error) {
	if v, err := tx.Get(key); err != nil {
		return nil, err
	} else if v == nil {
		return nil, nil
	} else {
		return driver.GoSlice(v), nil
	}
}

func (tx *Tx) Put(key []byte, value []byte) error {
	tx.st.PutNum.Add(1)
	return tx.tx.Put(key, value)
}

func (tx *Tx) Delete(key []byte) error {
	tx.st.DeleteNum.Add(1)
	return tx.tx.Delete(key)
}

func (tx *Tx) Commit() error {
	tx.st.TxCommitNum.Add(1)
	return tx.tx.Commit()
}

func (tx *Tx) Rollback() error {
	return tx.tx.Rollback()
}
