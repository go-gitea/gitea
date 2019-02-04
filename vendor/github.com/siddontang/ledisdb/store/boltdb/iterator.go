package boltdb

import (
	"github.com/boltdb/bolt"
)

type Iterator struct {
	tx    *bolt.Tx
	it    *bolt.Cursor
	key   []byte
	value []byte
}

func (it *Iterator) Close() error {
	if it.tx != nil {
		return it.tx.Rollback()
	} else {
		return nil
	}
}

func (it *Iterator) First() {
	it.key, it.value = it.it.First()
}

func (it *Iterator) Last() {
	it.key, it.value = it.it.Last()
}

func (it *Iterator) Seek(key []byte) {
	it.key, it.value = it.it.Seek(key)
}

func (it *Iterator) Next() {
	it.key, it.value = it.it.Next()
}
func (it *Iterator) Prev() {
	it.key, it.value = it.it.Prev()
}

func (it *Iterator) Valid() bool {
	return !(it.key == nil && it.value == nil)
}

func (it *Iterator) Key() []byte {
	return it.key
}
func (it *Iterator) Value() []byte {
	return it.value
}
