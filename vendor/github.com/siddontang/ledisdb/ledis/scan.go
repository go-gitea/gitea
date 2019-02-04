package ledis

import (
	"errors"
	"github.com/siddontang/ledisdb/store"
	"regexp"
)

var errDataType = errors.New("error data type")
var errMetaKey = errors.New("error meta key")

func (db *DB) scan(dataType byte, key []byte, count int, inclusive bool, match string) ([][]byte, error) {
	return db.scanGeneric(dataType, key, count, inclusive, match, false)
}

func (db *DB) revscan(dataType byte, key []byte, count int, inclusive bool, match string) ([][]byte, error) {
	return db.scanGeneric(dataType, key, count, inclusive, match, true)
}

func (db *DB) scanGeneric(dataType byte, key []byte, count int,
	inclusive bool, match string, reverse bool) ([][]byte, error) {
	var minKey, maxKey []byte
	var err error
	var r *regexp.Regexp

	if len(match) > 0 {
		if r, err = regexp.Compile(match); err != nil {
			return nil, err
		}
	}

	tp := store.RangeOpen

	if !reverse {
		if minKey, err = db.encodeScanMinKey(dataType, key); err != nil {
			return nil, err
		}
		if maxKey, err = db.encodeScanMaxKey(dataType, nil); err != nil {
			return nil, err
		}

		if inclusive {
			tp = store.RangeROpen
		}
	} else {
		if minKey, err = db.encodeScanMinKey(dataType, nil); err != nil {
			return nil, err
		}
		if maxKey, err = db.encodeScanMaxKey(dataType, key); err != nil {
			return nil, err
		}

		if inclusive {
			tp = store.RangeLOpen
		}
	}

	if count <= 0 {
		count = defaultScanCount
	}

	var it *store.RangeLimitIterator
	if !reverse {
		it = db.bucket.RangeIterator(minKey, maxKey, tp)
	} else {
		it = db.bucket.RevRangeIterator(minKey, maxKey, tp)
	}

	v := make([][]byte, 0, count)

	for i := 0; it.Valid() && i < count; it.Next() {
		if k, err := db.decodeScanKey(dataType, it.Key()); err != nil {
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

func (db *DB) encodeScanMinKey(dataType byte, key []byte) ([]byte, error) {
	if len(key) == 0 {
		return db.encodeScanKey(dataType, nil)
	} else {
		if err := checkKeySize(key); err != nil {
			return nil, err
		}
		return db.encodeScanKey(dataType, key)
	}
}

func (db *DB) encodeScanMaxKey(dataType byte, key []byte) ([]byte, error) {
	if len(key) > 0 {
		if err := checkKeySize(key); err != nil {
			return nil, err
		}

		return db.encodeScanKey(dataType, key)
	}

	k, err := db.encodeScanKey(dataType, nil)
	if err != nil {
		return nil, err
	}
	k[len(k)-1] = dataType + 1
	return k, nil
}

func (db *DB) encodeScanKey(dataType byte, key []byte) ([]byte, error) {
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
func (db *DB) decodeScanKey(dataType byte, ek []byte) ([]byte, error) {
	if len(ek) < 2 || ek[0] != db.index || ek[1] != dataType {
		return nil, errMetaKey
	}
	return ek[2:], nil
}
