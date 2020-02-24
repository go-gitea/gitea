package store

import (
	"github.com/lunny/nodb/store/driver"
)

type Tx struct {
	driver.Tx
}

func (tx *Tx) NewIterator() *Iterator {
	it := new(Iterator)
	it.it = tx.Tx.NewIterator()

	return it
}

func (tx *Tx) NewWriteBatch() WriteBatch {
	return tx.Tx.NewWriteBatch()
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
