package nodb

import (
	"encoding/binary"
	"errors"
	"time"

	"github.com/lunny/nodb/store"
)

const (
	listHeadSeq int32 = 1
	listTailSeq int32 = 2

	listMinSeq     int32 = 1000
	listMaxSeq     int32 = 1<<31 - 1000
	listInitialSeq int32 = listMinSeq + (listMaxSeq-listMinSeq)/2
)

var errLMetaKey = errors.New("invalid lmeta key")
var errListKey = errors.New("invalid list key")
var errListSeq = errors.New("invalid list sequence, overflow")

func (db *DB) lEncodeMetaKey(key []byte) []byte {
	buf := make([]byte, len(key)+2)
	buf[0] = db.index
	buf[1] = LMetaType

	copy(buf[2:], key)
	return buf
}

func (db *DB) lDecodeMetaKey(ek []byte) ([]byte, error) {
	if len(ek) < 2 || ek[0] != db.index || ek[1] != LMetaType {
		return nil, errLMetaKey
	}

	return ek[2:], nil
}

func (db *DB) lEncodeListKey(key []byte, seq int32) []byte {
	buf := make([]byte, len(key)+8)

	pos := 0
	buf[pos] = db.index
	pos++
	buf[pos] = ListType
	pos++

	binary.BigEndian.PutUint16(buf[pos:], uint16(len(key)))
	pos += 2

	copy(buf[pos:], key)
	pos += len(key)

	binary.BigEndian.PutUint32(buf[pos:], uint32(seq))

	return buf
}

func (db *DB) lDecodeListKey(ek []byte) (key []byte, seq int32, err error) {
	if len(ek) < 8 || ek[0] != db.index || ek[1] != ListType {
		err = errListKey
		return
	}

	keyLen := int(binary.BigEndian.Uint16(ek[2:]))
	if keyLen+8 != len(ek) {
		err = errListKey
		return
	}

	key = ek[4 : 4+keyLen]
	seq = int32(binary.BigEndian.Uint32(ek[4+keyLen:]))
	return
}

func (db *DB) lpush(key []byte, whereSeq int32, args ...[]byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	var headSeq int32
	var tailSeq int32
	var size int32
	var err error

	t := db.listBatch
	t.Lock()
	defer t.Unlock()

	metaKey := db.lEncodeMetaKey(key)
	headSeq, tailSeq, size, err = db.lGetMeta(nil, metaKey)
	if err != nil {
		return 0, err
	}

	var pushCnt int = len(args)
	if pushCnt == 0 {
		return int64(size), nil
	}

	var seq int32 = headSeq
	var delta int32 = -1
	if whereSeq == listTailSeq {
		seq = tailSeq
		delta = 1
	}

	//	append elements
	if size > 0 {
		seq += delta
	}

	for i := 0; i < pushCnt; i++ {
		ek := db.lEncodeListKey(key, seq+int32(i)*delta)
		t.Put(ek, args[i])
	}

	seq += int32(pushCnt-1) * delta
	if seq <= listMinSeq || seq >= listMaxSeq {
		return 0, errListSeq
	}

	//	set meta info
	if whereSeq == listHeadSeq {
		headSeq = seq
	} else {
		tailSeq = seq
	}

	db.lSetMeta(metaKey, headSeq, tailSeq)

	err = t.Commit()
	return int64(size) + int64(pushCnt), err
}

func (db *DB) lpop(key []byte, whereSeq int32) ([]byte, error) {
	if err := checkKeySize(key); err != nil {
		return nil, err
	}

	t := db.listBatch
	t.Lock()
	defer t.Unlock()

	var headSeq int32
	var tailSeq int32
	var err error

	metaKey := db.lEncodeMetaKey(key)
	headSeq, tailSeq, _, err = db.lGetMeta(nil, metaKey)
	if err != nil {
		return nil, err
	}

	var value []byte

	var seq int32 = headSeq
	if whereSeq == listTailSeq {
		seq = tailSeq
	}

	itemKey := db.lEncodeListKey(key, seq)
	value, err = db.bucket.Get(itemKey)
	if err != nil {
		return nil, err
	}

	if whereSeq == listHeadSeq {
		headSeq += 1
	} else {
		tailSeq -= 1
	}

	t.Delete(itemKey)
	size := db.lSetMeta(metaKey, headSeq, tailSeq)
	if size == 0 {
		db.rmExpire(t, HashType, key)
	}

	err = t.Commit()
	return value, err
}

