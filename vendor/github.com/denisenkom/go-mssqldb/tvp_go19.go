// +build go1.9

package mssql

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
)

const (
	jsonTag      = "json"
	tvpTag       = "tvp"
	skipTagValue = "-"
	sqlSeparator = "."
)

var (
	ErrorEmptyTVPTypeName = errors.New("TVPTypeName must not be empty")
	ErrorTypeSlice        = errors.New("TVPType must be slice type")
	ErrorTypeSliceIsEmpty = errors.New("TVPType mustn't be null value")
	ErrorSkip             = errors.New("all fields mustn't skip")
	ErrorObjectName       = errors.New("wrong tvp name")
)

//TVPType is driver type, which allows supporting Table Valued Parameters (TVP) in SQL Server
type TVPType struct {
	//TVP param name, mustn't be default value
	TVPTypeName string
	//TVP Value Param must be the slice, mustn't be nil
	TVPValue interface{}
}

func (tvp TVPType) check() error {
	if len(tvp.TVPTypeName) == 0 {
		return ErrorEmptyTVPTypeName
	}
	if !isProc(tvp.TVPTypeName) {
		return ErrorEmptyTVPTypeName
	}
	if sepCount := getCountSQLSeparators(tvp.TVPTypeName); sepCount > 1 {
		return ErrorObjectName
	}
	valueOf := reflect.ValueOf(tvp.TVPValue)
	if valueOf.Kind() != reflect.Slice {
		return ErrorTypeSlice
	}
	if valueOf.IsNil() {
		return ErrorTypeSliceIsEmpty
	}
	if reflect.TypeOf(tvp.TVPValue).Elem().Kind() != reflect.Struct {
		return ErrorTypeSlice
	}
	return nil
}

func (tvp TVPType) encode(schema, name string) ([]byte, error) {
	columnStr, tvpFieldIndexes, err := tvp.columnTypes()
	if err != nil {
		return nil, err
	}
	preparedBuffer := make([]byte, 0, 20+(10*len(columnStr)))
	buf := bytes.NewBuffer(preparedBuffer)
	err = writeBVarChar(buf, "")
	if err != nil {
		return nil, err
	}

	writeBVarChar(buf, schema)
	writeBVarChar(buf, name)
	binary.Write(buf, binary.LittleEndian, uint16(len(columnStr)))

	for i, column := range columnStr {
		binary.Write(buf, binary.LittleEndian, uint32(column.UserType))
		binary.Write(buf, binary.LittleEndian, uint16(column.Flags))
		writeTypeInfo(buf, &columnStr[i].ti)
		writeBVarChar(buf, "")
	}
	err = buf.WriteByte(_TVP_END_TOKEN)
	if err != nil {
		return nil, err
	}
	conn := new(Conn)
	conn.sess = new(tdsSession)
	conn.sess.loginAck = loginAckStruct{TDSVersion: verTDS73}
	stmt := &Stmt{
		c: conn,
	}

	val := reflect.ValueOf(tvp.TVPValue)
	for i := 0; i < val.Len(); i++ {
		refStr := reflect.ValueOf(val.Index(i).Interface())
		buf.WriteByte(_TVP_ROW_TOKEN)
		for _, j := range tvpFieldIndexes {
			field := refStr.Field(j)
			tvpVal := field.Interface()
			valOf := reflect.ValueOf(tvpVal)
			elemKind := field.Kind()
			if elemKind == reflect.Ptr && valOf.IsNil() {
				switch tvpVal.(type) {
				case *bool, *time.Time, *int8, *int16, *int32, *int64, *float32, *float64:
					binary.Write(buf, binary.LittleEndian, uint8(0))
					continue
				default:
					binary.Write(buf, binary.LittleEndian, uint64(_PLP_NULL))
					continue
				}
			}
			if elemKind == reflect.Slice && valOf.IsNil() {
				binary.Write(buf, binary.LittleEndian, uint64(_PLP_NULL))
				continue
			}

			cval, err := convertInputParameter(tvpVal)
			if err != nil {
				return nil, fmt.Errorf("failed to convert tvp parameter row col: %s", err)
			}
			param, err := stmt.makeParam(cval)
			if err != nil {
				return nil, fmt.Errorf("failed to make tvp parameter row col: %s", err)
			}
			columnStr[j].ti.Writer(buf, param.ti, param.buffer)
		}
	}
	buf.WriteByte(_TVP_END_TOKEN)
	return buf.Bytes(), nil
}

