package hbase

import (
	"bytes"
	"fmt"
	"io"

	"github.com/juju/errors"
	"github.com/pingcap/go-hbase/iohelper"
)

type Column struct {
	Family []byte
	Qual   []byte
}

func NewColumn(family, qual []byte) *Column {
	return &Column{
		Family: family,
		Qual:   qual,
	}
}

func encode(parts ...[]byte) ([]byte, error) {
	buf := &bytes.Buffer{}
	for _, p := range parts {
		err := iohelper.WriteVarBytes(buf, p)
		if err != nil {
			return nil, errors.Trace(err)
		}
	}
	return buf.Bytes(), nil
}

func decode(encoded []byte) ([][]byte, error) {
	var ret [][]byte
	buf := bytes.NewBuffer(encoded)
	for {
		b, err := iohelper.ReadVarBytes(buf)
		if len(b) == 0 || (err != nil && ErrorEqual(err, io.EOF)) {
			break
		}
		ret = append(ret, b)
	}
	return ret, nil
}

func (c *Column) Write(w io.Writer) error {
	err := iohelper.WriteVarBytes(w, c.Family)
	if err != nil {
		return errors.Trace(err)
	}

	err = iohelper.WriteVarBytes(w, c.Qual)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (c *Column) String() string {
	b, err := encode(c.Family, c.Qual)
	if err != nil {
		return fmt.Sprintf("invalid column - %v", err)
	}
	return string(b)
}

func (c *Column) ParseFromString(s string) error {
	pairs, err := decode([]byte(s))
	if err != nil {
		return errors.Trace(err)
	}

	c.Family = pairs[0]
	c.Qual = pairs[1]
	return nil
}

type ColumnCoordinate struct {
	Table []byte
	Row   []byte
	Column
}

func NewColumnCoordinate(table, row, family, qual []byte) *ColumnCoordinate {
	return &ColumnCoordinate{
		Table: table,
		Row:   row,
		Column: Column{
			Family: family,
			Qual:   qual,
		},
	}
}

func (c *ColumnCoordinate) Write(w io.Writer) error {
	err := iohelper.WriteVarBytes(w, c.Table)
	if err != nil {
		return errors.Trace(err)
	}

	err = iohelper.WriteVarBytes(w, c.Row)
	if err != nil {
		return errors.Trace(err)
	}

	err = c.Column.Write(w)
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (c *ColumnCoordinate) Equal(a *ColumnCoordinate) bool {
	return bytes.Compare(c.Table, a.Table) == 0 &&
		bytes.Compare(c.Row, a.Row) == 0 &&
		bytes.Compare(c.Family, a.Family) == 0 &&
		bytes.Compare(c.Qual, a.Qual) == 0
}

func (c *ColumnCoordinate) String() string {
	b, err := encode(c.Table, c.Row, c.Family, c.Qual)
	if err != nil {
		return fmt.Sprintf("invalid column coordinate - %v", err)
	}
	return string(b)
}

func (c *ColumnCoordinate) ParseFromString(s string) error {
	pairs, err := decode([]byte(s))
	if err != nil {
		return errors.Trace(err)
	}

	c.Table = pairs[0]
	c.Row = pairs[1]
	c.Family = pairs[2]
	c.Qual = pairs[3]
	return nil
}

func (c *ColumnCoordinate) ParseField(b iohelper.ByteMultiReader) error {
	table, err := iohelper.ReadVarBytes(b)
	if err != nil {
		return errors.Trace(err)
	}
	c.Table = table

	row, err := iohelper.ReadVarBytes(b)
	if err != nil {
		return errors.Trace(err)
	}
	c.Row = row

	family, err := iohelper.ReadVarBytes(b)
	if err != nil {
		return errors.Trace(err)
	}
	c.Family = family

	qual, err := iohelper.ReadVarBytes(b)
	if err != nil {
		return errors.Trace(err)
	}
	c.Qual = qual
	return nil
}

func (c *ColumnCoordinate) GetColumn() *Column {
	return &Column{
		Family: c.Family,
		Qual:   c.Qual,
	}
}
