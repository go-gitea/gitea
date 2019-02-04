package ledis

import (
	"errors"
)

const Version = "0.4"

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

	/*
		I make a big mistake about TTL time key format and have to use a new one (change 101 to 103).
		You must run the ledis-upgrade-ttl to upgrade db.
	*/
	ObsoleteExpTimeType byte = 101
	ExpMetaType         byte = 102
	ExpTimeType         byte = 103

	MetaType byte = 201
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
	MaxValueSize int = 1024 * 1024 * 1024
)

var (
	ErrScoreMiss     = errors.New("zset score miss")
	ErrWriteInROnly  = errors.New("write not support in readonly mode")
	ErrRplInRDWR     = errors.New("replication not support in read write mode")
	ErrRplNotSupport = errors.New("replication not support")
)

const (
	DBAutoCommit    uint8 = 0x0
	DBInTransaction uint8 = 0x1
	DBInMulti       uint8 = 0x2
)
