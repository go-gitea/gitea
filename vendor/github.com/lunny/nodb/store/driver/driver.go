package driver

import (
	"errors"
)

var (
	ErrTxSupport = errors.New("transaction is not supported")
)

type IDB interface {
	Close() error

	Get(key []byte) ([]byte, error)

	Put(key []byte, value []byte) error
	Delete(key []byte) error

	NewIterator() IIterator

	NewWriteBatch() IWriteBatch

	NewSnapshot() (ISnapshot, error)

	Begin() (Tx, error)
}

type ISnapshot interface {
	Get(key []byte) ([]byte, error)
	NewIterator() IIterator
	Close()
}

type IIterator interface {
	Close() error

	First()
	Last()
	Seek(key []byte)

	Next()
	Prev()

	Valid() bool

	Key() []byte
	Value() []byte
}

type IWriteBatch interface {
	Put(key []byte, value []byte)
	Delete(key []byte)
	Commit() error
	Rollback() error
}

type Tx interface {
	Get(key []byte) ([]byte, error)
	Put(key []byte, value []byte) error
	Delete(key []byte) error

	NewIterator() IIterator
	NewWriteBatch() IWriteBatch

	Commit() error
	Rollback() error
}
