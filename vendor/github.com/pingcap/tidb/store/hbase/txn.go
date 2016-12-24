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

package hbasekv

import (
	"fmt"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/go-hbase"
	"github.com/pingcap/go-themis"
	"github.com/pingcap/tidb/kv"
)

var (
	_ kv.Transaction = (*hbaseTxn)(nil)
)

// dbTxn implements kv.Transacton. It is not thread safe.
type hbaseTxn struct {
	us        kv.UnionStore
	txn       themis.Txn
	store     *hbaseStore // for commit
	storeName string
	tid       uint64
	valid     bool
	version   kv.Version // commit version
	dirty     bool
}

func newHbaseTxn(t themis.Txn, storeName string) *hbaseTxn {
	return &hbaseTxn{
		txn:       t,
		valid:     true,
		storeName: storeName,
		tid:       t.GetStartTS(),
		us:        kv.NewUnionStore(newHbaseSnapshot(t, storeName)),
	}
}

// Implement transaction interface

func (txn *hbaseTxn) Get(k kv.Key) ([]byte, error) {
	log.Debugf("[kv] get key:%q, txn:%d", k, txn.tid)
	return txn.us.Get(k)
}

func (txn *hbaseTxn) Set(k kv.Key, v []byte) error {
	log.Debugf("[kv] set %q txn:%d", k, txn.tid)
	txn.dirty = true
	return txn.us.Set(k, v)
}

func (txn *hbaseTxn) String() string {
	return fmt.Sprintf("%d", txn.tid)
}

func (txn *hbaseTxn) Seek(k kv.Key) (kv.Iterator, error) {
	log.Debugf("[kv] seek %q txn:%d", k, txn.tid)
	return txn.us.Seek(k)
}

func (txn *hbaseTxn) Delete(k kv.Key) error {
	log.Debugf("[kv] delete %q txn:%d", k, txn.tid)
	txn.dirty = true
	return txn.us.Delete(k)
}

func (txn *hbaseTxn) SetOption(opt kv.Option, val interface{}) {
	txn.us.SetOption(opt, val)
}

func (txn *hbaseTxn) DelOption(opt kv.Option) {
	txn.us.DelOption(opt)
}

func (txn *hbaseTxn) doCommit() error {
	if err := txn.us.CheckLazyConditionPairs(); err != nil {
		return errors.Trace(err)
	}

	err := txn.us.WalkBuffer(func(k kv.Key, v []byte) error {
		row := append([]byte(nil), k...)
		if len(v) == 0 { // Deleted marker
			d := hbase.NewDelete(row)
			d.AddStringColumn(hbaseColFamily, hbaseQualifier)
			err := txn.txn.Delete(txn.storeName, d)
			if err != nil {
				return errors.Trace(err)
			}
		} else {
			val := append([]byte(nil), v...)
			p := hbase.NewPut(row)
			p.AddValue(hbaseColFamilyBytes, hbaseQualifierBytes, val)
			txn.txn.Put(txn.storeName, p)
		}
		return nil
	})

	if err != nil {
		return errors.Trace(err)
	}

	err = txn.txn.Commit()
	if err != nil {
		log.Error(err)
		return errors.Trace(err)
	}

	txn.version = kv.NewVersion(txn.txn.GetCommitTS())
	log.Debugf("[kv] commit successfully, txn.version:%d", txn.version.Ver)
	return nil
}

func (txn *hbaseTxn) Commit() error {
	if !txn.valid {
		return kv.ErrInvalidTxn
	}
	log.Debugf("[kv] start to commit txn %d", txn.tid)
	defer func() {
		txn.close()
	}()
	return txn.doCommit()
}

func (txn *hbaseTxn) close() error {
	txn.us.Release()
	txn.valid = false
	return nil
}

//if fail, themis auto rollback
func (txn *hbaseTxn) Rollback() error {
	if !txn.valid {
		return kv.ErrInvalidTxn
	}
	log.Warnf("[kv] Rollback txn %d", txn.tid)
	return txn.close()
}

func (txn *hbaseTxn) LockKeys(keys ...kv.Key) error {
	for _, key := range keys {
		if err := txn.txn.LockRow(txn.storeName, key); err != nil {
			return errors.Trace(err)
		}
	}
	return nil
}

func (txn *hbaseTxn) IsReadOnly() bool {
	return !txn.dirty
}

func (txn *hbaseTxn) StartTS() int64 {
	return int64(txn.tid)
}

func (txn *hbaseTxn) GetClient() kv.Client {
	return nil
}

type hbaseClient struct {
}

func (c *hbaseClient) SupportRequestType(reqType, subType int64) bool {
	return false
}

func (c *hbaseClient) Send(req *kv.Request) kv.Response {
	return nil
}
