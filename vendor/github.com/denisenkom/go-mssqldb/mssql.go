package mssql

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"reflect"
	"strings"
	"time"
)

var driverInstance = &Driver{processQueryText: true}
var driverInstanceNoProcess = &Driver{processQueryText: false}

func init() {
	sql.Register("mssql", driverInstance)
	sql.Register("sqlserver", driverInstanceNoProcess)
	createDialer = func(p *connectParams) dialer {
		return tcpDialer{&net.Dialer{Timeout: p.dial_timeout, KeepAlive: p.keepAlive}}
	}
}

// Abstract the dialer for testing and for non-TCP based connections.
type dialer interface {
	Dial(ctx context.Context, addr string) (net.Conn, error)
}

var createDialer func(p *connectParams) dialer

type tcpDialer struct {
	nd *net.Dialer
}

func (d tcpDialer) Dial(ctx context.Context, addr string) (net.Conn, error) {
	return d.nd.DialContext(ctx, "tcp", addr)
}

type Driver struct {
	log optionalLogger

	processQueryText bool
}

// OpenConnector opens a new connector. Useful to dial with a context.
func (d *Driver) OpenConnector(dsn string) (*Connector, error) {
	params, err := parseConnectParams(dsn)
	if err != nil {
		return nil, err
	}
	return &Connector{
		params: params,
		driver: d,
	}, nil
}

func (d *Driver) Open(dsn string) (driver.Conn, error) {
	return d.open(context.Background(), dsn)
}

// Connector holds the parsed DSN and is ready to make a new connection
// at any time.
//
// In the future, settings that cannot be passed through a string DSN
// may be set directly on the connector.
type Connector struct {
	params connectParams
	driver *Driver
}

// Connect to the server and return a TDS connection.
func (c *Connector) Connect(ctx context.Context) (driver.Conn, error) {
	return c.driver.connect(ctx, c.params)
}

// Driver underlying the Connector.
func (c *Connector) Driver() driver.Driver {
	return c.driver
}

func SetLogger(logger Logger) {
	driverInstance.SetLogger(logger)
	driverInstanceNoProcess.SetLogger(logger)
}

func (d *Driver) SetLogger(logger Logger) {
	d.log = optionalLogger{logger}
}

type Conn struct {
	sess           *tdsSession
	transactionCtx context.Context

	processQueryText bool
	connectionGood   bool

	outs map[string]interface{}
}

func (c *Conn) checkBadConn(err error) error {
	// this is a hack to address Issue #275
	// we set connectionGood flag to false if
	// error indicates that connection is not usable
	// but we return actual error instead of ErrBadConn
	// this will cause connection to stay in a pool
	// but next request to this connection will return ErrBadConn

	// it might be possible to revise this hack after
	// https://github.com/golang/go/issues/20807
	// is implemented
	switch err {
	case nil:
		return nil
	case io.EOF:
		return driver.ErrBadConn
	case driver.ErrBadConn:
		// It is an internal programming error if driver.ErrBadConn
		// is ever passed to this function. driver.ErrBadConn should
		// only ever be returned in response to a *mssql.Conn.connectionGood == false
		// check in the external facing API.
		panic("driver.ErrBadConn in checkBadConn. This should not happen.")
	}

	switch err.(type) {
	case net.Error:
		c.connectionGood = false
		return err
	case StreamError:
		c.connectionGood = false
		return err
	default:
		return err
	}
}

func (c *Conn) clearOuts() {
	c.outs = nil
}

func (c *Conn) simpleProcessResp(ctx context.Context) error {
	tokchan := make(chan tokenStruct, 5)
	go processResponse(ctx, c.sess, tokchan, c.outs)
	c.clearOuts()
	for tok := range tokchan {
		switch token := tok.(type) {
		case doneStruct:
			if token.isError() {
				return c.checkBadConn(token.getError())
			}
		case error:
			return c.checkBadConn(token)
		}
	}
	return nil
}

func (c *Conn) Commit() error {
	if !c.connectionGood {
		return driver.ErrBadConn
	}
	if err := c.sendCommitRequest(); err != nil {
		return c.checkBadConn(err)
	}
	return c.simpleProcessResp(c.transactionCtx)
}

func (c *Conn) sendCommitRequest() error {
	headers := []headerStruct{
		{hdrtype: dataStmHdrTransDescr,
			data: transDescrHdr{c.sess.tranid, 1}.pack()},
	}
	if err := sendCommitXact(c.sess.buf, headers, "", 0, 0, ""); err != nil {
		if c.sess.logFlags&logErrors != 0 {
			c.sess.log.Printf("Failed to send CommitXact with %v", err)
		}
		c.connectionGood = false
		return fmt.Errorf("Faild to send CommitXact: %v", err)
	}
	return nil
}

