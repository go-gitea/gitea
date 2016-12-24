// Copyright 2013 The ql Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSES/QL-LICENSE file.

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

package tidb

import (
	"net/http"
	"time"
	// For pprof
	_ "net/http/pprof"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/domain"
	"github.com/pingcap/tidb/executor"
	"github.com/pingcap/tidb/kv"
	"github.com/pingcap/tidb/parser"
	"github.com/pingcap/tidb/sessionctx/autocommit"
	"github.com/pingcap/tidb/sessionctx/variable"
	"github.com/pingcap/tidb/store/hbase"
	"github.com/pingcap/tidb/store/localstore"
	"github.com/pingcap/tidb/store/localstore/boltdb"
	"github.com/pingcap/tidb/store/localstore/engine"
	"github.com/pingcap/tidb/store/localstore/goleveldb"
	"github.com/pingcap/tidb/util/types"
)

// Engine prefix name
const (
	EngineGoLevelDBMemory     = "memory://"
	EngineGoLevelDBPersistent = "goleveldb://"
	EngineBoltDB              = "boltdb://"
	EngineHBase               = "hbase://"
	defaultMaxRetries         = 30
	retrySleepInterval        = 500 * time.Millisecond
)

type domainMap struct {
	domains map[string]*domain.Domain
	mu      sync.Mutex
}

func (dm *domainMap) Get(store kv.Storage) (d *domain.Domain, err error) {
	key := store.UUID()
	dm.mu.Lock()
	defer dm.mu.Unlock()
	d = dm.domains[key]
	if d != nil {
		return
	}

	lease := time.Duration(0)
	if !localstore.IsLocalStore(store) {
		lease = schemaLease
	}
	d, err = domain.NewDomain(store, lease)
	if err != nil {
		return nil, errors.Trace(err)
	}
	dm.domains[key] = d
	return
}

var (
	domap = &domainMap{
		domains: map[string]*domain.Domain{},
	}
	stores = make(map[string]kv.Driver)
	// EnablePprof indicates whether to enable HTTP Pprof or not.
	EnablePprof = os.Getenv("TIDB_PPROF") != "0"
	// PprofAddr is the pprof url.
	PprofAddr = "localhost:8888"
	// store.UUID()-> IfBootstrapped
	storeBootstrapped = make(map[string]bool)

	// schemaLease is the time for re-updating remote schema.
	// In online DDL, we must wait 2 * SchemaLease time to guarantee
	// all servers get the neweset schema.
	// Default schema lease time is 1 second, you can change it with a proper time,
	// but you must know that too little may cause badly performance degradation.
	// For production, you should set a big schema lease, like 300s+.
	schemaLease = 1 * time.Second
)

// SetSchemaLease changes the default schema lease time for DDL.
// This function is very dangerous, don't use it if you really know what you do.
// SetSchemaLease only affects not local storage after bootstrapped.
func SetSchemaLease(lease time.Duration) {
	schemaLease = lease
}

// What character set should the server translate a statement to after receiving it?
// For this, the server uses the character_set_connection and collation_connection system variables.
// It converts statements sent by the client from character_set_client to character_set_connection
// (except for string literals that have an introducer such as _latin1 or _utf8).
// collation_connection is important for comparisons of literal strings.
// For comparisons of strings with column values, collation_connection does not matter because columns
// have their own collation, which has a higher collation precedence.
// See: https://dev.mysql.com/doc/refman/5.7/en/charset-connection.html
func getCtxCharsetInfo(ctx context.Context) (string, string) {
	sessionVars := variable.GetSessionVars(ctx)
	charset := sessionVars.Systems["character_set_connection"]
	collation := sessionVars.Systems["collation_connection"]
	return charset, collation
}

// Parse parses a query string to raw ast.StmtNode.
func Parse(ctx context.Context, src string) ([]ast.StmtNode, error) {
	log.Debug("compiling", src)
	charset, collation := getCtxCharsetInfo(ctx)
	stmts, err := parser.Parse(src, charset, collation)
	if err != nil {
		log.Warnf("compiling %s, error: %v", src, err)
		return nil, errors.Trace(err)
	}
	return stmts, nil
}

// Compile is safe for concurrent use by multiple goroutines.
func Compile(ctx context.Context, rawStmt ast.StmtNode) (ast.Statement, error) {
	compiler := &executor.Compiler{}
	st, err := compiler.Compile(ctx, rawStmt)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return st, nil
}

