package nodb

import (
	"encoding/binary"
	"errors"
	"time"

	"github.com/lunny/nodb/store"
)

var (
	errExpMetaKey = errors.New("invalid expire meta key")
	errExpTimeKey = errors.New("invalid expire time key")
)

type retireCallback func(*batch, []byte) int64

type elimination struct {
	db         *DB
	exp2Tx     []*batch
	exp2Retire []retireCallback
}

var errExpType = errors.New("invalid expire type")

func (db *DB) expEncodeTimeKey(dataType byte, key []byte, when int64) []byte {
	buf := make([]byte, len(key)+11)

	buf[0] = db.index
	buf[1] = ExpTimeType
	buf[2] = dataType
	pos := 3

	binary.BigEndian.PutUint64(buf[pos:], uint64(when))
	pos += 8

	copy(buf[pos:], key)

	return buf
}

func (db *DB) expEncodeMetaKey(dataType byte, key []byte) []byte {
	buf := make([]byte, len(key)+3)

	buf[0] = db.index
	buf[1] = ExpMetaType
	buf[2] = dataType
	pos := 3

	copy(buf[pos:], key)

	return buf
}

func (db *DB) expDecodeMetaKey(mk []byte) (byte, []byte, error) {
	if len(mk) <= 3 || mk[0] != db.index || mk[1] != ExpMetaType {
		return 0, nil, errExpMetaKey
	}

	return mk[2], mk[3:], nil
}

func (db *DB) expDecodeTimeKey(tk []byte) (byte, []byte, int64, error) {
	if len(tk) < 11 || tk[0] != db.index || tk[1] != ExpTimeType {
		return 0, nil, 0, errExpTimeKey
	}

	return tk[2], tk[11:], int64(binary.BigEndian.Uint64(tk[3:])), nil
}

func (db *DB) expire(t *batch, dataType byte, key []byte, duration int64) {
	db.expireAt(t, dataType, key, time.Now().Unix()+duration)
}

func (db *DB) expireAt(t *batch, dataType byte, key []byte, when int64) {
	mk := db.expEncodeMetaKey(dataType, key)
	tk := db.expEncodeTimeKey(dataType, key, when)

	t.Put(tk, mk)
	t.Put(mk, PutInt64(when))
}

func (db *DB) ttl(dataType byte, key []byte) (t int64, err error) {
	mk := db.expEncodeMetaKey(dataType, key)

	if t, err = Int64(db.bucket.Get(mk)); err != nil || t == 0 {
		t = -1
	} else {
		t -= time.Now().Unix()
		if t <= 0 {
			t = -1
		}
		// if t == -1 : to remove ????
	}

	return t, err
}

func (db *DB) rmExpire(t *batch, dataType byte, key []byte) (int64, error) {
	mk := db.expEncodeMetaKey(dataType, key)
	if v, err := db.bucket.Get(mk); err != nil {
		return 0, err
	} else if v == nil {
		return 0, nil
	} else if when, err2 := Int64(v, nil); err2 != nil {
		return 0, err2
	} else {
		tk := db.expEncodeTimeKey(dataType, key, when)
		t.Delete(mk)
		t.Delete(tk)
		return 1, nil
	}
}

func (db *DB) expFlush(t *batch, dataType byte) (err error) {
	minKey := make([]byte, 3)
	minKey[0] = db.index
	minKey[1] = ExpTimeType
	minKey[2] = dataType

	maxKey := make([]byte, 3)
	maxKey[0] = db.index
	maxKey[1] = ExpMetaType
	maxKey[2] = dataType + 1

	_, err = db.flushRegion(t, minKey, maxKey)
	err = t.Commit()
	return
}

//////////////////////////////////////////////////////////
//
//////////////////////////////////////////////////////////

func newEliminator(db *DB) *elimination {
	eli := new(elimination)
	eli.db = db
	eli.exp2Tx = make([]*batch, maxDataType)
	eli.exp2Retire = make([]retireCallback, maxDataType)
	return eli
}

func (eli *elimination) regRetireContext(dataType byte, t *batch, onRetire retireCallback) {

	//	todo .. need to ensure exist - mapExpMetaType[expType]

	eli.exp2Tx[dataType] = t
	eli.exp2Retire[dataType] = onRetire
}

//	call by outside ... (from *db to another *db)
func (eli *elimination) active() {
	now := time.Now().Unix()
	db := eli.db
	dbGet := db.bucket.Get

	minKey := db.expEncodeTimeKey(NoneType, nil, 0)
	maxKey := db.expEncodeTimeKey(maxDataType, nil, now)

	it := db.bucket.RangeLimitIterator(minKey, maxKey, store.RangeROpen, 0, -1)
	for ; it.Valid(); it.Next() {
		tk := it.RawKey()
		mk := it.RawValue()

		dt, k, _, err := db.expDecodeTimeKey(tk)
		if err != nil {
			continue
		}

		t := eli.exp2Tx[dt]
		onRetire := eli.exp2Retire[dt]
		if tk == nil || onRetire == nil {
			continue
		}

		t.Lock()

		if exp, err := Int64(dbGet(mk)); err == nil {
			// check expire again
			if exp <= now {
				onRetire(t, k)
				t.Delete(tk)
				t.Delete(mk)

				t.Commit()
			}

		}

		t.Unlock()
	}
	it.Close()

	return
}
