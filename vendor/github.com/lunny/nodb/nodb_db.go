package nodb

import (
	"fmt"
	"sync"

	"github.com/lunny/nodb/store"
)

type ibucket interface {
	Get(key []byte) ([]byte, error)

	Put(key []byte, value []byte) error
	Delete(key []byte) error

	NewIterator() *store.Iterator

	NewWriteBatch() store.WriteBatch

	RangeIterator(min []byte, max []byte, rangeType uint8) *store.RangeLimitIterator
	RevRangeIterator(min []byte, max []byte, rangeType uint8) *store.RangeLimitIterator
	RangeLimitIterator(min []byte, max []byte, rangeType uint8, offset int, count int) *store.RangeLimitIterator
	RevRangeLimitIterator(min []byte, max []byte, rangeType uint8, offset int, count int) *store.RangeLimitIterator
}

type DB struct {
	l *Nodb

	sdb *store.DB

	bucket ibucket

	index uint8

	kvBatch   *batch
	listBatch *batch
	hashBatch *batch
	zsetBatch *batch
	binBatch  *batch
	setBatch  *batch

	status uint8
}

func (l *Nodb) newDB(index uint8) *DB {
	d := new(DB)

	d.l = l

	d.sdb = l.ldb

	d.bucket = d.sdb

	d.status = DBAutoCommit
	d.index = index

	d.kvBatch = d.newBatch()
	d.listBatch = d.newBatch()
	d.hashBatch = d.newBatch()
	d.zsetBatch = d.newBatch()
	d.binBatch = d.newBatch()
	d.setBatch = d.newBatch()

	return d
}

func (db *DB) newBatch() *batch {
	return db.l.newBatch(db.bucket.NewWriteBatch(), &dbBatchLocker{l: &sync.Mutex{}, wrLock: &db.l.wLock}, nil)
}

func (db *DB) Index() int {
	return int(db.index)
}

func (db *DB) IsAutoCommit() bool {
	return db.status == DBAutoCommit
}

func (db *DB) FlushAll() (drop int64, err error) {
	all := [...](func() (int64, error)){
		db.flush,
		db.lFlush,
		db.hFlush,
		db.zFlush,
		db.bFlush,
		db.sFlush}

	for _, flush := range all {
		if n, e := flush(); e != nil {
			err = e
			return
		} else {
			drop += n
		}
	}

	return
}

func (db *DB) newEliminator() *elimination {
	eliminator := newEliminator(db)

	eliminator.regRetireContext(KVType, db.kvBatch, db.delete)
	eliminator.regRetireContext(ListType, db.listBatch, db.lDelete)
	eliminator.regRetireContext(HashType, db.hashBatch, db.hDelete)
	eliminator.regRetireContext(ZSetType, db.zsetBatch, db.zDelete)
	eliminator.regRetireContext(BitType, db.binBatch, db.bDelete)
	eliminator.regRetireContext(SetType, db.setBatch, db.sDelete)

	return eliminator
}

func (db *DB) flushRegion(t *batch, minKey []byte, maxKey []byte) (drop int64, err error) {
	it := db.bucket.RangeIterator(minKey, maxKey, store.RangeROpen)
	for ; it.Valid(); it.Next() {
		t.Delete(it.RawKey())
		drop++
		if drop&1023 == 0 {
			if err = t.Commit(); err != nil {
				return
			}
		}
	}
	it.Close()
	return
}

func (db *DB) flushType(t *batch, dataType byte) (drop int64, err error) {
	var deleteFunc func(t *batch, key []byte) int64
	var metaDataType byte
	switch dataType {
	case KVType:
		deleteFunc = db.delete
		metaDataType = KVType
	case ListType:
		deleteFunc = db.lDelete
		metaDataType = LMetaType
	case HashType:
		deleteFunc = db.hDelete
		metaDataType = HSizeType
	case ZSetType:
		deleteFunc = db.zDelete
		metaDataType = ZSizeType
	case BitType:
		deleteFunc = db.bDelete
		metaDataType = BitMetaType
	case SetType:
		deleteFunc = db.sDelete
		metaDataType = SSizeType
	default:
		return 0, fmt.Errorf("invalid data type: %s", TypeName[dataType])
	}

	var keys [][]byte
	keys, err = db.scan(metaDataType, nil, 1024, false, "")
	for len(keys) != 0 || err != nil {
		for _, key := range keys {
			deleteFunc(t, key)
			db.rmExpire(t, dataType, key)

		}

		if err = t.Commit(); err != nil {
			return
		} else {
			drop += int64(len(keys))
		}
		keys, err = db.scan(metaDataType, nil, 1024, false, "")
	}
	return
}
