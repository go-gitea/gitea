package mssql

import (
	"database/sql"
	"database/sql/driver"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"strings"
	"time"
)

func init() {
	sql.Register("mssql", &MssqlDriver{})
}

type MssqlDriver struct {
	log *log.Logger
}

func (d *MssqlDriver) SetLogger(logger *log.Logger) {
	d.log = logger
}

func CheckBadConn(err error) error {
	if err == io.EOF {
		return driver.ErrBadConn
	}

	switch e := err.(type) {
	case net.Error:
		if e.Timeout() {
			return e
		}
		return driver.ErrBadConn
	default:
		return err
	}
}

type MssqlConn struct {
	sess *tdsSession
}

func (c *MssqlConn) Commit() error {
	headers := []headerStruct{
		{hdrtype: dataStmHdrTransDescr,
			data: transDescrHdr{c.sess.tranid, 1}.pack()},
	}
	if err := sendCommitXact(c.sess.buf, headers, "", 0, 0, ""); err != nil {
		return err
	}

	tokchan := make(chan tokenStruct, 5)
	go processResponse(c.sess, tokchan)
	for tok := range tokchan {
		switch token := tok.(type) {
		case error:
			return token
		}
	}
	return nil
}

func (c *MssqlConn) Rollback() error {
	headers := []headerStruct{
		{hdrtype: dataStmHdrTransDescr,
			data: transDescrHdr{c.sess.tranid, 1}.pack()},
	}
	if err := sendRollbackXact(c.sess.buf, headers, "", 0, 0, ""); err != nil {
		return err
	}

	tokchan := make(chan tokenStruct, 5)
	go processResponse(c.sess, tokchan)
	for tok := range tokchan {
		switch token := tok.(type) {
		case error:
			return token
		}
	}
	return nil
}

func (c *MssqlConn) Begin() (driver.Tx, error) {
	headers := []headerStruct{
		{hdrtype: dataStmHdrTransDescr,
			data: transDescrHdr{0, 1}.pack()},
	}
	if err := sendBeginXact(c.sess.buf, headers, 0, ""); err != nil {
		return nil, CheckBadConn(err)
	}
	tokchan := make(chan tokenStruct, 5)
	go processResponse(c.sess, tokchan)
	for tok := range tokchan {
		switch token := tok.(type) {
		case error:
			if c.sess.tranid != 0 {
				return nil, token
			}
			return nil, CheckBadConn(token)
		}
	}
	// successful BEGINXACT request will return sess.tranid
	// for started transaction
	return c, nil
}

func (d *MssqlDriver) Open(dsn string) (driver.Conn, error) {
	params, err := parseConnectParams(dsn)
	if err != nil {
		return nil, err
	}

	sess, err := connect(params)
	if err != nil {
		// main server failed, try fail-over partner
		if params.failOverPartner == "" {
			return nil, err
		}

		params.host = params.failOverPartner
		if params.failOverPort != 0 {
			params.port = params.failOverPort
		}

		sess, err = connect(params)
		if err != nil {
			// fail-over partner also failed, now fail
			return nil, err
		}
	}

	conn := &MssqlConn{sess}
	conn.sess.log = (*Logger)(d.log)
	return conn, nil
}

func (c *MssqlConn) Close() error {
	return c.sess.buf.transport.Close()
}

type MssqlStmt struct {
	c          *MssqlConn
	query      string
	paramCount int
	notifSub   *queryNotifSub
}

type queryNotifSub struct {
	msgText string
	options string
	timeout uint32
}

func (c *MssqlConn) Prepare(query string) (driver.Stmt, error) {
	q, paramCount := parseParams(query)
	return &MssqlStmt{c, q, paramCount, nil}, nil
}

func (s *MssqlStmt) Close() error {
	return nil
}

func (s *MssqlStmt) SetQueryNotification(id, options string, timeout time.Duration) {
	to := uint32(timeout / time.Second)
	if to < 1 {
		to = 1
	}
	s.notifSub = &queryNotifSub{id, options, to}
}

func (s *MssqlStmt) NumInput() int {
	return s.paramCount
}