func (c *Conn) Rollback() error {
	if !c.connectionGood {
		return driver.ErrBadConn
	}
	if err := c.sendRollbackRequest(); err != nil {
		return c.checkBadConn(err)
	}
	return c.simpleProcessResp(c.transactionCtx)
}

func (c *Conn) sendRollbackRequest() error {
	headers := []headerStruct{
		{hdrtype: dataStmHdrTransDescr,
			data: transDescrHdr{c.sess.tranid, 1}.pack()},
	}
	if err := sendRollbackXact(c.sess.buf, headers, "", 0, 0, ""); err != nil {
		if c.sess.logFlags&logErrors != 0 {
			c.sess.log.Printf("Failed to send RollbackXact with %v", err)
		}
		c.connectionGood = false
		return fmt.Errorf("Failed to send RollbackXact: %v", err)
	}
	return nil
}

func (c *Conn) Begin() (driver.Tx, error) {
	return c.begin(context.Background(), isolationUseCurrent)
}

func (c *Conn) begin(ctx context.Context, tdsIsolation isoLevel) (tx driver.Tx, err error) {
	if !c.connectionGood {
		return nil, driver.ErrBadConn
	}
	err = c.sendBeginRequest(ctx, tdsIsolation)
	if err != nil {
		return nil, c.checkBadConn(err)
	}
	tx, err = c.processBeginResponse(ctx)
	if err != nil {
		return nil, c.checkBadConn(err)
	}
	return
}

func (c *Conn) sendBeginRequest(ctx context.Context, tdsIsolation isoLevel) error {
	c.transactionCtx = ctx
	headers := []headerStruct{
		{hdrtype: dataStmHdrTransDescr,
			data: transDescrHdr{0, 1}.pack()},
	}
	if err := sendBeginXact(c.sess.buf, headers, tdsIsolation, ""); err != nil {
		if c.sess.logFlags&logErrors != 0 {
			c.sess.log.Printf("Failed to send BeginXact with %v", err)
		}
		c.connectionGood = false
		return fmt.Errorf("Failed to send BiginXant: %v", err)
	}
	return nil
}

func (c *Conn) processBeginResponse(ctx context.Context) (driver.Tx, error) {
	if err := c.simpleProcessResp(ctx); err != nil {
		return nil, err
	}
	// successful BEGINXACT request will return sess.tranid
	// for started transaction
	return c, nil
}

func (d *Driver) open(ctx context.Context, dsn string) (*Conn, error) {
	params, err := parseConnectParams(dsn)
	if err != nil {
		return nil, err
	}
	return d.connect(ctx, params)
}

// connect to the server, using the provided context for dialing only.
func (d *Driver) connect(ctx context.Context, params connectParams) (*Conn, error) {
	sess, err := connect(ctx, d.log, params)
	if err != nil {
		// main server failed, try fail-over partner
		if params.failOverPartner == "" {
			return nil, err
		}

		params.host = params.failOverPartner
		if params.failOverPort != 0 {
			params.port = params.failOverPort
		}

		sess, err = connect(ctx, d.log, params)
		if err != nil {
			// fail-over partner also failed, now fail
			return nil, err
		}
	}

	conn := &Conn{
		sess:             sess,
		transactionCtx:   context.Background(),
		processQueryText: d.processQueryText,
		connectionGood:   true,
	}
	conn.sess.log = d.log
	return conn, nil
}

func (c *Conn) Close() error {
	return c.sess.buf.transport.Close()
}

type Stmt struct {
	c          *Conn
	query      string
	paramCount int
	notifSub   *queryNotifSub
}

type queryNotifSub struct {
	msgText string
	options string
	timeout uint32
}

func (c *Conn) Prepare(query string) (driver.Stmt, error) {
	if !c.connectionGood {
		return nil, driver.ErrBadConn
	}
	if len(query) > 10 && strings.EqualFold(query[:10], "INSERTBULK") {
		return c.prepareCopyIn(query)
	}

	return c.prepareContext(context.Background(), query)
}

func (c *Conn) prepareContext(ctx context.Context, query string) (*Stmt, error) {
	paramCount := -1
	if c.processQueryText {
		query, paramCount = parseParams(query)
	}
	return &Stmt{c, query, paramCount, nil}, nil
}

func (s *Stmt) Close() error {
	return nil
}