//	ps : here just focus on deleting the list data,
//		 any other likes expire is ignore.
func (db *DB) lDelete(t *batch, key []byte) int64 {
	mk := db.lEncodeMetaKey(key)

	var headSeq int32
	var tailSeq int32
	var err error

	it := db.bucket.NewIterator()
	defer it.Close()

	headSeq, tailSeq, _, err = db.lGetMeta(it, mk)
	if err != nil {
		return 0
	}

	var num int64 = 0
	startKey := db.lEncodeListKey(key, headSeq)
	stopKey := db.lEncodeListKey(key, tailSeq)

	rit := store.NewRangeIterator(it, &store.Range{startKey, stopKey, store.RangeClose})
	for ; rit.Valid(); rit.Next() {
		t.Delete(rit.RawKey())
		num++
	}

	t.Delete(mk)

	return num
}

func (db *DB) lGetMeta(it *store.Iterator, ek []byte) (headSeq int32, tailSeq int32, size int32, err error) {
	var v []byte
	if it != nil {
		v = it.Find(ek)
	} else {
		v, err = db.bucket.Get(ek)
	}
	if err != nil {
		return
	} else if v == nil {
		headSeq = listInitialSeq
		tailSeq = listInitialSeq
		size = 0
		return
	} else {
		headSeq = int32(binary.LittleEndian.Uint32(v[0:4]))
		tailSeq = int32(binary.LittleEndian.Uint32(v[4:8]))
		size = tailSeq - headSeq + 1
	}
	return
}

func (db *DB) lSetMeta(ek []byte, headSeq int32, tailSeq int32) int32 {
	t := db.listBatch

	var size int32 = tailSeq - headSeq + 1
	if size < 0 {
		//	todo : log error + panic
	} else if size == 0 {
		t.Delete(ek)
	} else {
		buf := make([]byte, 8)

		binary.LittleEndian.PutUint32(buf[0:4], uint32(headSeq))
		binary.LittleEndian.PutUint32(buf[4:8], uint32(tailSeq))

		t.Put(ek, buf)
	}

	return size
}

func (db *DB) lExpireAt(key []byte, when int64) (int64, error) {
	t := db.listBatch
	t.Lock()
	defer t.Unlock()

	if llen, err := db.LLen(key); err != nil || llen == 0 {
		return 0, err
	} else {
		db.expireAt(t, ListType, key, when)
		if err := t.Commit(); err != nil {
			return 0, err
		}
	}
	return 1, nil
}

func (db *DB) LIndex(key []byte, index int32) ([]byte, error) {
	if err := checkKeySize(key); err != nil {
		return nil, err
	}

	var seq int32
	var headSeq int32
	var tailSeq int32
	var err error

	metaKey := db.lEncodeMetaKey(key)

	it := db.bucket.NewIterator()
	defer it.Close()

	headSeq, tailSeq, _, err = db.lGetMeta(it, metaKey)
	if err != nil {
		return nil, err
	}

	if index >= 0 {
		seq = headSeq + index
	} else {
		seq = tailSeq + index + 1
	}

	sk := db.lEncodeListKey(key, seq)
	v := it.Find(sk)

	return v, nil
}

func (db *DB) LLen(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	ek := db.lEncodeMetaKey(key)
	_, _, size, err := db.lGetMeta(nil, ek)
	return int64(size), err
}

