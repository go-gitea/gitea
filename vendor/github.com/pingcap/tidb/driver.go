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

// database/sql/driver

package tidb

import (
	"database/sql"
	"database/sql/driver"
	"io"
	"net/url"
	"path/filepath"
	"strings"
	"sync"

	"github.com/juju/errors"
	"github.com/pingcap/tidb/ast"
	"github.com/pingcap/tidb/model"
	"github.com/pingcap/tidb/sessionctx"
	"github.com/pingcap/tidb/terror"
	"github.com/pingcap/tidb/util/types"
)

const (
	// DriverName is name of TiDB driver.
	DriverName = "tidb"
)

var (
	_ driver.Conn    = (*driverConn)(nil)
	_ driver.Execer  = (*driverConn)(nil)
	_ driver.Queryer = (*driverConn)(nil)
	_ driver.Tx      = (*driverConn)(nil)

	_ driver.Result = (*driverResult)(nil)
	_ driver.Rows   = (*driverRows)(nil)
	_ driver.Stmt   = (*driverStmt)(nil)
	_ driver.Driver = (*sqlDriver)(nil)

	txBeginSQL    = "BEGIN;"
	txCommitSQL   = "COMMIT;"
	txRollbackSQL = "ROLLBACK;"

	errNoResult = errors.New("query statement does not produce a result set (no top level SELECT)")
)

type errList []error

type driverParams struct {
	storePath string
	dbName    string
	// when set to true `mysql.Time` isn't encoded as string but passed as `time.Time`
	// this option is named for compatibility the same as in the mysql driver
	// while we actually do not have additional parsing to do
	parseTime bool
}

func (e *errList) append(err error) {
	if err != nil {
		*e = append(*e, err)
	}
}

func (e errList) error() error {
	if len(e) == 0 {
		return nil
	}

	return e
}

func (e errList) Error() string {
	a := make([]string, len(e))
	for i, v := range e {
		a[i] = v.Error()
	}
	return strings.Join(a, "\n")
}

func params(args []driver.Value) []interface{} {
	r := make([]interface{}, len(args))
	for i, v := range args {
		r[i] = interface{}(v)
	}
	return r
}

var (
	tidbDriver = &sqlDriver{}
	driverOnce sync.Once
)

// RegisterDriver registers TiDB driver.
// The name argument can be optionally prefixed by "engine://". In that case the
// prefix is recognized as a storage engine name.
//
// The name argument can be optionally prefixed by "memory://". In that case
// the prefix is stripped before interpreting it as a name of a memory-only,
// volatile DB.
//
//  [0]: http://golang.org/pkg/database/sql/driver/
func RegisterDriver() {
	driverOnce.Do(func() { sql.Register(DriverName, tidbDriver) })
}

// sqlDriver implements the interface required by database/sql/driver.
type sqlDriver struct {
	mu sync.Mutex
}

func (d *sqlDriver) lock() {
	d.mu.Lock()
}

func (d *sqlDriver) unlock() {
	d.mu.Unlock()
}

// parseDriverDSN cuts off DB name from dsn. It returns error if the dsn is not
// valid.
func parseDriverDSN(dsn string) (params *driverParams, err error) {
	u, err := url.Parse(dsn)
	if err != nil {
		return nil, errors.Trace(err)
	}
	path := filepath.Join(u.Host, u.Path)
	dbName := filepath.Clean(filepath.Base(path))
	if dbName == "" || dbName == "." || dbName == string(filepath.Separator) {
		return nil, errors.Errorf("invalid DB name %q", dbName)
	}
	// cut off dbName
	path = filepath.Clean(filepath.Dir(path))
	if path == "" || path == "." || path == string(filepath.Separator) {
		return nil, errors.Errorf("invalid dsn %q", dsn)
	}
	u.Path, u.Host = path, ""
	params = &driverParams{
		storePath: u.String(),
		dbName:    dbName,
	}
	// parse additional driver params
	query := u.Query()
	if parseTime := query.Get("parseTime"); parseTime == "true" {
		params.parseTime = true
	}

	return params, nil
}

