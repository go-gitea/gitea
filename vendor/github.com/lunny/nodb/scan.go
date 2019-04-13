package nodb

import (
	"bytes"
	"errors"
	"regexp"

	"github.com/lunny/nodb/store"
)

var errDataType = errors.New("error data type")
var errMetaKey = errors.New("error meta key")

// Seek search the prefix key
func (db *DB) Seek(key []byte) (*store.Iterator, error) {
	return db.seek(KVType, key)
}

func (db *DB) seek(dataType byte, key []byte) (*store.Iterator, error) {
	var minKey []byte
	var err error

	if len(key) > 0 {
		if err = checkKeySize(key); err != nil {
			return nil, err
		}
		if minKey, err = db.encodeMetaKey(dataType, key); err != nil {
			return nil, err
		}

	} else {
		if minKey, err = db.encodeMinKey(dataType); err != nil {
			return nil, err
		}
	}

	it := db.bucket.NewIterator()
	it.Seek(minKey)
	return it, nil
}

func (db *DB) MaxKey() ([]byte, error) {
	return db.encodeMaxKey(KVType)
}

func (db *DB) Key(it *store.Iterator) ([]byte, error) {
	return db.decodeMetaKey(KVType, it.Key())
}

func (db *DB) scan(dataType byte, key []byte, count int, inclusive bool, match string) ([][]byte, error) {
	var minKey, maxKey []byte
	var err error
	var r *regexp.Regexp

	if len(match) > 0 {
		if r, err = regexp.Compile(match); err != nil {
			return nil, err
		}
	}

	if len(key) > 0 {
		if err = checkKeySize(key); err != nil {
			return nil, err
		}
		if minKey, err = db.encodeMetaKey(dataType, key); err != nil {
			return nil, err
		}

	} else {
		if minKey, err = db.encodeMinKey(dataType); err != nil {
			return nil, err
		}
	}

	if maxKey, err = db.encodeMaxKey(dataType); err != nil {
		return nil, err
	}

	if count <= 0 {
		count = defaultScanCount
	}

	v := make([][]byte, 0, count)

	it := db.bucket.NewIterator()
	it.Seek(minKey)

	if !inclusive {
		if it.Valid() && bytes.Equal(it.RawKey(), minKey) {
			it.Next()
		}
	}

	for i := 0; it.Valid() && i < count && bytes.Compare(it.RawKey(), maxKey) < 0; it.Next() {
		if k, err := db.decodeMetaKey(dataType, it.Key()); err != nil {
			continue
		} else if r != nil && !r.Match(k) {
			continue
		} else {
			v = append(v, k)
			i++
		}
	}
	it.Close()
	return v, nil
}

func (db *DB) encodeMinKey(dataType byte) ([]byte, error) {
	return db.encodeMetaKey(dataType, nil)
}

func (db *DB) encodeMaxKey(dataType byte) ([]byte, error) {
	k, err := db.encodeMetaKey(dataType, nil)
	if err != nil {
		return nil, err
	}
	k[len(k)-1] = dataType + 1
	return k, nil
}

func (db *DB) encodeMetaKey(dataType byte, key []byte) ([]byte, error) {
	switch dataType {
	case KVType:
		return db.encodeKVKey(key), nil
	case LMetaType:
		return db.lEncodeMetaKey(key), nil
	case HSizeType:
		return db.hEncodeSizeKey(key), nil
	case ZSizeType:
		return db.zEncodeSizeKey(key), nil
	case BitMetaType:
		return db.bEncodeMetaKey(key), nil
	case SSizeType:
		return db.sEncodeSizeKey(key), nil
	default:
		return nil, errDataType
	}
}
func (db *DB) decodeMetaKey(dataType byte, ek []byte) ([]byte, error) {
	if len(ek) < 2 || ek[0] != db.index || ek[1] != dataType {
		return nil, errMetaKey
	}
	return ek[2:], nil
}
