package ledis

import (
	"encoding/binary"
	"errors"
	"github.com/siddontang/go/hack"
	"strconv"
)

var errIntNumber = errors.New("invalid integer")

/*
	Below I forget why I use little endian to store int.
	Maybe I was foolish at that time.
*/

func Int64(v []byte, err error) (int64, error) {
	if err != nil {
		return 0, err
	} else if v == nil || len(v) == 0 {
		return 0, nil
	} else if len(v) != 8 {
		return 0, errIntNumber
	}

	return int64(binary.LittleEndian.Uint64(v)), nil
}

func Uint64(v []byte, err error) (uint64, error) {
	if err != nil {
		return 0, err
	} else if v == nil || len(v) == 0 {
		return 0, nil
	} else if len(v) != 8 {
		return 0, errIntNumber
	}

	return binary.LittleEndian.Uint64(v), nil
}

func PutInt64(v int64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, uint64(v))
	return b
}

func StrInt64(v []byte, err error) (int64, error) {
	if err != nil {
		return 0, err
	} else if v == nil {
		return 0, nil
	} else {
		return strconv.ParseInt(hack.String(v), 10, 64)
	}
}

func StrUint64(v []byte, err error) (uint64, error) {
	if err != nil {
		return 0, err
	} else if v == nil {
		return 0, nil
	} else {
		return strconv.ParseUint(hack.String(v), 10, 64)
	}
}

func StrInt32(v []byte, err error) (int32, error) {
	if err != nil {
		return 0, err
	} else if v == nil {
		return 0, nil
	} else {
		res, err := strconv.ParseInt(hack.String(v), 10, 32)
		return int32(res), err
	}
}

func StrInt8(v []byte, err error) (int8, error) {
	if err != nil {
		return 0, err
	} else if v == nil {
		return 0, nil
	} else {
		res, err := strconv.ParseInt(hack.String(v), 10, 8)
		return int8(res), err
	}
}

func AsyncNotify(ch chan struct{}) {
	select {
	case ch <- struct{}{}:
	default:
	}
}
