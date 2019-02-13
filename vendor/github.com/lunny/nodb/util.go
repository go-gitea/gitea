package nodb

import (
	"encoding/binary"
	"errors"
	"reflect"
	"strconv"
	"unsafe"
)

var errIntNumber = errors.New("invalid integer")

// no copy to change slice to string
// use your own risk
func String(b []byte) (s string) {
	pbytes := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	pstring := (*reflect.StringHeader)(unsafe.Pointer(&s))
	pstring.Data = pbytes.Data
	pstring.Len = pbytes.Len
	return
}

// no copy to change string to slice
// use your own risk
func Slice(s string) (b []byte) {
	pbytes := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	pstring := (*reflect.StringHeader)(unsafe.Pointer(&s))
	pbytes.Data = pstring.Data
	pbytes.Len = pstring.Len
	pbytes.Cap = pstring.Len
	return
}

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

func PutInt64(v int64) []byte {
	var b []byte
	pbytes := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	pbytes.Data = uintptr(unsafe.Pointer(&v))
	pbytes.Len = 8
	pbytes.Cap = 8
	return b
}

func StrInt64(v []byte, err error) (int64, error) {
	if err != nil {
		return 0, err
	} else if v == nil {
		return 0, nil
	} else {
		return strconv.ParseInt(String(v), 10, 64)
	}
}

func StrInt32(v []byte, err error) (int32, error) {
	if err != nil {
		return 0, err
	} else if v == nil {
		return 0, nil
	} else {
		res, err := strconv.ParseInt(String(v), 10, 32)
		return int32(res), err
	}
}

func StrInt8(v []byte, err error) (int8, error) {
	if err != nil {
		return 0, err
	} else if v == nil {
		return 0, nil
	} else {
		res, err := strconv.ParseInt(String(v), 10, 8)
		return int8(res), err
	}
}

func StrPutInt64(v int64) []byte {
	return strconv.AppendInt(nil, v, 10)
}

func MinUInt32(a uint32, b uint32) uint32 {
	if a > b {
		return b
	} else {
		return a
	}
}

func MaxUInt32(a uint32, b uint32) uint32 {
	if a > b {
		return a
	} else {
		return b
	}
}

func MaxInt32(a int32, b int32) int32 {
	if a > b {
		return a
	} else {
		return b
	}
}
