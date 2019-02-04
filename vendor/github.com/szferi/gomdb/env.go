package mdb

/*
#cgo CFLAGS: -pthread -W -Wall -Wno-unused-parameter -Wbad-function-cast -O2 -g
#cgo freebsd CFLAGS: -DMDB_DSYNC=O_SYNC
#cgo openbsd CFLAGS: -DMDB_DSYNC=O_SYNC
#cgo netbsd CFLAGS: -DMDB_DSYNC=O_SYNC
#include <stdlib.h>
#include <stdio.h>
#include "lmdb.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

const SUCCESS = C.MDB_SUCCESS

// mdb_env Environment Flags
const (
	FIXEDMAP   = C.MDB_FIXEDMAP   // mmap at a fixed address (experimental)
	NOSUBDIR   = C.MDB_NOSUBDIR   // no environment directory
	NOSYNC     = C.MDB_NOSYNC     // don't fsync after commit
	RDONLY     = C.MDB_RDONLY     // read only
	NOMETASYNC = C.MDB_NOMETASYNC // don't fsync metapage after commit
	WRITEMAP   = C.MDB_WRITEMAP   // use writable mmap
	MAPASYNC   = C.MDB_MAPASYNC   // use asynchronous msync when MDB_WRITEMAP is use
	NOTLS      = C.MDB_NOTLS      // tie reader locktable slots to Txn objects instead of threads
)

type DBI uint

type Errno C.int

// minimum and maximum values produced for the Errno type. syscall.Errnos of
// other values may still be produced.
const minErrno, maxErrno C.int = C.MDB_KEYEXIST, C.MDB_LAST_ERRCODE

func (e Errno) Error() string {
	s := C.GoString(C.mdb_strerror(C.int(e)))
	if s == "" {
		return fmt.Sprint("mdb errno:", int(e))
	}
	return s
}

// for tests that can't import C
func _errno(ret int) error {
	return errno(C.int(ret))
}

func errno(ret C.int) error {
	if ret == C.MDB_SUCCESS {
		return nil
	}
	if minErrno <= ret && ret <= maxErrno {
		return Errno(ret)
	}
	return syscall.Errno(ret)
}

// error codes
const (
	KeyExist        = Errno(C.MDB_KEYEXIST)
	NotFound        = Errno(C.MDB_NOTFOUND)
	PageNotFound    = Errno(C.MDB_PAGE_NOTFOUND)
	Corrupted       = Errno(C.MDB_CORRUPTED)
	Panic           = Errno(C.MDB_PANIC)
	VersionMismatch = Errno(C.MDB_VERSION_MISMATCH)
	Invalid         = Errno(C.MDB_INVALID)
	MapFull         = Errno(C.MDB_MAP_FULL)
	DbsFull         = Errno(C.MDB_DBS_FULL)
	ReadersFull     = Errno(C.MDB_READERS_FULL)
	TlsFull         = Errno(C.MDB_TLS_FULL)
	TxnFull         = Errno(C.MDB_TXN_FULL)
	CursorFull      = Errno(C.MDB_CURSOR_FULL)
	PageFull        = Errno(C.MDB_PAGE_FULL)
	MapResized      = Errno(C.MDB_MAP_RESIZED)
	Incompatibile   = Errno(C.MDB_INCOMPATIBLE)
)

func Version() string {
	var major, minor, patch *C.int
	ver_str := C.mdb_version(major, minor, patch)
	return C.GoString(ver_str)
}

// Env is opaque structure for a database environment.
// A DB environment supports multiple databases, all residing in the
// same shared-memory map.
type Env struct {
	_env *C.MDB_env
}

// Create an MDB environment handle.
func NewEnv() (*Env, error) {
	var _env *C.MDB_env
	ret := C.mdb_env_create(&_env)
	if ret != SUCCESS {
		return nil, errno(ret)
	}
	return &Env{_env}, nil
}

// Open an environment handle. If this function fails Close() must be called to discard the Env handle.
func (env *Env) Open(path string, flags uint, mode uint) error {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	ret := C.mdb_env_open(env._env, cpath, C.uint(NOTLS|flags), C.mdb_mode_t(mode))
	return errno(ret)
}

func (env *Env) Close() error {
	if env._env == nil {
		return errors.New("Environment already closed")
	}
	C.mdb_env_close(env._env)
	env._env = nil
	return nil
}

func (env *Env) Copy(path string) error {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	ret := C.mdb_env_copy(env._env, cpath)
	return errno(ret)
}

// Statistics for a database in the environment
type Stat struct {
	PSize         uint   // Size of a database page. This is currently the same for all databases.
	Depth         uint   // Depth (height) of the B-tree
	BranchPages   uint64 // Number of internal (non-leaf) pages
	LeafPages     uint64 // Number of leaf pages
	OverflowPages uint64 // Number of overflow pages
	Entries       uint64 // Number of data items
}

func (env *Env) Stat() (*Stat, error) {
	var _stat C.MDB_stat
	ret := C.mdb_env_stat(env._env, &_stat)
	if ret != SUCCESS {
		return nil, errno(ret)
	}
	stat := Stat{PSize: uint(_stat.ms_psize),
		Depth:         uint(_stat.ms_depth),
		BranchPages:   uint64(_stat.ms_branch_pages),
		LeafPages:     uint64(_stat.ms_leaf_pages),
		OverflowPages: uint64(_stat.ms_overflow_pages),
		Entries:       uint64(_stat.ms_entries)}
	return &stat, nil
}

type Info struct {
	MapSize    uint64 // Size of the data memory map
	LastPNO    uint64 // ID of the last used page
	LastTxnID  uint64 // ID of the last committed transaction
	MaxReaders uint   // maximum number of threads for the environment
	NumReaders uint   // maximum number of threads used in the environment
}

func (env *Env) Info() (*Info, error) {
	var _info C.MDB_envinfo
	ret := C.mdb_env_info(env._env, &_info)
	if ret != SUCCESS {
		return nil, errno(ret)
	}
	info := Info{MapSize: uint64(_info.me_mapsize),
		LastPNO:    uint64(_info.me_last_pgno),
		LastTxnID:  uint64(_info.me_last_txnid),
		MaxReaders: uint(_info.me_maxreaders),
		NumReaders: uint(_info.me_numreaders)}
	return &info, nil
}

func (env *Env) Sync(force int) error {
	ret := C.mdb_env_sync(env._env, C.int(force))
	return errno(ret)
}

func (env *Env) SetFlags(flags uint, onoff int) error {
	ret := C.mdb_env_set_flags(env._env, C.uint(flags), C.int(onoff))
	return errno(ret)
}

func (env *Env) Flags() (uint, error) {
	var _flags C.uint
	ret := C.mdb_env_get_flags(env._env, &_flags)
	if ret != SUCCESS {
		return 0, errno(ret)
	}
	return uint(_flags), nil
}

func (env *Env) Path() (string, error) {
	var path string
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	ret := C.mdb_env_get_path(env._env, &cpath)
	if ret != SUCCESS {
		return "", errno(ret)
	}
	return C.GoString(cpath), nil
}

func (env *Env) SetMapSize(size uint64) error {
	ret := C.mdb_env_set_mapsize(env._env, C.size_t(size))
	return errno(ret)
}

func (env *Env) SetMaxReaders(size uint) error {
	ret := C.mdb_env_set_maxreaders(env._env, C.uint(size))
	return errno(ret)
}

func (env *Env) SetMaxDBs(size DBI) error {
	ret := C.mdb_env_set_maxdbs(env._env, C.MDB_dbi(size))
	return errno(ret)
}

func (env *Env) DBIClose(dbi DBI) {
	C.mdb_dbi_close(env._env, C.MDB_dbi(dbi))
}
