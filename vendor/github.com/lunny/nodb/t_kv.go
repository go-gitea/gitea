package nodb

import (
	"errors"
	"time"
)

type KVPair struct {
	Key   []byte
	Value []byte
}

var errKVKey = errors.New("invalid encode kv key")

func checkKeySize(key []byte) error {
	if len(key) > MaxKeySize || len(key) == 0 {
		return errKeySize
	}
	return nil
}

func checkValueSize(value []byte) error {
	if len(value) > MaxValueSize {
		return errValueSize
	}

	return nil
}

func (db *DB) encodeKVKey(key []byte) []byte {
	ek := make([]byte, len(key)+2)
	ek[0] = db.index
	ek[1] = KVType
	copy(ek[2:], key)
	return ek
}

func (db *DB) decodeKVKey(ek []byte) ([]byte, error) {
	if len(ek) < 2 || ek[0] != db.index || ek[1] != KVType {
		return nil, errKVKey
	}

	return ek[2:], nil
}

func (db *DB) encodeKVMinKey() []byte {
	ek := db.encodeKVKey(nil)
	return ek
}

func (db *DB) encodeKVMaxKey() []byte {
	ek := db.encodeKVKey(nil)
	ek[len(ek)-1] = KVType + 1
	return ek
}

func (db *DB) incr(key []byte, delta int64) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	var err error
	key = db.encodeKVKey(key)

	t := db.kvBatch

	t.Lock()
	defer t.Unlock()

	var n int64
	n, err = StrInt64(db.bucket.Get(key))
	if err != nil {
		return 0, err
	}

	n += delta

	t.Put(key, StrPutInt64(n))

	//todo binlog

	err = t.Commit()
	return n, err
}

//	ps : here just focus on deleting the key-value data,
//		 any other likes expire is ignore.
func (db *DB) delete(t *batch, key []byte) int64 {
	key = db.encodeKVKey(key)
	t.Delete(key)
	return 1
}

func (db *DB) setExpireAt(key []byte, when int64) (int64, error) {
	t := db.kvBatch
	t.Lock()
	defer t.Unlock()

	if exist, err := db.Exists(key); err != nil || exist == 0 {
		return 0, err
	} else {
		db.expireAt(t, KVType, key, when)
		if err := t.Commit(); err != nil {
			return 0, err
		}
	}
	return 1, nil
}

func (db *DB) Decr(key []byte) (int64, error) {
	return db.incr(key, -1)
}

func (db *DB) DecrBy(key []byte, decrement int64) (int64, error) {
	return db.incr(key, -decrement)
}

func (db *DB) Del(keys ...[]byte) (int64, error) {
	if len(keys) == 0 {
		return 0, nil
	}

	codedKeys := make([][]byte, len(keys))
	for i, k := range keys {
		codedKeys[i] = db.encodeKVKey(k)
	}

	t := db.kvBatch
	t.Lock()
	defer t.Unlock()

	for i, k := range keys {
		t.Delete(codedKeys[i])
		db.rmExpire(t, KVType, k)
	}

	err := t.Commit()
	return int64(len(keys)), err
}

func (db *DB) Exists(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	var err error
	key = db.encodeKVKey(key)

	var v []byte
	v, err = db.bucket.Get(key)
	if v != nil && err == nil {
		return 1, nil
	}

	return 0, err
}

func (db *DB) Get(key []byte) ([]byte, error) {
	if err := checkKeySize(key); err != nil {
		return nil, err
	}

	key = db.encodeKVKey(key)

	return db.bucket.Get(key)
}

func (db *DB) GetSet(key []byte, value []byte) ([]byte, error) {
	if err := checkKeySize(key); err != nil {
		return nil, err
	} else if err := checkValueSize(value); err != nil {
		return nil, err
	}

	key = db.encodeKVKey(key)

	t := db.kvBatch

	t.Lock()
	defer t.Unlock()

	oldValue, err := db.bucket.Get(key)
	if err != nil {
		return nil, err
	}

	t.Put(key, value)
	//todo, binlog

	err = t.Commit()

	return oldValue, err
}