func (s *Stmt) SetQueryNotification(id, options string, timeout time.Duration) {
	to := uint32(timeout / time.Second)
	if to < 1 {
		to = 1
	}
	s.notifSub = &queryNotifSub{id, options, to}
}

func (s *Stmt) NumInput() int {
	return s.paramCount
}

func (s *Stmt) sendQuery(args []namedValue) (err error) {
	headers := []headerStruct{
		{hdrtype: dataStmHdrTransDescr,
			data: transDescrHdr{s.c.sess.tranid, 1}.pack()},
	}

	if s.notifSub != nil {
		headers = append(headers,
			headerStruct{
				hdrtype: dataStmHdrQueryNotif,
				data: queryNotifHdr{
					s.notifSub.msgText,
					s.notifSub.options,
					s.notifSub.timeout,
				}.pack(),
			})
	}

	// no need to check number of parameters here, it is checked by database/sql
	if s.c.sess.logFlags&logSQL != 0 {
		s.c.sess.log.Println(s.query)
	}
	if s.c.sess.logFlags&logParams != 0 && len(args) > 0 {
		for i := 0; i < len(args); i++ {
			if len(args[i].Name) > 0 {
				s.c.sess.log.Printf("\t@%s\t%v\n", args[i].Name, args[i].Value)
			} else {
				s.c.sess.log.Printf("\t@p%d\t%v\n", i+1, args[i].Value)
			}
		}

	}
	if len(args) == 0 {
		if err = sendSqlBatch72(s.c.sess.buf, s.query, headers); err != nil {
			if s.c.sess.logFlags&logErrors != 0 {
				s.c.sess.log.Printf("Failed to send SqlBatch with %v", err)
			}
			s.c.connectionGood = false
			return fmt.Errorf("failed to send SQL Batch: %v", err)
		}
	} else {
		proc := Sp_ExecuteSql
		var params []Param
		if isProc(s.query) {
			proc.name = s.query
			params, _, err = s.makeRPCParams(args, 0)
		} else {
			var decls []string
			params, decls, err = s.makeRPCParams(args, 2)
			if err != nil {
				return
			}
			params[0] = makeStrParam(s.query)
			params[1] = makeStrParam(strings.Join(decls, ","))
		}
		if err = sendRpc(s.c.sess.buf, headers, proc, 0, params); err != nil {
			if s.c.sess.logFlags&logErrors != 0 {
				s.c.sess.log.Printf("Failed to send Rpc with %v", err)
			}
			s.c.connectionGood = false
			return fmt.Errorf("Failed to send RPC: %v", err)
		}
	}
	return
}

// isProc takes the query text in s and determines if it is a stored proc name
// or SQL text.
func isProc(s string) bool {
	if len(s) == 0 {
		return false
	}
	if s[0] == '[' && s[len(s)-1] == ']' && strings.ContainsAny(s, "\n\r") == false {
		return true
	}
	return !strings.ContainsAny(s, " \t\n\r;")
}

func (s *Stmt) makeRPCParams(args []namedValue, offset int) ([]Param, []string, error) {
	var err error
	params := make([]Param, len(args)+offset)
	decls := make([]string, len(args))
	for i, val := range args {
		params[i+offset], err = s.makeParam(val.Value)
		if err != nil {
			return nil, nil, err
		}
		var name string
		if len(val.Name) > 0 {
			name = "@" + val.Name
		} else {
			name = fmt.Sprintf("@p%d", val.Ordinal)
		}
		params[i+offset].Name = name
		decls[i] = fmt.Sprintf("%s %s", name, makeDecl(params[i+offset].ti))
	}
	return params, decls, nil
}

type namedValue struct {
	Name    string
	Ordinal int
	Value   driver.Value
}

func convertOldArgs(args []driver.Value) []namedValue {
	list := make([]namedValue, len(args))
	for i, v := range args {
		list[i] = namedValue{
			Ordinal: i + 1,
			Value:   v,
		}
	}
	return list
}

func (s *Stmt) Query(args []driver.Value) (driver.Rows, error) {
	return s.queryContext(context.Background(), convertOldArgs(args))
}

func (s *Stmt) queryContext(ctx context.Context, args []namedValue) (rows driver.Rows, err error) {
	if !s.c.connectionGood {
		return nil, driver.ErrBadConn
	}
	if err = s.sendQuery(args); err != nil {
		return nil, s.c.checkBadConn(err)
	}
	return s.processQueryResponse(ctx)
}