func (tvp TVPType) columnTypes() ([]columnStruct, []int, error) {
	val := reflect.ValueOf(tvp.TVPValue)
	var firstRow interface{}
	if val.Len() != 0 {
		firstRow = val.Index(0).Interface()
	} else {
		firstRow = reflect.New(reflect.TypeOf(tvp.TVPValue).Elem()).Elem().Interface()
	}

	tvpRow := reflect.TypeOf(firstRow)
	columnCount := tvpRow.NumField()
	defaultValues := make([]interface{}, 0, columnCount)
	tvpFieldIndexes := make([]int, 0, columnCount)
	for i := 0; i < columnCount; i++ {
		field := tvpRow.Field(i)
		tvpTagValue, isTvpTag := field.Tag.Lookup(tvpTag)
		jsonTagValue, isJsonTag := field.Tag.Lookup(jsonTag)
		if IsSkipField(tvpTagValue, isTvpTag, jsonTagValue, isJsonTag) {
			continue
		}
		tvpFieldIndexes = append(tvpFieldIndexes, i)
		if field.Type.Kind() == reflect.Ptr {
			v := reflect.New(field.Type.Elem())
			defaultValues = append(defaultValues, v.Interface())
			continue
		}
		defaultValues = append(defaultValues, reflect.Zero(field.Type).Interface())
	}

	if columnCount-len(tvpFieldIndexes) == columnCount {
		return nil, nil, ErrorSkip
	}

	conn := new(Conn)
	conn.sess = new(tdsSession)
	conn.sess.loginAck = loginAckStruct{TDSVersion: verTDS73}
	stmt := &Stmt{
		c: conn,
	}

	columnConfiguration := make([]columnStruct, 0, columnCount)
	for index, val := range defaultValues {
		cval, err := convertInputParameter(val)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to convert tvp parameter row %d col %d: %s", index, val, err)
		}
		param, err := stmt.makeParam(cval)
		if err != nil {
			return nil, nil, err
		}
		column := columnStruct{
			ti: param.ti,
		}
		switch param.ti.TypeId {
		case typeNVarChar, typeBigVarBin:
			column.ti.Size = 0
		}
		columnConfiguration = append(columnConfiguration, column)
	}

	return columnConfiguration, tvpFieldIndexes, nil
}

func IsSkipField(tvpTagValue string, isTvpValue bool, jsonTagValue string, isJsonTagValue bool) bool {
	if !isTvpValue && !isJsonTagValue {
		return false
	} else if isTvpValue && tvpTagValue != skipTagValue {
		return false
	} else if !isTvpValue && isJsonTagValue && jsonTagValue != skipTagValue {
		return false
	}
	return true
}

func getSchemeAndName(tvpName string) (string, string, error) {
	if len(tvpName) == 0 {
		return "", "", ErrorEmptyTVPTypeName
	}
	splitVal := strings.Split(tvpName, ".")
	if len(splitVal) > 2 {
		return "", "", errors.New("wrong tvp name")
	}
	if len(splitVal) == 2 {
		res := make([]string, 2)
		for key, value := range splitVal {
			tmp := strings.Replace(value, "[", "", -1)
			tmp = strings.Replace(tmp, "]", "", -1)
			res[key] = tmp
		}
		return res[0], res[1], nil
	}
	tmp := strings.Replace(splitVal[0], "[", "", -1)
	tmp = strings.Replace(tmp, "]", "", -1)

	return "", tmp, nil
}

func getCountSQLSeparators(str string) int {
	return strings.Count(str, sqlSeparator)
}
