package goleveldb

import (
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/cache"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"github.com/syndtr/goleveldb/leveldb/storage"

	"github.com/lunny/nodb/config"
	"github.com/lunny/nodb/store/driver"

	"os"
)

const defaultFilterBits int = 10

type Store struct {
}

func (s Store) String() string {
	return DBName
}

type MemStore struct {
}

func (s MemStore) String() string {
	return MemDBName
}

type DB struct {
	path string

	cfg *config.LevelDBConfig

	db *leveldb.DB

	opts *opt.Options

	iteratorOpts *opt.ReadOptions

	cache cache.Cache

	filter filter.Filter
}

func (s Store) Open(path string, cfg *config.Config) (driver.IDB, error) {
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		return nil, err
	}

	db := new(DB)
	db.path = path
	db.cfg = &cfg.LevelDB

	db.initOpts()

	var err error
	db.db, err = leveldb.OpenFile(db.path, db.opts)

	if err != nil {
		return nil, err
	}

	return db, nil
}

func (s Store) Repair(path string, cfg *config.Config) error {
	db, err := leveldb.RecoverFile(path, newOptions(&cfg.LevelDB))
	if err != nil {
		return err
	}

	db.Close()
	return nil
}

func (s MemStore) Open(path string, cfg *config.Config) (driver.IDB, error) {
	db := new(DB)
	db.path = path
	db.cfg = &cfg.LevelDB

	db.initOpts()

	var err error
	db.db, err = leveldb.Open(storage.NewMemStorage(), db.opts)
	if err != nil {
		return nil, err
	}

	return db, nil
}

func (s MemStore) Repair(path string, cfg *config.Config) error {
	return nil
}

func (db *DB) initOpts() {
	db.opts = newOptions(db.cfg)

	db.iteratorOpts = &opt.ReadOptions{}
	db.iteratorOpts.DontFillCache = true
}

func newOptions(cfg *config.LevelDBConfig) *opt.Options {
	opts := &opt.Options{}
	opts.ErrorIfMissing = false

	cfg.Adjust()

	//opts.BlockCacher = cache.NewLRU(cfg.CacheSize)
	opts.BlockCacheCapacity = cfg.CacheSize

	//we must use bloomfilter
	opts.Filter = filter.NewBloomFilter(defaultFilterBits)

	if !cfg.Compression {
		opts.Compression = opt.NoCompression
	} else {
		opts.Compression = opt.SnappyCompression
	}

	opts.BlockSize = cfg.BlockSize
	opts.WriteBuffer = cfg.WriteBufferSize

	return opts
}

func (db *DB) Close() error {
	return db.db.Close()
}

func (db *DB) Put(key, value []byte) error {
	return db.db.Put(key, value, nil)
}

func (db *DB) Get(key []byte) ([]byte, error) {
	v, err := db.db.Get(key, nil)
	if err == leveldb.ErrNotFound {
		return nil, nil
	}
	return v, nil
}

func (db *DB) Delete(key []byte) error {
	return db.db.Delete(key, nil)
}

func (db *DB) NewWriteBatch() driver.IWriteBatch {
	wb := &WriteBatch{
		db:     db,
		wbatch: new(leveldb.Batch),
	}
	return wb
}

func (db *DB) NewIterator() driver.IIterator {
	it := &Iterator{
		db.db.NewIterator(nil, db.iteratorOpts),
	}

	return it
}

func (db *DB) Begin() (driver.Tx, error) {
	return nil, driver.ErrTxSupport
}

func (db *DB) NewSnapshot() (driver.ISnapshot, error) {
	snapshot, err := db.db.GetSnapshot()
	if err != nil {
		return nil, err
	}

	s := &Snapshot{
		db:  db,
		snp: snapshot,
	}

	return s, nil
}

func init() {
	driver.Register(Store{})
	driver.Register(MemStore{})
}
