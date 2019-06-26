// +build go1.9

package mssql

import (
	"database/sql"
	"database/sql/driver"
	"errors"
	"fmt"
	"reflect"
	"time"

	// "github.com/cockroachdb/apd"
	"cloud.google.com/go/civil"
)

// Type alias provided for compatibility.

type MssqlDriver = Driver           // Deprecated: users should transition to the new name when possible.
type MssqlBulk = Bulk               // Deprecated: users should transition to the new name when possible.
type MssqlBulkOptions = BulkOptions // Deprecated: users should transition to the new name when possible.
type MssqlConn = Conn               // Deprecated: users should transition to the new name when possible.
type MssqlResult = Result           // Deprecated: users should transition to the new name when possible.
type MssqlRows = Rows               // Deprecated: users should transition to the new name when possible.
type MssqlStmt = Stmt               // Deprecated: users should transition to the new name when possible.

var _ driver.NamedValueChecker = &Conn{}

// VarChar parameter types.
type VarChar string

// DateTime1 encodes parameters to original DateTime SQL types.
type DateTime1 time.Time

// DateTimeOffset encodes parameters to DateTimeOffset, preserving the UTC offset.
type DateTimeOffset time.Time

func (c *Conn) CheckNamedValue(nv *driver.NamedValue) error {
	switch v := nv.Value.(type) {
	case sql.Out:
		if c.outs == nil {
			c.outs = make(map[string]interface{})
		}
		c.outs[nv.Name] = v.Dest

		if v.Dest == nil {
			return errors.New("destination is a nil pointer")
		}

		dest_info := reflect.ValueOf(v.Dest)
		if dest_info.Kind() != reflect.Ptr {
			return errors.New("destination not a pointer")
		}

		pointed_value := reflect.Indirect(dest_info)

		// Unwrap the Out value and check the inner value.
		lnv := *nv
		lnv.Value = pointed_value.Interface()
		err := c.CheckNamedValue(&lnv)
		if err != nil {
			return err
		}
		nv.Value = sql.Out{Dest: lnv.Value}
		return nil
	case VarChar:
		return nil
	case DateTime1:
		return nil
	case DateTimeOffset:
		return nil
	case civil.Date:
		return nil
	case civil.DateTime:
		return nil
	case civil.Time:
		return nil
	// case *apd.Decimal:
	// 	return nil
	default:
		conv_val, err := driver.DefaultParameterConverter.ConvertValue(v)
		nv.Value = conv_val
		return err
	}
}

func (s *Stmt) makeParamExtra(val driver.Value) (res param, err error) {
	switch val := val.(type) {
	case VarChar:
		res.ti.TypeId = typeBigVarChar
		res.buffer = []byte(val)
		res.ti.Size = len(res.buffer)
	case DateTime1:
		t := time.Time(val)
		res.ti.TypeId = typeDateTimeN
		res.buffer = encodeDateTime(t)
		res.ti.Size = len(res.buffer)
	case DateTimeOffset:
		res.ti.TypeId = typeDateTimeOffsetN
		res.ti.Scale = 7
		res.buffer = encodeDateTimeOffset(time.Time(val), int(res.ti.Scale))
		res.ti.Size = len(res.buffer)
	case civil.Date:
		res.ti.TypeId = typeDateN
		res.buffer = encodeDate(val.In(time.UTC))
		res.ti.Size = len(res.buffer)
	case civil.DateTime:
		res.ti.TypeId = typeDateTime2N
		res.ti.Scale = 7
		res.buffer = encodeDateTime2(val.In(time.UTC), int(res.ti.Scale))
		res.ti.Size = len(res.buffer)
	case civil.Time:
		res.ti.TypeId = typeTimeN
		res.ti.Scale = 7
		res.buffer = encodeTime(val.Hour, val.Minute, val.Second, val.Nanosecond, int(res.ti.Scale))
		res.ti.Size = len(res.buffer)
	case sql.Out:
		res, err = s.makeParam(val.Dest)
		res.Flags = fByRevValue
	default:
		err = fmt.Errorf("mssql: unknown type for %T", val)
	}
	return
}

func scanIntoOut(name string, fromServer, scanInto interface{}) error {
	switch fs := fromServer.(type) {
	case int64:
		switch si := scanInto.(type) {
		case *int64:
			*si = fs
			return nil
		}
	case string:
		switch si := scanInto.(type) {
		case *string:
			*si = fs
			return nil
		case *VarChar:
			*si = VarChar(fs)
			return nil
		}
	}

	dpv := reflect.ValueOf(scanInto)
	if dpv.Kind() != reflect.Ptr {
		return errors.New("destination not a pointer")
	}
	if dpv.IsNil() {
		return errors.New("destination is a nil pointer")
	}
	sv := reflect.ValueOf(fromServer)

	dv := reflect.Indirect(dpv)
	if sv.IsValid() && sv.Type().AssignableTo(dv.Type()) {
		dv.Set(sv)
		return nil
	}

	if sv.Type().ConvertibleTo(dv.Type()) {
		dv.Set(sv.Convert(dv.Type()))
		return nil
	}
	return fmt.Errorf("unsupported type for parameter %[3]q from server %[1]T=%[1]v into %[2]T=%[2]v ", fromServer, scanInto, name)
}