func (s *MssqlStmt) sendQuery(args []driver.Value) (err error) {
	headers := []headerStruct{
		{hdrtype: dataStmHdrTransDescr,
			data: transDescrHdr{s.c.sess.tranid, 1}.pack()},
	}

	if s.notifSub != nil {
		headers = append(headers, headerStruct{hdrtype: dataStmHdrQueryNotif,
			data: queryNotifHdr{s.notifSub.msgText, s.notifSub.options, s.notifSub.timeout}.pack()})
	}

	if len(args) != s.paramCount {
		return errors.New(fmt.Sprintf("sql: expected %d parameters, got %d", s.paramCount, len(args)))
	}
	if s.c.sess.logFlags&logSQL != 0 {
		s.c.sess.log.Println(s.query)
	}
	if s.c.sess.logFlags&logParams != 0 && len(args) > 0 {
		for i := 0; i < len(args); i++ {
			s.c.sess.log.Printf("\t@p%d\t%v\n", i+1, args[i])
		}

	}
	if len(args) == 0 {
		if err = sendSqlBatch72(s.c.sess.buf, s.query, headers); err != nil {
			if s.c.sess.tranid != 0 {
				return err
			}
			return CheckBadConn(err)
		}
	} else {
		params := make([]Param, len(args)+2)
		decls := make([]string, len(args))
		params[0], err = s.makeParam(s.query)
		if err != nil {
			return
		}
		for i, val := range args {
			params[i+2], err = s.makeParam(val)
			if err != nil {
				return
			}
			name := fmt.Sprintf("@p%d", i+1)
			params[i+2].Name = name
			decls[i] = fmt.Sprintf("%s %s", name, makeDecl(params[i+2].ti))
		}
		params[1], err = s.makeParam(strings.Join(decls, ","))
		if err != nil {
			return
		}
		if err = sendRpc(s.c.sess.buf, headers, Sp_ExecuteSql, 0, params); err != nil {
			if s.c.sess.tranid != 0 {
				return err
			}
			return CheckBadConn(err)
		}
	}
	return
}

func (s *MssqlStmt) Query(args []driver.Value) (res driver.Rows, err error) {
	if err = s.sendQuery(args); err != nil {
		return
	}
	tokchan := make(chan tokenStruct, 5)
	go processResponse(s.c.sess, tokchan)
	// process metadata
	var cols []string
loop:
	for tok := range tokchan {
		switch token := tok.(type) {
		// by ignoring DONE token we effectively
		// skip empty result-sets
		// this improves results in queryes like that:
		// set nocount on; select 1
		// see TestIgnoreEmptyResults test
		//case doneStruct:
		//break loop
		case []columnStruct:
			cols = make([]string, len(token))
			for i, col := range token {
				cols[i] = col.ColName
			}
			break loop
		case error:
			if s.c.sess.tranid != 0 {
				return nil, token
			}
			return nil, CheckBadConn(token)
		}
	}
	return &MssqlRows{sess: s.c.sess, tokchan: tokchan, cols: cols}, nil
}

func (s *MssqlStmt) Exec(args []driver.Value) (res driver.Result, err error) {
	if err = s.sendQuery(args); err != nil {
		return
	}
	tokchan := make(chan tokenStruct, 5)
	go processResponse(s.c.sess, tokchan)
	var rowCount int64
	for token := range tokchan {
		switch token := token.(type) {
		case doneInProcStruct:
			if token.Status&doneCount != 0 {
				rowCount = int64(token.RowCount)
			}
		case doneStruct:
			if token.Status&doneCount != 0 {
				rowCount = int64(token.RowCount)
			}
		case error:
			if s.c.sess.logFlags&logErrors != 0 {
				s.c.sess.log.Println("got error:", token)
			}
			if s.c.sess.tranid != 0 {
				return nil, token
			}
			return nil, CheckBadConn(token)
		}
	}
	return &MssqlResult{s.c, rowCount}, nil
}

type MssqlRows struct {
	sess    *tdsSession
	cols    []string
	tokchan chan tokenStruct

	nextCols []string
}

func (rc *MssqlRows) Close() error {
	for _ = range rc.tokchan {
	}
	rc.tokchan = nil
	return nil
}

func (rc *MssqlRows) Columns() (res []string) {
	return rc.cols
}

func (rc *MssqlRows) Next(dest []driver.Value) (err error) {
	if rc.nextCols != nil {
		return io.EOF
	}
	for tok := range rc.tokchan {
		switch tokdata := tok.(type) {
		case []columnStruct:
			cols := make([]string, len(tokdata))
			for i, col := range tokdata {
				cols[i] = col.ColName
			}
			rc.nextCols = cols
			return io.EOF
		case []interface{}:
			for i := range dest {
				dest[i] = tokdata[i]
			}
			return nil
		case error:
			return tokdata
		}
	}
	return io.EOF
}

func (rc *MssqlRows) HasNextResultSet() bool {
	return rc.nextCols != nil
}

func (rc *MssqlRows) NextResultSet() error {
	rc.cols = rc.nextCols
	rc.nextCols = nil
	if rc.cols == nil {
		return io.EOF
	}
	return nil
}

func (s *MssqlStmt) makeParam(val driver.Value) (res Param, err error) {
	if val == nil {
		res.ti.TypeId = typeNVarChar
		res.buffer = nil
		res.ti.Size = 2
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
		res.ti.TypeId = typeNVarChar
		res.buffer = str2ucs2(val)
		res.ti.Size = len(res.buffer)
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
		err = fmt.Errorf("mssql: unknown type for %T", val)
		return
	}
	return
}

type MssqlResult struct {
	c            *MssqlConn
	rowsAffected int64
}

func (r *MssqlResult) RowsAffected() (int64, error) {
	return r.rowsAffected, nil
}

func (r *MssqlResult) LastInsertId() (int64, error) {
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
