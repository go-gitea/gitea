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

package domain

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/tidb/ddl"
	"github.com/pingcap/tidb/infoschema"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/meta"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/store/localstore"
	"github.com/pingcap/tidb/terror"
)

var ddlLastReloadSchemaTS = "ddl_last_reload_schema_ts"

// Domain represents a storage space. Different domains can use the same database name.
// Multiple domains can be used in parallel without synchronization.
type Domain struct {
	store      kv.Storage
	infoHandle *infoschema.Handle
	ddl        ddl.DDL
	leaseCh    chan time.Duration
	// nano seconds
	lastLeaseTS int64
	m           sync.Mutex
}

func (do *Domain) loadInfoSchema(txn kv.Transaction) (err error) {
	m := meta.NewMeta(txn)
	schemaMetaVersion, err := m.GetSchemaVersion()
	if err != nil {
		return errors.Trace(err)
	}

	info := do.infoHandle.Get()
	if info != nil && schemaMetaVersion <= info.SchemaMetaVersion() {
		// info may be changed by other txn, so here its version may be bigger than schema version,
		// so we don't need to reload.
		log.Debugf("[ddl] schema version is still %d, no need reload", schemaMetaVersion)
		return nil
	}

	schemas, err := m.ListDatabases()
	if err != nil {
		return errors.Trace(err)
	}

	for _, di := range schemas {
		if di.State != model.StatePublic {
			// schema is not public, can't be used outside.
			continue
		}

		tables, err1 := m.ListTables(di.ID)
		if err1 != nil {
			return errors.Trace(err1)
		}

		di.Tables = make([]*model.TableInfo, 0, len(tables))
		for _, tbl := range tables {
			if tbl.State != model.StatePublic {
				// schema is not public, can't be used outsiee.
				continue
			}
			di.Tables = append(di.Tables, tbl)
		}
	}

	log.Infof("[ddl] loadInfoSchema %d", schemaMetaVersion)
	err = do.infoHandle.Set(schemas, schemaMetaVersion)
	return errors.Trace(err)
}

// InfoSchema gets information schema from domain.
func (do *Domain) InfoSchema() infoschema.InfoSchema {
	// try reload if possible.
	do.tryReload()
	return do.infoHandle.Get()
}

// DDL gets DDL from domain.
func (do *Domain) DDL() ddl.DDL {
	return do.ddl
}

// Store gets KV store from domain.
func (do *Domain) Store() kv.Storage {
	return do.store
}

// SetLease will reset the lease time for online DDL change.
func (do *Domain) SetLease(lease time.Duration) {
	do.leaseCh <- lease

	// let ddl to reset lease too.
	do.ddl.SetLease(lease)
}

// Stats returns the domain statistic.
func (do *Domain) Stats() (map[string]interface{}, error) {
	m := make(map[string]interface{})
	m[ddlLastReloadSchemaTS] = atomic.LoadInt64(&do.lastLeaseTS) / 1e9

	return m, nil
}

// GetScope gets the status variables scope.
func (do *Domain) GetScope(status string) variable.ScopeFlag {
	// Now domain status variables scope are all default scope.
	return variable.DefaultScopeFlag
}

func (do *Domain) tryReload() {
	// if we don't have update the schema for a long time > lease, we must force reloading it.
	// Although we try to reload schema every lease time in a goroutine, sometimes it may not
	// run accurately, e.g, the machine has a very high load, running the ticker is delayed.
	last := atomic.LoadInt64(&do.lastLeaseTS)
	lease := do.ddl.GetLease()

	// if lease is 0, we use the local store, so no need to reload.
	if lease > 0 && time.Now().UnixNano()-last > lease.Nanoseconds() {
		do.mustReload()
	}
}

const minReloadTimeout = 20 * time.Second

func (do *Domain) reload() error {
	// lock here for only once at same time.
	do.m.Lock()
	defer do.m.Unlock()

	timeout := do.ddl.GetLease() / 2
	if timeout < minReloadTimeout {
		timeout = minReloadTimeout
	}

	done := make(chan error, 1)
	go func() {
		var err error

		for {
			err = kv.RunInNewTxn(do.store, false, do.loadInfoSchema)
			// if err is db closed, we will return it directly, otherwise, we will
			// check reloading again.
			if terror.ErrorEqual(err, localstore.ErrDBClosed) {
				break
			}

			if err != nil {
				log.Errorf("[ddl] load schema err %v, retry again", errors.ErrorStack(err))
				// TODO: use a backoff algorithm.
				time.Sleep(500 * time.Millisecond)
				continue
			}

			atomic.StoreInt64(&do.lastLeaseTS, time.Now().UnixNano())
			break
		}

		done <- err
	}()

	select {
	case err := <-done:
		return errors.Trace(err)
	case <-time.After(timeout):
		return errors.New("reload schema timeout")
	}
}

func (do *Domain) mustReload() {
	// if reload error, we will terminate whole program to guarantee data safe.
	err := do.reload()
	if err != nil {
		log.Fatalf("[ddl] reload schema err %v", errors.ErrorStack(err))
	}
}

// check schema every 300 seconds default.
const defaultLoadTime = 300 * time.Second

func (do *Domain) loadSchemaInLoop(lease time.Duration) {
	if lease <= 0 {
		lease = defaultLoadTime
	}

	ticker := time.NewTicker(lease)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			err := do.reload()
			// we may close store in test, but the domain load schema loop is still checking,
			// so we can't panic for ErrDBClosed and just return here.
			if terror.ErrorEqual(err, localstore.ErrDBClosed) {
				return
			} else if err != nil {
				log.Fatalf("[ddl] reload schema err %v", errors.ErrorStack(err))
			}
		case newLease := <-do.leaseCh:
			if newLease <= 0 {
				newLease = defaultLoadTime
			}

			if lease == newLease {
				// nothing to do
				continue
			}

			lease = newLease
			// reset ticker too.
			ticker.Stop()
			ticker = time.NewTicker(lease)
		}
	}
}

type ddlCallback struct {
	ddl.BaseCallback
	do *Domain
}

func (c *ddlCallback) OnChanged(err error) error {
	if err != nil {
		return err
	}
	log.Warnf("[ddl] on DDL change")

	c.do.mustReload()
	return nil
}

// NewDomain creates a new domain.
func NewDomain(store kv.Storage, lease time.Duration) (d *Domain, err error) {
	d = &Domain{
		store:   store,
		leaseCh: make(chan time.Duration, 1),
	}

	d.infoHandle = infoschema.NewHandle(d.store)
	d.ddl = ddl.NewDDL(d.store, d.infoHandle, &ddlCallback{do: d}, lease)
	d.mustReload()

	variable.RegisterStatistics(d)

	go d.loadSchemaInLoop(lease)

	return d, nil
}