func (s *Stmt) processQueryResponse(ctx context.Context) (res driver.Rows, err error) {
	tokchan := make(chan tokenStruct, 5)
	ctx, cancel := context.WithCancel(ctx)
	go processResponse(ctx, s.c.sess, tokchan, s.c.outs)
	s.c.clearOuts()
	// process metadata
	var cols []columnStruct
loop:
	for tok := range tokchan {
		switch token := tok.(type) {
		// By ignoring DONE token we effectively
		// skip empty result-sets.
		// This improves results in queries like that:
		// set nocount on; select 1
		// see TestIgnoreEmptyResults test
		//case doneStruct:
		//break loop
		case []columnStruct:
			cols = token
			break loop
		case doneStruct:
			if token.isError() {
				return nil, s.c.checkBadConn(token.getError())
			}
		case error:
			return nil, s.c.checkBadConn(token)
		}
	}
	res = &Rows{stmt: s, tokchan: tokchan, cols: cols, cancel: cancel}
	return
}

func (s *Stmt) Exec(args []driver.Value) (driver.Result, error) {
	return s.exec(context.Background(), convertOldArgs(args))
}

func (s *Stmt) exec(ctx context.Context, args []namedValue) (res driver.Result, err error) {
	if !s.c.connectionGood {
		return nil, driver.ErrBadConn
	}
	if err = s.sendQuery(args); err != nil {
		return nil, s.c.checkBadConn(err)
	}
	if res, err = s.processExec(ctx); err != nil {
		return nil, s.c.checkBadConn(err)
	}
	return
}

func (s *Stmt) processExec(ctx context.Context) (res driver.Result, err error) {
	tokchan := make(chan tokenStruct, 5)
	go processResponse(ctx, s.c.sess, tokchan, s.c.outs)
	s.c.clearOuts()
	var rowCount int64
	for token := range tokchan {
		switch token := token.(type) {
		case doneInProcStruct:
			if token.Status&doneCount != 0 {
				rowCount += int64(token.RowCount)
			}
		case doneStruct:
			if token.Status&doneCount != 0 {
				rowCount += int64(token.RowCount)
			}
			if token.isError() {
				return nil, token.getError()
			}
		case error:
			return nil, token
		}
	}
	return &Result{s.c, rowCount}, nil
}

type Rows struct {
	stmt    *Stmt
	cols    []columnStruct
	tokchan chan tokenStruct

	nextCols []columnStruct

	cancel func()
}

func (rc *Rows) Close() error {
	rc.cancel()
	for _ = range rc.tokchan {
	}
	rc.tokchan = nil
	return nil
}

func (rc *Rows) Columns() (res []string) {
	res = make([]string, len(rc.cols))
	for i, col := range rc.cols {
		res[i] = col.ColName
	}
	return
}

func (rc *Rows) Next(dest []driver.Value) error {
	if !rc.stmt.c.connectionGood {
		return driver.ErrBadConn
	}
	if rc.nextCols != nil {
		return io.EOF
	}
	for tok := range rc.tokchan {
		switch tokdata := tok.(type) {
		case []columnStruct:
			rc.nextCols = tokdata
			return io.EOF
		case []interface{}:
			for i := range dest {
				dest[i] = tokdata[i]
			}
			return nil
		case doneStruct:
			if tokdata.isError() {
				return rc.stmt.c.checkBadConn(tokdata.getError())
			}
		case error:
			return rc.stmt.c.checkBadConn(tokdata)
		}
	}
	return io.EOF
}

func (rc *Rows) HasNextResultSet() bool {
	return rc.nextCols != nil
}

func (rc *Rows) NextResultSet() error {
	rc.cols = rc.nextCols
	rc.nextCols = nil
	if rc.cols == nil {
		return io.EOF
	}
	return nil
}

// It should return
// the value type that can be used to scan types into. For example, the database
// column type "bigint" this should return "reflect.TypeOf(int64(0))".
func (r *Rows) ColumnTypeScanType(index int) reflect.Type {
	return makeGoLangScanType(r.cols[index].ti)
}

// RowsColumnTypeDatabaseTypeName may be implemented by Rows. It should return the
// database system type name without the length. Type names should be uppercase.
// Examples of returned types: "VARCHAR", "NVARCHAR", "VARCHAR2", "CHAR", "TEXT",
// "DECIMAL", "SMALLINT", "INT", "BIGINT", "BOOL", "[]BIGINT", "JSONB", "XML",
// "TIMESTAMP".
func (r *Rows) ColumnTypeDatabaseTypeName(index int) string {
	return makeGoLangTypeName(r.cols[index].ti)
}