// Open returns a new connection to the database.
//
// The dsn must be a URL format 'engine://path/dbname?params'.
// Engine is the storage name registered with RegisterStore.
// Path is the storage specific format.
// Params is key-value pairs split by '&', optional params are storage specific.
// Examples:
//    goleveldb://relative/path/test
//    boltdb:///absolute/path/test
//    hbase://zk1,zk2,zk3/hbasetbl/test?tso=zk
//
// Open may return a cached connection (one previously closed), but doing so is
// unnecessary; the sql package maintains a pool of idle connections for
// efficient re-use.
//
// The behavior of the mysql driver regarding time parsing can also be imitated
// by passing ?parseTime
//
// The returned connection is only used by one goroutine at a time.
func (d *sqlDriver) Open(dsn string) (driver.Conn, error) {
	params, err := parseDriverDSN(dsn)
	if err != nil {
		return nil, errors.Trace(err)
	}
	store, err := NewStore(params.storePath)
	if err != nil {
		return nil, errors.Trace(err)
	}

	sess, err := CreateSession(store)
	if err != nil {
		return nil, errors.Trace(err)
	}
	s := sess.(*session)

	d.lock()
	defer d.unlock()

	DBName := model.NewCIStr(params.dbName)
	domain := sessionctx.GetDomain(s)
	cs := &ast.CharsetOpt{
		Chs: "utf8",
		Col: "utf8_bin",
	}
	if !domain.InfoSchema().SchemaExists(DBName) {
		err = domain.DDL().CreateSchema(s, DBName, cs)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	driver := &sqlDriver{}
	return newDriverConn(s, driver, DBName.O, params)
}

// driverConn is a connection to a database. It is not used concurrently by
// multiple goroutines.
//
// Conn is assumed to be stateful.
type driverConn struct {
	s      Session
	driver *sqlDriver
	stmts  map[string]driver.Stmt
	params *driverParams
}

func newDriverConn(sess *session, d *sqlDriver, schema string, params *driverParams) (driver.Conn, error) {
	r := &driverConn{
		driver: d,
		stmts:  map[string]driver.Stmt{},
		s:      sess,
		params: params,
	}

	_, err := r.s.Execute("use " + schema)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return r, nil
}

// Prepare returns a prepared statement, bound to this connection.
func (c *driverConn) Prepare(query string) (driver.Stmt, error) {
	stmtID, paramCount, fields, err := c.s.PrepareStmt(query)
	if err != nil {
		return nil, err
	}
	s := &driverStmt{
		conn:       c,
		query:      query,
		stmtID:     stmtID,
		paramCount: paramCount,
		isQuery:    fields != nil,
	}
	c.stmts[query] = s
	return s, nil
}

// Close invalidates and potentially stops any current prepared statements and
// transactions, marking this connection as no longer in use.
//
// Because the sql package maintains a free pool of connections and only calls
// Close when there's a surplus of idle connections, it shouldn't be necessary
// for drivers to do their own connection caching.
func (c *driverConn) Close() error {
	var err errList
	for _, s := range c.stmts {
		stmt := s.(*driverStmt)
		err.append(stmt.conn.s.DropPreparedStmt(stmt.stmtID))
	}

	c.driver.lock()
	defer c.driver.unlock()

	return err.error()
}

// Begin starts and returns a new transaction.
func (c *driverConn) Begin() (driver.Tx, error) {
	if c.s == nil {
		return nil, errors.Errorf("Need init first")
	}

	if _, err := c.s.Execute(txBeginSQL); err != nil {
		return nil, errors.Trace(err)
	}

	return c, nil
}

func (c *driverConn) Commit() error {
	if c.s == nil {
		return terror.CommitNotInTransaction
	}
	_, err := c.s.Execute(txCommitSQL)

	if err != nil {
		return errors.Trace(err)
	}

	err = c.s.FinishTxn(false)
	return errors.Trace(err)
}

func (c *driverConn) Rollback() error {
	if c.s == nil {
		return terror.RollbackNotInTransaction
	}

	if _, err := c.s.Execute(txRollbackSQL); err != nil {
		return errors.Trace(err)
	}

	return nil
}

// Execer is an optional interface that may be implemented by a Conn.
//
// If a Conn does not implement Execer, the sql package's DB.Exec will first
// prepare a query, execute the statement, and then close the statement.
//
// Exec may return driver.ErrSkip.
func (c *driverConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	return c.driverExec(query, args)

}

func (c *driverConn) getStmt(query string) (stmt driver.Stmt, err error) {
	stmt, ok := c.stmts[query]
	if !ok {
		stmt, err = c.Prepare(query)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	return
}

func (c *driverConn) driverExec(query string, args []driver.Value) (driver.Result, error) {
	if len(args) == 0 {
		if _, err := c.s.Execute(query); err != nil {
			return nil, errors.Trace(err)
		}
		r := &driverResult{}
		r.lastInsertID, r.rowsAffected = int64(c.s.LastInsertID()), int64(c.s.AffectedRows())
		return r, nil
	}
	stmt, err := c.getStmt(query)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return stmt.Exec(args)
}

// Queryer is an optional interface that may be implemented by a Conn.
//
// If a Conn does not implement Queryer, the sql package's DB.Query will first
// prepare a query, execute the statement, and then close the statement.
//
// Query may return driver.ErrSkip.
func (c *driverConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	return c.driverQuery(query, args)
}

func (c *driverConn) driverQuery(query string, args []driver.Value) (driver.Rows, error) {
	if len(args) == 0 {
		rss, err := c.s.Execute(query)
		if err != nil {
			return nil, errors.Trace(err)
		}
		if len(rss) == 0 {
			return nil, errors.Trace(errNoResult)
		}
		return &driverRows{params: c.params, rs: rss[0]}, nil
	}
	stmt, err := c.getStmt(query)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return stmt.Query(args)
}

// driverResult is the result of a query execution.
type driverResult struct {
	lastInsertID int64
	rowsAffected int64
}

// LastInsertID returns the database's auto-generated ID after, for example, an
// INSERT into a table with primary key.
func (r *driverResult) LastInsertId() (int64, error) { // -golint
	return r.lastInsertID, nil
}

// RowsAffected returns the number of rows affected by the query.
func (r *driverResult) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
}

// driverRows is an iterator over an executed query's results.
type driverRows struct {
	rs     ast.RecordSet
	params *driverParams
}

// Columns returns the names of the columns. The number of columns of the
// result is inferred from the length of the slice.  If a particular column
// name isn't known, an empty string should be returned for that entry.
func (r *driverRows) Columns() []string {
	if r.rs == nil {
		return []string{}
	}
	fs, _ := r.rs.Fields()
	names := make([]string, len(fs))
	for i, f := range fs {
		names[i] = f.ColumnAsName.O
	}
	return names
}

// Close closes the rows iterator.
func (r *driverRows) Close() error {
	if r.rs != nil {
		return r.rs.Close()
	}
	return nil
}

// Next is called to populate the next row of data into the provided slice. The
// provided slice will be the same size as the Columns() are wide.
//
// The dest slice may be populated only with a driver Value type, but excluding
// string.  All string values must be converted to []byte.
//
// Next should return io.EOF when there are no more rows.
func (r *driverRows) Next(dest []driver.Value) error {
	if r.rs == nil {
		return io.EOF
	}
	row, err := r.rs.Next()
	if err != nil {
		return errors.Trace(err)
	}
	if row == nil {
		return io.EOF
	}
	if len(row.Data) != len(dest) {
		return errors.Errorf("field count mismatch: got %d, need %d", len(row.Data), len(dest))
	}
	for i, xi := range row.Data {
		switch xi.Kind() {
		case types.KindNull:
			dest[i] = nil
		case types.KindInt64:
			dest[i] = xi.GetInt64()
		case types.KindUint64:
			dest[i] = xi.GetUint64()
		case types.KindFloat32:
			dest[i] = xi.GetFloat32()
		case types.KindFloat64:
			dest[i] = xi.GetFloat64()
		case types.KindString:
			dest[i] = xi.GetString()
		case types.KindBytes:
			dest[i] = xi.GetBytes()
		case types.KindMysqlBit:
			dest[i] = xi.GetMysqlBit().ToString()
		case types.KindMysqlDecimal:
			dest[i] = xi.GetMysqlDecimal().String()
		case types.KindMysqlDuration:
			dest[i] = xi.GetMysqlDuration().String()
		case types.KindMysqlEnum:
			dest[i] = xi.GetMysqlEnum().String()
		case types.KindMysqlHex:
			dest[i] = xi.GetMysqlHex().ToString()
		case types.KindMysqlSet:
			dest[i] = xi.GetMysqlSet().String()
		case types.KindMysqlTime:
			t := xi.GetMysqlTime()
			if !r.params.parseTime {
				dest[i] = t.String()
			} else {
				dest[i] = t.Time
			}
		default:
			return errors.Errorf("unable to handle type %T", xi.GetValue())
		}
	}
	return nil
}

// driverStmt is a prepared statement. It is bound to a driverConn and not used
// by multiple goroutines concurrently.
type driverStmt struct {
	conn       *driverConn
	query      string
	stmtID     uint32
	paramCount int
	isQuery    bool
}

// Close closes the statement.
//
// As of Go 1.1, a Stmt will not be closed if it's in use by any queries.
func (s *driverStmt) Close() error {
	s.conn.s.DropPreparedStmt(s.stmtID)
	delete(s.conn.stmts, s.query)
	return nil
}

// NumInput returns the number of placeholder parameters.
//
// If NumInput returns >= 0, the sql package will sanity check argument counts
// from callers and return errors to the caller before the statement's Exec or
// Query methods are called.
//
// NumInput may also return -1, if the driver doesn't know its number of
// placeholders. In that case, the sql package will not sanity check Exec or
// Query argument counts.
func (s *driverStmt) NumInput() int {
	return s.paramCount
}

// Exec executes a query that doesn't return rows, such as an INSERT or UPDATE.
func (s *driverStmt) Exec(args []driver.Value) (driver.Result, error) {
	c := s.conn
	_, err := c.s.ExecutePreparedStmt(s.stmtID, params(args)...)
	if err != nil {
		return nil, errors.Trace(err)
	}
	r := &driverResult{}
	if s != nil {
		r.lastInsertID, r.rowsAffected = int64(c.s.LastInsertID()), int64(c.s.AffectedRows())
	}
	return r, nil
}

// Exec executes a query that may return rows, such as a SELECT.
func (s *driverStmt) Query(args []driver.Value) (driver.Rows, error) {
	c := s.conn
	rs, err := c.s.ExecutePreparedStmt(s.stmtID, params(args)...)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if rs == nil {
		if s.isQuery {
			return nil, errors.Trace(errNoResult)
		}
		// The statement is not a query.
		return &driverRows{}, nil
	}
	return &driverRows{params: s.conn.params, rs: rs}, nil
}

func init() {
	RegisterDriver()
}
