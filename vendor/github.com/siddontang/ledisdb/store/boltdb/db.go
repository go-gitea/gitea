package boltdb

import (
	"github.com/boltdb/bolt"
	"github.com/siddontang/ledisdb/config"
	"github.com/siddontang/ledisdb/store/driver"
	"os"
	"path"
)

var bucketName = []byte("ledisdb")

type Store struct {
}

func (s Store) String() string {
	return DBName
}

func (s Store) Open(dbPath string, cfg *config.Config) (driver.IDB, error) {
	os.MkdirAll(dbPath, 0755)
	name := path.Join(dbPath, "ledis_bolt.db")
	db := new(DB)
	var err error

	db.path = name
	db.cfg = cfg

	db.db, err = bolt.Open(name, 0600, nil)
	if err != nil {
		return nil, err
	}

	var tx *bolt.Tx
	tx, err = db.db.Begin(true)
	if err != nil {
		return nil, err
	}

	_, err = tx.CreateBucketIfNotExists(bucketName)
	if err != nil {
		tx.Rollback()
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return db, nil
}

func (s Store) Repair(path string, cfg *config.Config) error {
	return nil
}

type DB struct {
	cfg  *config.Config
	db   *bolt.DB
	path string
}

func (db *DB) Close() error {
	return db.db.Close()
}

func (db *DB) Get(key []byte) ([]byte, error) {
	var value []byte

	t, err := db.db.Begin(false)
	if err != nil {
		return nil, err
	}
	defer t.Rollback()

	b := t.Bucket(bucketName)

	value = b.Get(key)

	if value == nil {
		return nil, nil
	} else {
		return append([]byte{}, value...), nil
	}
}

func (db *DB) Put(key []byte, value []byte) error {
	err := db.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		return b.Put(key, value)
	})
	return err
}

func (db *DB) Delete(key []byte) error {
	err := db.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		return b.Delete(key)
	})
	return err
}

func (db *DB) SyncPut(key []byte, value []byte) error {
	return db.Put(key, value)
}

func (db *DB) SyncDelete(key []byte) error {
	return db.Delete(key)
}

func (db *DB) NewIterator() driver.IIterator {
	tx, err := db.db.Begin(false)
	if err != nil {
		return &Iterator{}
	}
	b := tx.Bucket(bucketName)

	return &Iterator{
		tx: tx,
		it: b.Cursor()}
}

func (db *DB) NewWriteBatch() driver.IWriteBatch {
	return driver.NewWriteBatch(db)
}

func (db *DB) Begin() (driver.Tx, error) {
	tx, err := db.db.Begin(true)
	if err != nil {
		return nil, err
	}

	return &Tx{
		tx: tx,
		b:  tx.Bucket(bucketName),
	}, nil
}

func (db *DB) NewSnapshot() (driver.ISnapshot, error) {
	return newSnapshot(db)
}

func (db *DB) BatchPut(writes []driver.Write) error {
	err := db.db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket(bucketName)
		var err error
		for _, w := range writes {
			if w.Value == nil {
				err = b.Delete(w.Key)
			} else {
				err = b.Put(w.Key, w.Value)
			}
			if err != nil {
				return err
			}
		}
		return nil
	})
	return err
}

func (db *DB) SyncBatchPut(writes []driver.Write) error {
	return db.BatchPut(writes)
}

func (db *DB) Compact() error {
	return nil
}

func init() {
	driver.Register(Store{})
}
