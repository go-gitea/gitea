// +build !windows

package mdb

import (
	"github.com/siddontang/ledisdb/config"
	"github.com/siddontang/ledisdb/store/driver"
	mdb "github.com/szferi/gomdb"
	"os"
)

type Store struct {
}

func (s Store) String() string {
	return DBName
}

type MDB struct {
	env  *mdb.Env
	db   mdb.DBI
	path string
	cfg  *config.Config
}

func (s Store) Open(path string, c *config.Config) (driver.IDB, error) {
	mapSize := c.LMDB.MapSize
	noSync := c.LMDB.NoSync

	if mapSize <= 0 {
		mapSize = 500 * 1024 * 1024
	}

	env, err := mdb.NewEnv()
	if err != nil {
		return MDB{}, err
	}

	// TODO: max dbs should be configurable
	if err := env.SetMaxDBs(1); err != nil {
		return MDB{}, err
	}

	if err := env.SetMapSize(uint64(mapSize)); err != nil {
		return MDB{}, err
	}

	if _, err := os.Stat(path); err != nil {
		err = os.MkdirAll(path, 0755)
		if err != nil {
			return MDB{}, err
		}
	}

	var flags uint = mdb.CREATE
	if noSync {
		flags |= mdb.NOSYNC | mdb.NOMETASYNC | mdb.WRITEMAP | mdb.MAPASYNC
	}

	err = env.Open(path, flags, 0755)
	if err != nil {
		return MDB{}, err
	}

	tx, err := env.BeginTxn(nil, 0)
	if err != nil {
		return MDB{}, err
	}

	dbi, err := tx.DBIOpen(nil, mdb.CREATE)
	if err != nil {
		return MDB{}, err
	}

	if err := tx.Commit(); err != nil {
		return MDB{}, err
	}

	db := MDB{
		env:  env,
		db:   dbi,
		path: path,
	}

	return db, nil
}

func (s Store) Repair(path string, c *config.Config) error {
	println("llmd not supports repair")
	return nil
}

func (db MDB) Put(key, value []byte) error {
	itr := db.iterator(false)
	defer itr.Close()

	itr.err = itr.c.Put(key, value, 0)
	itr.setState()
	return itr.err
}

func (db MDB) BatchPut(writes []driver.Write) error {
	itr := db.iterator(false)
	defer itr.Close()

	for _, w := range writes {
		if w.Value == nil {
			itr.key, itr.value, itr.err = itr.c.Get(w.Key, nil, mdb.SET)
			if itr.err == nil {
				itr.err = itr.c.Del(0)
			}
		} else {
			itr.err = itr.c.Put(w.Key, w.Value, 0)
		}

		if itr.err != nil && itr.err != mdb.NotFound {
			break
		}
	}
	itr.setState()

	return itr.err
}

func (db MDB) SyncBatchPut(writes []driver.Write) error {
	if err := db.BatchPut(writes); err != nil {
		return err
	}

	return db.env.Sync(1)
}

func (db MDB) Get(key []byte) ([]byte, error) {
	tx, err := db.env.BeginTxn(nil, mdb.RDONLY)
	if err != nil {
		return nil, err
	}
	defer tx.Commit()

	v, err := tx.Get(db.db, key)
	if err == mdb.NotFound {
		return nil, nil
	}
	return v, err
}

func (db MDB) Delete(key []byte) error {
	itr := db.iterator(false)
	defer itr.Close()

	itr.key, itr.value, itr.err = itr.c.Get(key, nil, mdb.SET)
	if itr.err == nil {
		itr.err = itr.c.Del(0)
	}
	itr.setState()
	return itr.Error()
}

func (db MDB) SyncPut(key []byte, value []byte) error {
	if err := db.Put(key, value); err != nil {
		return err
	}

	return db.env.Sync(1)
}

func (db MDB) SyncDelete(key []byte) error {
	if err := db.Delete(key); err != nil {
		return err
	}

	return db.env.Sync(1)
}

type MDBIterator struct {
	key   []byte
	value []byte
	c     *mdb.Cursor
	tx    *mdb.Txn
	valid bool
	err   error

	closeAutoCommit bool
}

func (itr *MDBIterator) Key() []byte {
	return itr.key
}

func (itr *MDBIterator) Value() []byte {
	return itr.value
}

func (itr *MDBIterator) Valid() bool {
	return itr.valid
}

func (itr *MDBIterator) Error() error {
	return itr.err
}

func (itr *MDBIterator) getCurrent() {
	itr.key, itr.value, itr.err = itr.c.Get(nil, nil, mdb.GET_CURRENT)
	itr.setState()
}

func (itr *MDBIterator) Seek(key []byte) {
	itr.key, itr.value, itr.err = itr.c.Get(key, nil, mdb.SET_RANGE)
	itr.setState()
}
func (itr *MDBIterator) Next() {
	itr.key, itr.value, itr.err = itr.c.Get(nil, nil, mdb.NEXT)
	itr.setState()
}

func (itr *MDBIterator) Prev() {
	itr.key, itr.value, itr.err = itr.c.Get(nil, nil, mdb.PREV)
	itr.setState()
}

func (itr *MDBIterator) First() {
	itr.key, itr.value, itr.err = itr.c.Get(nil, nil, mdb.FIRST)
	itr.setState()
}

func (itr *MDBIterator) Last() {
	itr.key, itr.value, itr.err = itr.c.Get(nil, nil, mdb.LAST)
	itr.setState()
}

func (itr *MDBIterator) setState() {
	if itr.err != nil {
		if itr.err == mdb.NotFound {
			itr.err = nil
		}
		itr.valid = false
	} else {
		itr.valid = true
	}
}

func (itr *MDBIterator) Close() error {
	if err := itr.c.Close(); err != nil {
		itr.tx.Abort()
		return err
	}

	if !itr.closeAutoCommit {
		return itr.err
	}

	if itr.err != nil {
		itr.tx.Abort()
		return itr.err
	}
	return itr.tx.Commit()
}

func (_ MDB) Name() string {
	return "lmdb"
}

func (db MDB) Path() string {
	return db.path
}

func (db MDB) iterator(rdonly bool) *MDBIterator {
	flags := uint(0)
	if rdonly {
		flags = mdb.RDONLY
	}
	tx, err := db.env.BeginTxn(nil, flags)
	if err != nil {
		return &MDBIterator{nil, nil, nil, nil, false, err, true}
	}

	c, err := tx.CursorOpen(db.db)
	if err != nil {
		tx.Abort()
		return &MDBIterator{nil, nil, nil, nil, false, err, true}
	}

	return &MDBIterator{nil, nil, c, tx, true, nil, true}
}

func (db MDB) Close() error {
	db.env.DBIClose(db.db)
	if err := db.env.Close(); err != nil {
		panic(err)
	}
	return nil
}

func (db MDB) NewIterator() driver.IIterator {
	return db.iterator(true)
}

func (db MDB) NewWriteBatch() driver.IWriteBatch {
	return driver.NewWriteBatch(db)
}

func (db MDB) Begin() (driver.Tx, error) {
	return newTx(db)
}

func (db MDB) NewSnapshot() (driver.ISnapshot, error) {
	return newSnapshot(db)
}

func (db MDB) Compact() error {
	return nil
}

func init() {
	driver.Register(Store{})
}
