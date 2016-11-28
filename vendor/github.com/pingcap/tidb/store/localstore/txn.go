// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package localstore

import (
	"fmt"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/tidb/kv"
)

var (
	_ kv.Transaction = (*dbTxn)(nil)
)

// dbTxn is not thread safe
type dbTxn struct {
	us         kv.UnionStore
	store      *dbStore // for commit
	tid        uint64
	valid      bool
	version    kv.Version          // commit version
	lockedKeys map[string]struct{} // origin version in snapshot
	dirty      bool
}

func newTxn(s *dbStore, ver kv.Version) *dbTxn {
	txn := &dbTxn{
		us:         kv.NewUnionStore(newSnapshot(s, ver)),
		store:      s,
		tid:        ver.Ver,
		valid:      true,
		version:    kv.MinVersion,
		lockedKeys: make(map[string]struct{}),
	}
	log.Debugf("[kv] Begin txn:%d", txn.tid)
	return txn
}

// Implement transaction interface

func (txn *dbTxn) Get(k kv.Key) ([]byte, error) {
	log.Debugf("[kv] get key:%q, txn:%d", k, txn.tid)
	return txn.us.Get(k)
}

func (txn *dbTxn) Set(k kv.Key, data []byte) error {
	log.Debugf("[kv] set key:%q, txn:%d", k, txn.tid)
	txn.dirty = true
	return txn.us.Set(k, data)
}

func (txn *dbTxn) String() string {
	return fmt.Sprintf("%d", txn.tid)
}

func (txn *dbTxn) Seek(k kv.Key) (kv.Iterator, error) {
	log.Debugf("[kv] seek key:%q, txn:%d", k, txn.tid)
	return txn.us.Seek(k)
}

func (txn *dbTxn) Delete(k kv.Key) error {
	log.Debugf("[kv] delete key:%q, txn:%d", k, txn.tid)
	txn.dirty = true
	return txn.us.Delete(k)
}

func (txn *dbTxn) SetOption(opt kv.Option, val interface{}) {
	txn.us.SetOption(opt, val)
}

func (txn *dbTxn) DelOption(opt kv.Option) {
	txn.us.DelOption(opt)
}

func (txn *dbTxn) doCommit() error {
	// check lazy condition pairs
	if err := txn.us.CheckLazyConditionPairs(); err != nil {
		return errors.Trace(err)
	}

	err := txn.us.WalkBuffer(func(k kv.Key, v []byte) error {
		e := txn.LockKeys(k)
		return errors.Trace(e)
	})
	if err != nil {
		return errors.Trace(err)
	}

	return txn.store.CommitTxn(txn)
}

func (txn *dbTxn) Commit() error {
	if !txn.valid {
		return errors.Trace(kv.ErrInvalidTxn)
	}
	log.Debugf("[kv] commit txn %d", txn.tid)
	defer func() {
		txn.close()
	}()

	return errors.Trace(txn.doCommit())
}

func (txn *dbTxn) close() error {
	txn.us.Release()
	txn.lockedKeys = nil
	txn.valid = false
	return nil
}

func (txn *dbTxn) Rollback() error {
	if !txn.valid {
		return errors.Trace(kv.ErrInvalidTxn)
	}
	log.Warnf("[kv] Rollback txn %d", txn.tid)
	return txn.close()
}

func (txn *dbTxn) LockKeys(keys ...kv.Key) error {
	for _, key := range keys {
		txn.lockedKeys[string(key)] = struct{}{}
	}
	return nil
}

func (txn *dbTxn) IsReadOnly() bool {
	return !txn.dirty
}

func (txn *dbTxn) StartTS() int64 {
	return int64(txn.tid)
}

func (txn *dbTxn) GetClient() kv.Client {
	return nil
}

type dbClient struct {
}

func (c *dbClient) SupportRequestType(reqType, subType int64) bool {
	return false
}

func (c *dbClient) Send(req *kv.Request) kv.Response {
	return nil
}
