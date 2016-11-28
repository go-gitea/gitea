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
	"github.com/juju/errors"
	"github.com/pingcap/go-hbase"
	"github.com/pingcap/go-themis"
	"github.com/pingcap/tidb/kv"
)

var (
	_ kv.Snapshot = (*hbaseSnapshot)(nil)
	_ kv.Iterator = (*hbaseIter)(nil)
)

// hbaseBatchSize is used for go-themis Scanner.
const hbaseBatchSize = 1000

// hbaseSnapshot implements MvccSnapshot interface.
type hbaseSnapshot struct {
	txn       themis.Txn
	storeName string
}

// newHBaseSnapshot creates a snapshot of an HBase store.
func newHbaseSnapshot(txn themis.Txn, storeName string) *hbaseSnapshot {
	return &hbaseSnapshot{
		txn:       txn,
		storeName: storeName,
	}
}

// Get gets the value for key k from snapshot.
func (s *hbaseSnapshot) Get(k kv.Key) ([]byte, error) {
	g := hbase.NewGet([]byte(k))
	g.AddColumn(hbaseColFamilyBytes, hbaseQualifierBytes)
	v, err := internalGet(s, g)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return v, nil
}

// BatchGet implements kv.Snapshot.BatchGet interface.
func (s *hbaseSnapshot) BatchGet(keys []kv.Key) (map[string][]byte, error) {
	gets := make([]*hbase.Get, len(keys))
	for i, key := range keys {
		g := hbase.NewGet(key)
		g.AddColumn(hbaseColFamilyBytes, hbaseQualifierBytes)
		gets[i] = g
	}
	rows, err := s.txn.Gets(s.storeName, gets)
	if err != nil {
		return nil, errors.Trace(err)
	}

	m := make(map[string][]byte, len(rows))
	for _, r := range rows {
		k := string(r.Row)
		v := r.Columns[hbaseFmlAndQual].Value
		m[k] = v
	}
	return m, nil
}

func internalGet(s *hbaseSnapshot, g *hbase.Get) ([]byte, error) {
	r, err := s.txn.Get(s.storeName, g)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if r == nil || len(r.Columns) == 0 {
		return nil, errors.Trace(kv.ErrNotExist)
	}
	return r.Columns[hbaseFmlAndQual].Value, nil
}

func (s *hbaseSnapshot) Seek(k kv.Key) (kv.Iterator, error) {
	scanner := s.txn.GetScanner([]byte(s.storeName), []byte(k), nil, hbaseBatchSize)
	return newInnerScanner(scanner), nil
}

func newInnerScanner(scanner *themis.ThemisScanner) kv.Iterator {
	it := &hbaseIter{
		ThemisScanner: scanner,
	}
	it.Next()
	return it
}

func (s *hbaseSnapshot) Release() {
	if s.txn != nil {
		s.txn.Release()
		s.txn = nil
	}
}

type hbaseIter struct {
	*themis.ThemisScanner
	rs *hbase.ResultRow
}

func (it *hbaseIter) Next() error {
	it.rs = it.ThemisScanner.Next()
	return nil
}

func (it *hbaseIter) Valid() bool {
	if it.rs == nil || len(it.rs.Columns) == 0 {
		return false
	}
	if it.ThemisScanner.Closed() {
		return false
	}
	return true
}

func (it *hbaseIter) Key() kv.Key {
	return it.rs.Row
}

func (it *hbaseIter) Value() []byte {
	return it.rs.Columns[hbaseFmlAndQual].Value
}

func (it *hbaseIter) Close() {
	if it.ThemisScanner != nil {
		it.ThemisScanner.Close()
		it.ThemisScanner = nil
	}
	it.rs = nil
}
