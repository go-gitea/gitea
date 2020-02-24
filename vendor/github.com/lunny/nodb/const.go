package nodb

import (
	"errors"
)

const (
	NoneType    byte = 0
	KVType      byte = 1
	HashType    byte = 2
	HSizeType   byte = 3
	ListType    byte = 4
	LMetaType   byte = 5
	ZSetType    byte = 6
	ZSizeType   byte = 7
	ZScoreType  byte = 8
	BitType     byte = 9
	BitMetaType byte = 10
	SetType     byte = 11
	SSizeType   byte = 12

	maxDataType byte = 100

	ExpTimeType byte = 101
	ExpMetaType byte = 102
)

var (
	TypeName = map[byte]string{
		KVType:      "kv",
		HashType:    "hash",
		HSizeType:   "hsize",
		ListType:    "list",
		LMetaType:   "lmeta",
		ZSetType:    "zset",
		ZSizeType:   "zsize",
		ZScoreType:  "zscore",
		BitType:     "bit",
		BitMetaType: "bitmeta",
		SetType:     "set",
		SSizeType:   "ssize",
		ExpTimeType: "exptime",
		ExpMetaType: "expmeta",
	}
)

const (
	defaultScanCount int = 10
)

var (
	errKeySize        = errors.New("invalid key size")
	errValueSize      = errors.New("invalid value size")
	errHashFieldSize  = errors.New("invalid hash field size")
	errSetMemberSize  = errors.New("invalid set member size")
	errZSetMemberSize = errors.New("invalid zset member size")
	errExpireValue    = errors.New("invalid expire value")
)

const (
	//we don't support too many databases
	MaxDBNumber uint8 = 16

	//max key size
	MaxKeySize int = 1024

	//max hash field size
	MaxHashFieldSize int = 1024

	//max zset member size
	MaxZSetMemberSize int = 1024

	//max set member size
	MaxSetMemberSize int = 1024

	//max value size
	MaxValueSize int = 10 * 1024 * 1024
)

var (
	ErrScoreMiss = errors.New("zset score miss")
)

const (
	BinLogTypeDeletion uint8 = 0x0
	BinLogTypePut      uint8 = 0x1
	BinLogTypeCommand  uint8 = 0x2
)

const (
	DBAutoCommit    uint8 = 0x0
	DBInTransaction uint8 = 0x1
	DBInMulti       uint8 = 0x2
)

var (
	Version = "0.1"
)
