package themis

import (
	"bytes"
	"encoding/binary"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/go-hbase"
	"github.com/pingcap/go-hbase/iohelper"
)

type themisPrimaryLock struct {
	*themisLock
	// {coordinate => type}
	secondaries map[string]hbase.Type
}

func newThemisPrimaryLock() *themisPrimaryLock {
	return &themisPrimaryLock{
		themisLock: &themisLock{
			clientAddr: "null",
		},
		secondaries: map[string]hbase.Type{},
	}
}

func (l *themisPrimaryLock) Primary() Lock {
	return l
}

func (l *themisPrimaryLock) Secondaries() []Lock {
	var slocks []Lock
	for k, v := range l.secondaries {
		c := &hbase.ColumnCoordinate{}
		// TODO: handle error, now just ignore
		if err := c.ParseFromString(k); err != nil {
			log.Warnf("parse from string error, column coordinate: %s, secondary: %s, error: %v", c, k, err)
			continue
		}
		slock := newThemisSecondaryLock()
		slock.primaryCoordinate = l.coordinate
		slock.coordinate = c
		slock.ts = l.ts
		slock.typ = v
		slocks = append(slocks, slock)
	}
	return slocks
}

func (l *themisPrimaryLock) Encode() []byte {
	buf := bytes.NewBuffer(nil)
	// set is primary
	binary.Write(buf, binary.BigEndian, uint8(1))
	l.themisLock.write(buf)

	// write secondaries
	binary.Write(buf, binary.BigEndian, int32(len(l.secondaries)))
	for k, v := range l.secondaries {
		c := &hbase.ColumnCoordinate{}
		// TODO: handle error, now just log
		if err := c.ParseFromString(k); err != nil {
			log.Warnf("parse from string error, column coordinate: %s, secondary: %s, error: %v", c, k, err)
		}
		// TODO: handle error, now just log
		if err := c.Write(buf); err != nil {
			log.Warnf("write error, column coordinate: %s, buf: %s, error: %v", c, buf, err)
		}
		buf.WriteByte(uint8(v))
	}
	return buf.Bytes()
}

func (l *themisPrimaryLock) IsExpired() bool {
	return l.themisLock.expired
}

func (l *themisPrimaryLock) getSecondaryColumnType(c *hbase.ColumnCoordinate) hbase.Type {
	v, ok := l.secondaries[c.String()]
	if !ok {
		return hbase.TypeMinimum
	}
	return v
}

func (l *themisPrimaryLock) Role() LockRole {
	return RolePrimary
}

func (l *themisPrimaryLock) addSecondary(col *hbase.ColumnCoordinate, t hbase.Type) {
	l.secondaries[col.String()] = t
}

func (l *themisPrimaryLock) parse(buf iohelper.ByteMultiReader) error {
	l.themisLock.parse(buf)
	var sz int32
	err := binary.Read(buf, binary.BigEndian, &sz)
	if err != nil {
		return errors.Trace(err)
	}
	for i := 0; i < int(sz); i++ {
		c := &hbase.ColumnCoordinate{}
		c.ParseField(buf)
		b, err := buf.ReadByte()
		if err != nil {
			return errors.Trace(err)
		}
		t := hbase.Type(b)
		l.addSecondary(c, t)
	}
	return nil
}