func runStmt(ctx context.Context, s ast.Statement, args ...interface{}) (ast.RecordSet, error) {
	var err error
	var rs ast.RecordSet
	// before every execution, we must clear affectedrows.
	variable.GetSessionVars(ctx).SetAffectedRows(0)
	if s.IsDDL() {
		err = ctx.FinishTxn(false)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	rs, err = s.Exec(ctx)
	// All the history should be added here.
	se := ctx.(*session)
	se.history.add(0, s)
	// MySQL DDL should be auto-commit
	if s.IsDDL() || autocommit.ShouldAutocommit(ctx) {
		if err != nil {
			ctx.FinishTxn(true)
		} else {
			err = ctx.FinishTxn(false)
		}
	}
	return rs, errors.Trace(err)
}

// GetRows gets all the rows from a RecordSet.
func GetRows(rs ast.RecordSet) ([][]types.Datum, error) {
	if rs == nil {
		return nil, nil
	}
	var rows [][]types.Datum
	defer rs.Close()
	// Negative limit means no limit.
	for {
		row, err := rs.Next()
		if err != nil {
			return nil, errors.Trace(err)
		}
		if row == nil {
			break
		}
		rows = append(rows, row.Data)
	}
	return rows, nil
}

// RegisterStore registers a kv storage with unique name and its associated Driver.
func RegisterStore(name string, driver kv.Driver) error {
	name = strings.ToLower(name)

	if _, ok := stores[name]; ok {
		return errors.Errorf("%s is already registered", name)
	}

	stores[name] = driver
	return nil
}

// RegisterLocalStore registers a local kv storage with unique name and its associated engine Driver.
func RegisterLocalStore(name string, driver engine.Driver) error {
	d := localstore.Driver{Driver: driver}
	return RegisterStore(name, d)
}

// NewStore creates a kv Storage with path.
//
// The path must be a URL format 'engine://path?params' like the one for
// tidb.Open() but with the dbname cut off.
// Examples:
//    goleveldb://relative/path
//    boltdb:///absolute/path
//    hbase://zk1,zk2,zk3/hbasetbl?tso=127.0.0.1:1234
//
// The engine should be registered before creating storage.
func NewStore(path string) (kv.Storage, error) {
	return newStoreWithRetry(path, defaultMaxRetries)
}

func newStoreWithRetry(path string, maxRetries int) (kv.Storage, error) {
	url, err := url.Parse(path)
	if err != nil {
		return nil, errors.Trace(err)
	}

	name := strings.ToLower(url.Scheme)
	d, ok := stores[name]
	if !ok {
		return nil, errors.Errorf("invalid uri format, storage %s is not registered", name)
	}

	var s kv.Storage
	for i := 1; i <= maxRetries; i++ {
		s, err = d.Open(path)
		if err == nil || !kv.IsRetryableError(err) {
			break
		}
		sleepTime := time.Duration(uint64(retrySleepInterval) * uint64(i))
		log.Warnf("Waiting store to get ready, sleep %v and try again...", sleepTime)
		time.Sleep(sleepTime)
	}
	return s, errors.Trace(err)
}

var queryStmtTable = []string{"explain", "select", "show", "execute", "describe", "desc", "admin"}

func trimSQL(sql string) string {
	// Trim space.
	sql = strings.TrimSpace(sql)
	// Trim leading /*comment*/
	// There may be multiple comments
	for strings.HasPrefix(sql, "/*") {
		i := strings.Index(sql, "*/")
		if i != -1 && i < len(sql)+1 {
			sql = sql[i+2:]
			sql = strings.TrimSpace(sql)
			continue
		}
		break
	}
	// Trim leading '('. For `(select 1);` is also a query.
	return strings.TrimLeft(sql, "( ")
}

// IsQuery checks if a sql statement is a query statement.
func IsQuery(sql string) bool {
	sqlText := strings.ToLower(trimSQL(sql))
	for _, key := range queryStmtTable {
		if strings.HasPrefix(sqlText, key) {
			return true
		}
	}

	return false
}

func init() {
	// Register default memory and goleveldb storage
	RegisterLocalStore("memory", goleveldb.MemoryDriver{})
	RegisterLocalStore("goleveldb", goleveldb.Driver{})
	RegisterLocalStore("boltdb", boltdb.Driver{})
	RegisterStore("hbase", hbasekv.Driver{})

	// start pprof handlers
	if EnablePprof {
		go http.ListenAndServe(PprofAddr, nil)
	}
}
