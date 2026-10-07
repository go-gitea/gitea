// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package db

import (
	"database/sql"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strings"

	"rdb.red-soft.ru/fbx/stdlib"
	"xorm.io/xorm/convert"
	"xorm.io/xorm/core"
	"xorm.io/xorm/dialects"
	"xorm.io/xorm/schemas"
)

type firebirdDriver struct{}

// Parse accepts a full "firebird://" DSN as well as a bare database name or path.
func (d *firebirdDriver) Parse(_, dsn string) (*dialects.URI, error) {
	uri := &dialects.URI{DBType: schemas.FIREBIRD}

	if strings.Contains(dsn, "://") {
		u, err := url.Parse(dsn)
		if err != nil {
			return nil, err
		}
		if u.User != nil {
			uri.User = u.User.Username()
			uri.Passwd, _ = u.User.Password()
		}
		uri.Host, uri.Port, err = net.SplitHostPort(u.Host)
		if err != nil {
			uri.Host = u.Host
		}
		// fbx strips exactly one leading slash from the URL path, so an absolute
		// database path must be written with a doubled slash
		uri.DBName = strings.TrimPrefix(u.Path, "/")
	} else {
		uri.DBName = dsn
	}
	return uri, nil
}

func (d *firebirdDriver) Features() *dialects.DriverFeatures {
	// Firebird has no LastInsertId. xorm calls it and silently skips assigning the
	// generated id when it fails, so this must stay false and the id is fetched
	// through INSERT ... RETURNING instead.
	return &dialects.DriverFeatures{SupportReturnInsertedID: false}
}

func (d *firebirdDriver) GenScanResult(columnType string) (any, error) {
	switch strings.ToUpper(columnType) {
	case "BOOLEAN":
		return new(sql.NullBool), nil
	case "SMALLINT":
		return new(sql.NullInt16), nil
	case "INTEGER":
		return new(sql.NullInt32), nil
	case "BIGINT":
		return new(sql.NullInt64), nil
	case "DATE", "TIME", "TIMESTAMP":
		return new(sql.NullTime), nil
	case "REAL", "FLOAT", "DOUBLE PRECISION", "NUMERIC", "DECIMAL":
		return new(sql.NullFloat64), nil
	}
	return new(sql.NullString), nil
}

// firebirdSubTypeText is the Firebird BLOB SUB_TYPE 1 (text) discriminator. fbx
// only exposes it as stdlib.Blob.SubType, reachable solely from this struct.
const firebirdSubTypeText = 1

// blobScanner decodes a Firebird BLOB, which database/sql hands over as
// stdlib.Blob rather than as bytes or string. Text blobs become strings so that
// xorm "Text" columns round-trip, while binary blobs stay []byte.
type blobScanner struct {
	isText bool
	valid  bool
	text   string
	data   []byte
}

func (s *blobScanner) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		s.valid = false
		return nil
	case stdlib.Blob:
		s.isText = v.SubType == firebirdSubTypeText
		if s.isText {
			// Text() decodes using the column charset and closes the reader
			t, err := v.Text()
			if err != nil {
				return err
			}
			s.text, s.valid = t, true
			return nil
		}
		b, err := v.Bytes()
		if err != nil {
			return err
		}
		s.data, s.valid = b, true
		return nil
	case []byte:
		s.data, s.valid = v, true
		return nil
	case string:
		s.isText, s.text, s.valid = true, v, true
		return nil
	}
	return fmt.Errorf("firebird: cannot scan %T into a BLOB column", src)
}

func (d *firebirdDriver) Scan(ctx *dialects.ScanContext, rows *core.Rows, columns []*sql.ColumnType, values ...any) error {
	scanResults := make([]any, 0, len(columns))
	blobs := make([]*blobScanner, len(columns))
	for i := range values {
		// DatabaseTypeName reports the numeric blob subtype ("520" for text), so
		// the scan type is the reliable discriminator
		if columns[i].ScanType() == reflect.TypeFor[stdlib.Blob]() {
			blobs[i] = &blobScanner{}
			scanResults = append(scanResults, blobs[i])
			continue
		}
		scanResults = append(scanResults, values[i])
	}

	if err := rows.Scan(scanResults...); err != nil {
		return err
	}

	for i, s := range blobs {
		if s == nil {
			continue
		}
		var out any
		if s.valid {
			if s.isText {
				out = s.text
			} else {
				out = s.data
			}
		}
		if err := convert.Assign(values[i], out, ctx.DBLocation, ctx.UserLocation); err != nil {
			return err
		}
	}
	return rows.Err()
}

func init() {
	// registered eagerly: "gitea dump" looks the dialect up by name to convert a dump
	// to another SQL dialect, without connecting to Firebird at all
	sql.Register(sqlDriverFirebird, stdlib.GetDefaultDriver())
	dialects.RegisterDriver(sqlDriverFirebird, &firebirdDriver{})
	dialects.RegisterDialect(schemas.FIREBIRD, func() dialects.Dialect { return &firebird{} })
}
