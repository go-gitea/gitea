package ledis

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/siddontang/go/hack"
)

var errInvalidEvent = errors.New("invalid event")

func formatEventKey(buf []byte, k []byte) ([]byte, error) {
	if len(k) < 2 {
		return nil, errInvalidEvent
	}

	buf = append(buf, fmt.Sprintf("DB:%2d ", k[0])...)
	buf = append(buf, fmt.Sprintf("%s ", TypeName[k[1]])...)

	db := new(DB)
	index, _, err := decodeDBIndex(k)
	if err != nil {
		return nil, err
	}
	db.setIndex(index)

	//to do format at respective place

	switch k[1] {
	case KVType:
		if key, err := db.decodeKVKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, hack.String(key))
		}
	case HashType:
		if key, field, err := db.hDecodeHashKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, hack.String(key))
			buf = append(buf, ' ')
			buf = strconv.AppendQuote(buf, hack.String(field))
		}
	case HSizeType:
		if key, err := db.hDecodeSizeKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, hack.String(key))
		}
	case ListType:
		if key, seq, err := db.lDecodeListKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, hack.String(key))
			buf = append(buf, ' ')
			buf = strconv.AppendInt(buf, int64(seq), 10)
		}
	case LMetaType:
		if key, err := db.lDecodeMetaKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, hack.String(key))
		}
	case ZSetType:
		if key, m, err := db.zDecodeSetKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, hack.String(key))
			buf = append(buf, ' ')
			buf = strconv.AppendQuote(buf, hack.String(m))
		}
	case ZSizeType:
		if key, err := db.zDecodeSizeKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, hack.String(key))
		}
	case ZScoreType:
		if key, m, score, err := db.zDecodeScoreKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, hack.String(key))
			buf = append(buf, ' ')
			buf = strconv.AppendQuote(buf, hack.String(m))
			buf = append(buf, ' ')
			buf = strconv.AppendInt(buf, score, 10)
		}
	case SetType:
		if key, member, err := db.sDecodeSetKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, hack.String(key))
			buf = append(buf, ' ')
			buf = strconv.AppendQuote(buf, hack.String(member))
		}
	case SSizeType:
		if key, err := db.sDecodeSizeKey(k); err != nil {
			return nil, err
		} else {
			buf = strconv.AppendQuote(buf, hack.String(key))
		}
	case ExpTimeType:
		if tp, key, t, err := db.expDecodeTimeKey(k); err != nil {
			return nil, err
		} else {
			buf = append(buf, TypeName[tp]...)
			buf = append(buf, ' ')
			buf = strconv.AppendQuote(buf, hack.String(key))
			buf = append(buf, ' ')
			buf = strconv.AppendInt(buf, t, 10)
		}
	case ExpMetaType:
		if tp, key, err := db.expDecodeMetaKey(k); err != nil {
			return nil, err
		} else {
			buf = append(buf, TypeName[tp]...)
			buf = append(buf, ' ')
			buf = strconv.AppendQuote(buf, hack.String(key))
		}
	default:
		return nil, errInvalidEvent
	}

	return buf, nil
}