func (db *DB) LPop(key []byte) ([]byte, error) {
	return db.lpop(key, listHeadSeq)
}

func (db *DB) LPush(key []byte, arg1 []byte, args ...[]byte) (int64, error) {
	var argss = [][]byte{arg1}
	argss = append(argss, args...)
	return db.lpush(key, listHeadSeq, argss...)
}

func (db *DB) LRange(key []byte, start int32, stop int32) ([][]byte, error) {
	if err := checkKeySize(key); err != nil {
		return nil, err
	}

	var headSeq int32
	var llen int32
	var err error

	metaKey := db.lEncodeMetaKey(key)

	it := db.bucket.NewIterator()
	defer it.Close()

	if headSeq, _, llen, err = db.lGetMeta(it, metaKey); err != nil {
		return nil, err
	}

	if start < 0 {
		start = llen + start
	}
	if stop < 0 {
		stop = llen + stop
	}
	if start < 0 {
		start = 0
	}

	if start > stop || start >= llen {
		return [][]byte{}, nil
	}

	if stop >= llen {
		stop = llen - 1
	}

	limit := (stop - start) + 1
	headSeq += start

	v := make([][]byte, 0, limit)

	startKey := db.lEncodeListKey(key, headSeq)
	rit := store.NewRangeLimitIterator(it,
		&store.Range{
			Min:  startKey,
			Max:  nil,
			Type: store.RangeClose},
		&store.Limit{
			Offset: 0,
			Count:  int(limit)})

	for ; rit.Valid(); rit.Next() {
		v = append(v, rit.Value())
	}

	return v, nil
}

func (db *DB) RPop(key []byte) ([]byte, error) {
	return db.lpop(key, listTailSeq)
}

func (db *DB) RPush(key []byte, arg1 []byte, args ...[]byte) (int64, error) {
	var argss = [][]byte{arg1}
	argss = append(argss, args...)
	return db.lpush(key, listTailSeq, argss...)
}

func (db *DB) LClear(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	t := db.listBatch
	t.Lock()
	defer t.Unlock()

	num := db.lDelete(t, key)
	db.rmExpire(t, ListType, key)

	err := t.Commit()
	return num, err
}

func (db *DB) LMclear(keys ...[]byte) (int64, error) {
	t := db.listBatch
	t.Lock()
	defer t.Unlock()

	for _, key := range keys {
		if err := checkKeySize(key); err != nil {
			return 0, err
		}

		db.lDelete(t, key)
		db.rmExpire(t, ListType, key)

	}

	err := t.Commit()
	return int64(len(keys)), err
}

func (db *DB) lFlush() (drop int64, err error) {
	t := db.listBatch
	t.Lock()
	defer t.Unlock()
	return db.flushType(t, ListType)
}

func (db *DB) LExpire(key []byte, duration int64) (int64, error) {
	if duration <= 0 {
		return 0, errExpireValue
	}

	return db.lExpireAt(key, time.Now().Unix()+duration)
}

func (db *DB) LExpireAt(key []byte, when int64) (int64, error) {
	if when <= time.Now().Unix() {
		return 0, errExpireValue
	}

	return db.lExpireAt(key, when)
}

func (db *DB) LTTL(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return -1, err
	}

	return db.ttl(ListType, key)
}

func (db *DB) LPersist(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	t := db.listBatch
	t.Lock()
	defer t.Unlock()

	n, err := db.rmExpire(t, ListType, key)
	if err != nil {
		return 0, err
	}

	err = t.Commit()
	return n, err
}

func (db *DB) LScan(key []byte, count int, inclusive bool, match string) ([][]byte, error) {
	return db.scan(LMetaType, key, count, inclusive, match)
}

func (db *DB) lEncodeMinKey() []byte {
	return db.lEncodeMetaKey(nil)
}

func (db *DB) lEncodeMaxKey() []byte {
	ek := db.lEncodeMetaKey(nil)
	ek[len(ek)-1] = LMetaType + 1
	return ek
}
