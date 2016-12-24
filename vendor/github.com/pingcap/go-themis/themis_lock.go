package themis

import (
	"bytes"
	"encoding/binary"
	"io"

	"github.com/juju/errors"
	"github.com/pingcap/go-hbase"
	"github.com/pingcap/go-hbase/iohelper"
)

var (
	_ Lock = (*themisPrimaryLock)(nil)
	_ Lock = (*themisSecondaryLock)(nil)
)

type themisLock struct {
	// lock coordinate, table, row, cf, q
	coordinate *hbase.ColumnCoordinate
	// lock type: put/delete/minimal(lock only)
	typ hbase.Type
	// prewrite ts
	ts uint64
	// not used, for alignment
	wallTs uint64
	// not used, for alignment
	clientAddr string
	expired    bool
}

func (l *themisLock) Timestamp() uint64 {
	return l.ts
}

func (l *themisLock) IsExpired() bool {
	return l.expired
}

func (l *themisLock) SetExpired(b bool) {
	l.expired = b
}

func (l *themisLock) SetCoordinate(c *hbase.ColumnCoordinate) {
	l.coordinate = c
}

func (l *themisLock) Coordinate() *hbase.ColumnCoordinate {
	return l.coordinate
}

func (l *themisLock) Context() interface{} {
	return nil
}

func (l *themisLock) Type() hbase.Type {
	return l.typ
}

func (l *themisLock) write(w io.Writer) {
	binary.Write(w, binary.BigEndian, byte(l.typ))
	binary.Write(w, binary.BigEndian, int64(l.ts))
	// write client addr
	iohelper.WriteVarBytes(w, []byte(l.clientAddr))
	binary.Write(w, binary.BigEndian, int64(l.wallTs))
}

func (l *themisLock) parse(r iohelper.ByteMultiReader) error {
	// read type
	var typ uint8
	err := binary.Read(r, binary.BigEndian, &typ)
	if err != nil {
		return errors.Trace(err)
	}
	l.typ = hbase.Type(typ)

	// read ts
	var ts int64
	err = binary.Read(r, binary.BigEndian, &ts)
	if err != nil {
		return errors.Trace(err)
	}
	l.ts = uint64(ts)

	// read client addr
	sz, err := binary.ReadUvarint(r)
	if err != nil {
		return errors.Trace(err)
	}
	addr := make([]byte, sz)
	r.Read(addr)
	l.clientAddr = string(addr)

	// read wall time
	var wallTs int64
	err = binary.Read(r, binary.BigEndian, &wallTs)
	if err != nil {
		return errors.Trace(err)
	}
	l.wallTs = uint64(wallTs)
	return nil
}

func parseLockFromBytes(b []byte) (Lock, error) {
	buf := bytes.NewBuffer(b)
	var isPrimary uint8
	err := binary.Read(buf, binary.BigEndian, &isPrimary)
	if err != nil {
		return nil, errors.Trace(err)
	}
	var ret Lock
	if isPrimary == 1 {
		l := newThemisPrimaryLock()
		err = l.parse(buf)
		ret = l
	} else {
		l := newThemisSecondaryLock()
		err = l.parse(buf)
		ret = l
	}
	if err != nil {
		return nil, errors.Trace(err)
	}
	return ret, nil
}

func isLockResult(r *hbase.ResultRow) bool {
	return len(r.SortedColumns) > 0 && isLockColumn(r.SortedColumns[0].Column)
}

func isLockColumn(c hbase.Column) bool {
	return bytes.Compare(c.Family, LockFamilyName) == 0
}
