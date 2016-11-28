package themis

import (
	"bytes"
	"encoding/binary"

	"github.com/ngaut/log"
	"github.com/pingcap/go-hbase"
)

type ThemisScanner struct {
	scan *hbase.Scan
	txn  *themisTxn
	tbl  []byte
}

func newThemisScanner(tbl []byte, txn *themisTxn, batchSize int, c hbase.HBaseClient) *ThemisScanner {
	s := hbase.NewScan(tbl, batchSize, c)
	// add start ts
	b := bytes.NewBuffer(nil)
	binary.Write(b, binary.BigEndian, txn.startTs)
	s.AddAttr("_themisTransationStartTs_", b.Bytes())
	return &ThemisScanner{
		scan: s,
		txn:  txn,
		tbl:  tbl,
	}
}

func (s *ThemisScanner) setStartRow(start []byte) {
	s.scan.StartRow = start
}

func (s *ThemisScanner) setStopRow(stop []byte) {
	s.scan.StopRow = stop
}

func (s *ThemisScanner) SetTimeRange(tsRangeFrom uint64, tsRangeTo uint64) {
	s.scan.TsRangeFrom = tsRangeFrom
	s.scan.TsRangeTo = tsRangeTo
}

func (s *ThemisScanner) SetMaxVersions(maxVersions uint32) {
	s.scan.MaxVersions = maxVersions
}

func (s *ThemisScanner) createGetFromScan(row []byte) *hbase.Get {
	return s.scan.CreateGetFromScan(row)
}

func (s *ThemisScanner) Next() *hbase.ResultRow {
	r := s.scan.Next()
	if r == nil {
		return nil
	}
	// if we encounter conflict locks, we need to clean lock for this row and read again
	if isLockResult(r) {
		g := s.createGetFromScan(r.Row)
		r, err := s.txn.tryToCleanLockAndGetAgain(s.tbl, g, r.SortedColumns)
		if err != nil {
			log.Error(err)
			return nil
		}
		// empty result indicates the current row has been erased, we should get next row
		if r == nil {
			return s.Next()
		} else {
			return r
		}
	}
	return r
}

func (s *ThemisScanner) Closed() bool {
	return s.scan.Closed()
}

func (s *ThemisScanner) Close() {
	if !s.scan.Closed() {
		// TODO: handle error, now just log
		if err := s.scan.Close(); err != nil {
			log.Warnf("scanner close error, scan: %s, error: %v", s.scan, err)
		}
	}
}