// RowsColumnTypeLength may be implemented by Rows. It should return the length
// of the column type if the column is a variable length type. If the column is
// not a variable length type ok should return false.
// If length is not limited other than system limits, it should return math.MaxInt64.
// The following are examples of returned values for various types:
//   TEXT          (math.MaxInt64, true)
//   varchar(10)   (10, true)
//   nvarchar(10)  (10, true)
//   decimal       (0, false)
//   int           (0, false)
//   bytea(30)     (30, true)
func (r *Rows) ColumnTypeLength(index int) (int64, bool) {
	return makeGoLangTypeLength(r.cols[index].ti)
}

// It should return
// the precision and scale for decimal types. If not applicable, ok should be false.
// The following are examples of returned values for various types:
//   decimal(38, 4)    (38, 4, true)
//   int               (0, 0, false)
//   decimal           (math.MaxInt64, math.MaxInt64, true)
func (r *Rows) ColumnTypePrecisionScale(index int) (int64, int64, bool) {
	return makeGoLangTypePrecisionScale(r.cols[index].ti)
}

// The nullable value should
// be true if it is known the column may be null, or false if the column is known
// to be not nullable.
// If the column nullability is unknown, ok should be false.
func (r *Rows) ColumnTypeNullable(index int) (nullable, ok bool) {
	nullable = r.cols[index].Flags&colFlagNullable != 0
	ok = true
	return
}

func makeStrParam(val string) (res Param) {
	res.ti.TypeId = typeNVarChar
	res.buffer = str2ucs2(val)
	res.ti.Size = len(res.buffer)
	return
}

func (s *Stmt) makeParam(val driver.Value) (res Param, err error) {
	if val == nil {
		res.ti.TypeId = typeNull
		res.buffer = nil
		res.ti.Size = 0
		return
	}
	switch val := val.(type) {
	case int64:
		res.ti.TypeId = typeIntN
		res.buffer = make([]byte, 8)
		res.ti.Size = 8
		binary.LittleEndian.PutUint64(res.buffer, uint64(val))
	case float64:
		res.ti.TypeId = typeFltN
		res.ti.Size = 8
		res.buffer = make([]byte, 8)
		binary.LittleEndian.PutUint64(res.buffer, math.Float64bits(val))
	case []byte:
		res.ti.TypeId = typeBigVarBin
		res.ti.Size = len(val)
		res.buffer = val
	case string:
		res = makeStrParam(val)
	case bool:
		res.ti.TypeId = typeBitN
		res.ti.Size = 1
		res.buffer = make([]byte, 1)
		if val {
			res.buffer[0] = 1
		}
	case time.Time:
		if s.c.sess.loginAck.TDSVersion >= verTDS73 {
			res.ti.TypeId = typeDateTimeOffsetN
			res.ti.Scale = 7
			res.ti.Size = 10
			buf := make([]byte, 10)
			res.buffer = buf
			days, ns := dateTime2(val)
			ns /= 100
			buf[0] = byte(ns)
			buf[1] = byte(ns >> 8)
			buf[2] = byte(ns >> 16)
			buf[3] = byte(ns >> 24)
			buf[4] = byte(ns >> 32)
			buf[5] = byte(days)
			buf[6] = byte(days >> 8)
			buf[7] = byte(days >> 16)
			_, offset := val.Zone()
			offset /= 60
			buf[8] = byte(offset)
			buf[9] = byte(offset >> 8)
		} else {
			res.ti.TypeId = typeDateTimeN
			res.ti.Size = 8
			res.buffer = make([]byte, 8)
			ref := time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC)
			dur := val.Sub(ref)
			days := dur / (24 * time.Hour)
			tm := (300 * (dur % (24 * time.Hour))) / time.Second
			binary.LittleEndian.PutUint32(res.buffer[0:4], uint32(days))
			binary.LittleEndian.PutUint32(res.buffer[4:8], uint32(tm))
		}
	default:
		return s.makeParamExtra(val)
	}
	return
}

type Result struct {
	c            *Conn
	rowsAffected int64
}

func (r *Result) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
}

func (r *Result) LastInsertId() (int64, error) {
	s, err := r.c.Prepare("select cast(@@identity as bigint)")
	if err != nil {
		return 0, err
	}
	defer s.Close()
	rows, err := s.Query(nil)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	dest := make([]driver.Value, 1)
	err = rows.Next(dest)
	if err != nil {
		return 0, err
	}
	if dest[0] == nil {
		return -1, errors.New("There is no generated identity value")
	}
	lastInsertId := dest[0].(int64)
	return lastInsertId, nil
}
