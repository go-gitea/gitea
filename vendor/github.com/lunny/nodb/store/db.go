package store

import (
	"github.com/lunny/nodb/store/driver"
)

type DB struct {
	driver.IDB
}

func (db *DB) NewIterator() *Iterator {
	it := new(Iterator)
	it.it = db.IDB.NewIterator()

	return it
}

func (db *DB) NewWriteBatch() WriteBatch {
	return db.IDB.NewWriteBatch()
}

func (db *DB) NewSnapshot() (*Snapshot, error) {
	var err error
	s := &Snapshot{}
	if s.ISnapshot, err = db.IDB.NewSnapshot(); err != nil {
		return nil, err
	}

	return s, nil
}

func (db *DB) RangeIterator(min []byte, max []byte, rangeType uint8) *RangeLimitIterator {
	return NewRangeLimitIterator(db.NewIterator(), &Range{min, max, rangeType}, &Limit{0, -1})
}

func (db *DB) RevRangeIterator(min []byte, max []byte, rangeType uint8) *RangeLimitIterator {
	return NewRevRangeLimitIterator(db.NewIterator(), &Range{min, max, rangeType}, &Limit{0, -1})
}

//count < 0, unlimit.
//
//offset must >= 0, if < 0, will get nothing.
func (db *DB) RangeLimitIterator(min []byte, max []byte, rangeType uint8, offset int, count int) *RangeLimitIterator {
	return NewRangeLimitIterator(db.NewIterator(), &Range{min, max, rangeType}, &Limit{offset, count})
}

//count < 0, unlimit.
//
//offset must >= 0, if < 0, will get nothing.
func (db *DB) RevRangeLimitIterator(min []byte, max []byte, rangeType uint8, offset int, count int) *RangeLimitIterator {
	return NewRevRangeLimitIterator(db.NewIterator(), &Range{min, max, rangeType}, &Limit{offset, count})
}

func (db *DB) Begin() (*Tx, error) {
	tx, err := db.IDB.Begin()
	if err != nil {
		return nil, err
	}

	return &Tx{tx}, nil
}
