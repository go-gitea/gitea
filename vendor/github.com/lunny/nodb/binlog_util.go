package nodb

import (
	"encoding/binary"
	"errors"
	"fmt"
	"strconv"
)

var (
	errBinLogDeleteType  = errors.New("invalid bin log delete type")
	errBinLogPutType     = errors.New("invalid bin log put type")
	errBinLogCommandType = errors.New("invalid bin log command type")
)

func encodeBinLogDelete(key []byte) []byte {
	buf := make([]byte, 1+len(key))
	buf[0] = BinLogTypeDeletion
	copy(buf[1:], key)
	return buf
}

func decodeBinLogDelete(sz []byte) ([]byte, error) {
	if len(sz) < 1 || sz[0] != BinLogTypeDeletion {
		return nil, errBinLogDeleteType
	}

	return sz[1:], nil
}

func encodeBinLogPut(key []byte, value []byte) []byte {
	buf := make([]byte, 3+len(key)+len(value))
	buf[0] = BinLogTypePut
	pos := 1
	binary.BigEndian.PutUint16(buf[pos:], uint16(len(key)))
	pos += 2
	copy(buf[pos:], key)
	pos += len(key)
	copy(buf[pos:], value)

	return buf
}

func decodeBinLogPut(sz []byte) ([]byte, []byte, error) {
	if len(sz) < 3 || sz[0] != BinLogTypePut {
		return nil, nil, errBinLogPutType
	}

	keyLen := int(binary.BigEndian.Uint16(sz[1:]))
	if 3+keyLen > len(sz) {
		return nil, nil, errBinLogPutType
	}

	return sz[3 : 3+keyLen], sz[3+keyLen:], nil
}

func FormatBinLogEvent(event []byte) (string, error) {
	logType := uint8(event[0])

	var err error
	var k []byte
	var v []byte

	var buf []byte = make([]byte, 0, 1024)

	switch logType {
	case BinLogTypePut:
		k, v, err = decodeBinLogPut(event)
		buf = append(buf, "PUT "...)
	case BinLogTypeDeletion:
		k, err = decodeBinLogDelete(event)
		buf = append(buf, "DELETE "...)
	default:
		err = errInvalidBinLogEvent
	}

	if err != nil {
		return "", err
	}

	if buf, err = formatDataKey(buf, k); err != nil {
		return "", err
	}

	if v != nil && len(v) != 0 {
		buf = append(buf, fmt.Sprintf(" %q", v)...)
	}

	return String(buf), nil
}

func formatDataKey(buf []byte, k []byte) ([]byte, error) {
	if len(k) < 2 {
		return nil, errInvalidBinLogEvent
	}

	buf = append(buf, fmt.Sprintf("DB:%2d ", k[0])...)
	buf = append(buf, fmt.Sprintf("%s ", TypeName[k[1]])...)

	db := new(DB)
	db.index = k[0]

	//to do format at respective place

	switch k[1] {
	case KVType:
		if key, err := db.decodeKVKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, String(key))
		}
	case HashType:
		if key, field, err := db.hDecodeHashKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, String(key))
			buf = append(buf, ' ')
			buf = strconv.AppendQuote(buf, String(field))
		}
	case HSizeType:
		if key, err := db.hDecodeSizeKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, String(key))
		}
	case ListType:
		if key, seq, err := db.lDecodeListKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, String(key))
			buf = append(buf, ' ')
			buf = strconv.AppendInt(buf, int64(seq), 10)
		}
	case LMetaType:
		if key, err := db.lDecodeMetaKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, String(key))
		}
	case ZSetType:
		if key, m, err := db.zDecodeSetKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, String(key))
			buf = append(buf, ' ')
			buf = strconv.AppendQuote(buf, String(m))
		}
	case ZSizeType:
		if key, err := db.zDecodeSizeKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, String(key))
		}
	case ZScoreType:
		if key, m, score, err := db.zDecodeScoreKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, String(key))
			buf = append(buf, ' ')
			buf = strconv.AppendQuote(buf, String(m))
			buf = append(buf, ' ')
			buf = strconv.AppendInt(buf, score, 10)
		}
	case BitType:
		if key, seq, err := db.bDecodeBinKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, String(key))
			buf = append(buf, ' ')
			buf = strconv.AppendUint(buf, uint64(seq), 10)
		}
	case BitMetaType:
		if key, err := db.bDecodeMetaKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, String(key))
		}
	case SetType:
		if key, member, err := db.sDecodeSetKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, String(key))
			buf = append(buf, ' ')
			buf = strconv.AppendQuote(buf, String(member))
		}
	case SSizeType:
		if key, err := db.sDecodeSizeKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, String(key))
		}
	case ExpTimeType:
		if tp, key, t, err := db.expDecodeTimeKey(k); err != nil {
			return nil, err
		} else {
			buf = append(buf, TypeName[tp]...)
			buf = append(buf, ' ')
			buf = strconv.AppendQuote(buf, String(key))
			buf = append(buf, ' ')
			buf = strconv.AppendInt(buf, t, 10)
		}
	case ExpMetaType:
		if tp, key, err := db.expDecodeMetaKey(k); err != nil {
			return nil, err
		} else {
			buf = append(buf, TypeName[tp]...)
			buf = append(buf, ' ')
			buf = strconv.AppendQuote(buf, String(key))
		}
	default:
		return nil, errInvalidBinLogEvent
	}

	return buf, nil
}
