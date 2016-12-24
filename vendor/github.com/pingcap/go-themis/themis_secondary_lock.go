package themis

import (
	"bytes"
	"encoding/binary"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/go-hbase"
	"github.com/pingcap/go-hbase/iohelper"
)

type themisSecondaryLock struct {
	*themisLock
	primaryCoordinate *hbase.ColumnCoordinate
}

func newThemisSecondaryLock() *themisSecondaryLock {
	return &themisSecondaryLock{
		themisLock: &themisLock{
			clientAddr: "null",
		},
		primaryCoordinate: &hbase.ColumnCoordinate{},
	}
}

func (l *themisSecondaryLock) Primary() Lock {
	pl := newThemisPrimaryLock()
	pl.coordinate = l.primaryCoordinate
	pl.ts = l.ts
	pl.clientAddr = l.clientAddr
	pl.addSecondary(l.coordinate, l.typ)
	return pl
}

func (l *themisSecondaryLock) Secondaries() []Lock {
	return nil
}

func (l *themisSecondaryLock) Role() LockRole {
	return RoleSecondary
}

func (l *themisSecondaryLock) Encode() []byte {
	buf := bytes.NewBuffer(nil)
	binary.Write(buf, binary.BigEndian, uint8(0))
	l.themisLock.write(buf)
	// TODO: handle error, now just log
	if err := l.primaryCoordinate.Write(buf); err != nil {
		log.Warnf("write error, primary coordinate: %s, buf: %s, err: %v", l, buf, err)
	}
	return buf.Bytes()
}

func (l *themisSecondaryLock) parse(r iohelper.ByteMultiReader) error {
	l.themisLock.parse(r)
	primary := &hbase.ColumnCoordinate{}
	err := primary.ParseField(r)
	if err != nil {
		return errors.Trace(err)
	}
	l.primaryCoordinate = primary
	return nil
}