func (db *DB) Incr(key []byte) (int64, error) {
	return db.incr(key, 1)
}

func (db *DB) IncrBy(key []byte, increment int64) (int64, error) {
	return db.incr(key, increment)
}

func (db *DB) MGet(keys ...[]byte) ([][]byte, error) {
	values := make([][]byte, len(keys))

	it := db.bucket.NewIterator()
	defer it.Close()

	for i := range keys {
		if err := checkKeySize(keys[i]); err != nil {
			return nil, err
		}

		values[i] = it.Find(db.encodeKVKey(keys[i]))
	}

	return values, nil
}

func (db *DB) MSet(args ...KVPair) error {
	if len(args) == 0 {
		return nil
	}

	t := db.kvBatch

	var err error
	var key []byte
	var value []byte

	t.Lock()
	defer t.Unlock()

	for i := 0; i < len(args); i++ {
		if err := checkKeySize(args[i].Key); err != nil {
			return err
		} else if err := checkValueSize(args[i].Value); err != nil {
			return err
		}

		key = db.encodeKVKey(args[i].Key)

		value = args[i].Value

		t.Put(key, value)

		//todo binlog
	}

	err = t.Commit()
	return err
}

func (db *DB) Set(key []byte, value []byte) error {
	if err := checkKeySize(key); err != nil {
		return err
	} else if err := checkValueSize(value); err != nil {
		return err
	}

	var err error
	key = db.encodeKVKey(key)

	t := db.kvBatch

	t.Lock()
	defer t.Unlock()

	t.Put(key, value)

	err = t.Commit()

	return err
}

func (db *DB) SetNX(key []byte, value []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	} else if err := checkValueSize(value); err != nil {
		return 0, err
	}

	var err error
	key = db.encodeKVKey(key)

	var n int64 = 1

	t := db.kvBatch

	t.Lock()
	defer t.Unlock()

	if v, err := db.bucket.Get(key); err != nil {
		return 0, err
	} else if v != nil {
		n = 0
	} else {
		t.Put(key, value)

		//todo binlog

		err = t.Commit()
	}

	return n, err
}

func (db *DB) flush() (drop int64, err error) {
	t := db.kvBatch
	t.Lock()
	defer t.Unlock()
	return db.flushType(t, KVType)
}

//if inclusive is true, scan range [key, inf) else (key, inf)
func (db *DB) Scan(key []byte, count int, inclusive bool, match string) ([][]byte, error) {
	return db.scan(KVType, key, count, inclusive, match)
}

func (db *DB) Expire(key []byte, duration int64) (int64, error) {
	if duration <= 0 {
		return 0, errExpireValue
	}

	return db.setExpireAt(key, time.Now().Unix()+duration)
}

func (db *DB) ExpireAt(key []byte, when int64) (int64, error) {
	if when <= time.Now().Unix() {
		return 0, errExpireValue
	}

	return db.setExpireAt(key, when)
}

func (db *DB) TTL(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return -1, err
	}

	return db.ttl(KVType, key)
}

func (db *DB) Persist(key []byte) (int64, error) {
	if err := checkKeySize(key); err != nil {
		return 0, err
	}

	t := db.kvBatch
	t.Lock()
	defer t.Unlock()
	n, err := db.rmExpire(t, KVType, key)
	if err != nil {
		return 0, err
	}

	err = t.Commit()
	return n, err
}

func (db *DB) Lock() {
	t := db.kvBatch
	t.Lock()
}

func (db *DB) Remove(key []byte) bool {
	if len(key) == 0 {
		return false
	}
	t := db.kvBatch
	t.Delete(db.encodeKVKey(key))
	_, err := db.rmExpire(t, KVType, key)
	if err != nil {
		return false
	}
	return true
}

func (db *DB) Commit() error {
	t := db.kvBatch
	return t.Commit()
}

func (db *DB) Unlock() {
	t := db.kvBatch
	t.Unlock()
}
