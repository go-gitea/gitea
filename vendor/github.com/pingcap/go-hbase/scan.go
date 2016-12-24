package hbase

import (
	"bytes"

	pb "github.com/golang/protobuf/proto"
	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/go-hbase/proto"
)

// nextKey returns the next key in byte-order.
// for example:
// nil -> [0]
// [] -> [0]
// [0] -> [1]
// [1, 2, 3] -> [1, 2, 4]
// [1, 255] -> [2, 0]
// [255] -> [0, 0]
func nextKey(data []byte) []byte {
	// nil or []byte{}
	dataLen := len(data)
	if dataLen == 0 {
		return []byte{0}
	}

	// Check and process carry bit.
	i := dataLen - 1
	data[i]++
	for i > 0 {
		if data[i] == 0 {
			i--
			data[i]++
		} else {
			break
		}
	}

	// Check whether need to add another byte for carry bit,
	// like [255] -> [0, 0]
	if data[i] == 0 {
		data = append([]byte{0}, data...)
	}

	return data
}

const (
	defaultScanMaxRetries = 3
)

type Scan struct {
	client *client
	id     uint64
	table  []byte
	// row key
	StartRow     []byte
	StopRow      []byte
	families     [][]byte
	qualifiers   [][][]byte
	nextStartKey []byte
	numCached    int
	closed       bool
	location     *RegionInfo
	server       *connection
	cache        []*ResultRow
	attrs        map[string][]byte
	MaxVersions  uint32
	TsRangeFrom  uint64
	TsRangeTo    uint64
	lastResult   *ResultRow
	// if region split, set startKey = lastResult.Row, but must skip the first
	skipFirst  bool
	maxRetries int
}

func NewScan(table []byte, batchSize int, c HBaseClient) *Scan {
	if batchSize <= 0 {
		batchSize = 100
	}
	return &Scan{
		client:       c.(*client),
		table:        table,
		nextStartKey: nil,
		families:     make([][]byte, 0),
		qualifiers:   make([][][]byte, 0),
		numCached:    batchSize,
		closed:       false,
		attrs:        make(map[string][]byte),
		maxRetries:   defaultScanMaxRetries,
	}
}

func (s *Scan) Close() error {
	if s.closed {
		return nil
	}

	err := s.closeScan(s.server, s.location, s.id)
	if err != nil {
		return errors.Trace(err)
	}

	s.closed = true
	return nil
}

func (s *Scan) AddColumn(family, qual []byte) {
	s.AddFamily(family)
	pos := s.posOfFamily(family)
	s.qualifiers[pos] = append(s.qualifiers[pos], qual)
}

func (s *Scan) AddStringColumn(family, qual string) {
	s.AddColumn([]byte(family), []byte(qual))
}

func (s *Scan) AddFamily(family []byte) {
	pos := s.posOfFamily(family)
	if pos == -1 {
		s.families = append(s.families, family)
		s.qualifiers = append(s.qualifiers, make([][]byte, 0))
	}
}

func (s *Scan) AddStringFamily(family string) {
	s.AddFamily([]byte(family))
}

func (s *Scan) posOfFamily(family []byte) int {
	for p, v := range s.families {
		if bytes.Equal(family, v) {
			return p
		}
	}
	return -1
}

func (s *Scan) AddAttr(name string, val []byte) {
	s.attrs[name] = val
}

func (s *Scan) AddTimeRange(from uint64, to uint64) {
	s.TsRangeFrom = from
	s.TsRangeTo = to
}

func (s *Scan) Closed() bool {
	return s.closed
}

func (s *Scan) CreateGetFromScan(row []byte) *Get {
	g := NewGet(row)
	for i, family := range s.families {
		if len(s.qualifiers[i]) > 0 {
			for _, qual := range s.qualifiers[i] {
				g.AddColumn(family, qual)
			}
		} else {
			g.AddFamily(family)
		}
	}
	return g
}

func (s *Scan) getData(startKey []byte, retries int) ([]*ResultRow, error) {
	server, location, err := s.getServerAndLocation(s.table, startKey)
	if err != nil {
		return nil, errors.Trace(err)
	}

	req := &proto.ScanRequest{
		Region: &proto.RegionSpecifier{
			Type:  proto.RegionSpecifier_REGION_NAME.Enum(),
			Value: []byte(location.Name),
		},
		NumberOfRows: pb.Uint32(uint32(s.numCached)),
		Scan:         &proto.Scan{},
	}

	// set attributes
	var attrs []*proto.NameBytesPair
	for k, v := range s.attrs {
		p := &proto.NameBytesPair{
			Name:  pb.String(k),
			Value: v,
		}
		attrs = append(attrs, p)
	}
	if len(attrs) > 0 {
		req.Scan.Attribute = attrs
	}

	if s.id > 0 {
		req.ScannerId = pb.Uint64(s.id)
	}
	req.Scan.StartRow = startKey
	if s.StopRow != nil {
		req.Scan.StopRow = s.StopRow
	}
	if s.MaxVersions > 0 {
		req.Scan.MaxVersions = &s.MaxVersions
	}
	if s.TsRangeTo > s.TsRangeFrom {
		req.Scan.TimeRange = &proto.TimeRange{
			From: pb.Uint64(s.TsRangeFrom),
			To:   pb.Uint64(s.TsRangeTo),
		}
	}

	for i, v := range s.families {
		req.Scan.Column = append(req.Scan.Column, &proto.Column{
			Family:    v,
			Qualifier: s.qualifiers[i],
		})
	}

	cl := newCall(req)
	err = server.call(cl)
	if err != nil {
		return nil, errors.Trace(err)
	}

	msg := <-cl.responseCh
	rs, err := s.processResponse(msg)
	if err != nil && (isNotInRegionError(err) || isUnknownScannerError(err)) {
		if retries <= s.maxRetries {
			// clean this table region cache and try again
			s.client.CleanRegionCache(s.table)
			// create new scanner and set startRow to lastResult
			s.id = 0
			if s.lastResult != nil {
				startKey = s.lastResult.Row
				s.skipFirst = true
			}
			s.server = nil
			s.location = nil
			log.Warnf("Retryint get data for %d time(s)", retries+1)
			retrySleep(retries + 1)
			return s.getData(startKey, retries+1)
		}
	}
	return rs, nil
}

func (s *Scan) processResponse(response pb.Message) ([]*ResultRow, error) {
	var res *proto.ScanResponse
	switch r := response.(type) {
	case *proto.ScanResponse:
		res = r
	case *exception:
		return nil, errors.New(r.msg)
	default:
		return nil, errors.Errorf("Invalid response seen [response: %#v]", response)
	}

	// Check whether response is nil.
	if res == nil {
		return nil, errors.Errorf("Empty response: [table=%s] [StartRow=%q] [StopRow=%q] ", s.table, s.StartRow, s.StopRow)
	}

	nextRegion := true
	s.nextStartKey = nil
	s.id = res.GetScannerId()

	results := res.GetResults()
	n := len(results)

	if (n == s.numCached) ||
		len(s.location.EndKey) == 0 ||
		(s.StopRow != nil && bytes.Compare(s.location.EndKey, s.StopRow) > 0 && n < s.numCached) ||
		res.GetMoreResultsInRegion() {
		nextRegion = false
	}

	var err error
	if nextRegion {
		s.nextStartKey = s.location.EndKey
		err = s.closeScan(s.server, s.location, s.id)
		if err != nil {
			return nil, errors.Trace(err)
		}
		s.server = nil
		s.location = nil
		s.id = 0
	}

	if n == 0 && !nextRegion {
		err = s.Close()
		if err != nil {
			return nil, errors.Trace(err)
		}
	}

	if s.skipFirst {
		results = results[1:]
		s.skipFirst = false
		n = len(results)
	}

	tbr := make([]*ResultRow, n)
	for i, v := range results {
		if v != nil {
			tbr[i] = NewResultRow(v)
		}
	}

	return tbr, nil
}

func (s *Scan) nextBatch() int {
	startKey := s.nextStartKey
	if startKey == nil {
		startKey = s.StartRow
	}

	// Notice: ignore error here.
	// TODO: add error check, now only add a log.
	rs, err := s.getData(startKey, 0)
	if err != nil {
		log.Errorf("scan next batch failed - [startKey=%q], %v", startKey, errors.ErrorStack(err))
	}

	// Current region get 0 data, try switch to next region.
	if len(rs) == 0 && len(s.nextStartKey) > 0 {
		// TODO: add error check, now only add a log.
		rs, err = s.getData(s.nextStartKey, 0)
		if err != nil {
			log.Errorf("scan next batch failed - [startKey=%q], %v", s.nextStartKey, errors.ErrorStack(err))
		}
	}

	s.cache = rs
	return len(s.cache)
}

func (s *Scan) Next() *ResultRow {
	if s.closed {
		return nil
	}
	var ret *ResultRow
	if len(s.cache) == 0 {
		n := s.nextBatch()
		// no data returned
		if n == 0 {
			return nil
		}
	}

	ret = s.cache[0]
	s.lastResult = ret
	s.cache = s.cache[1:]
	return ret
}

func (s *Scan) closeScan(server *connection, location *RegionInfo, id uint64) error {
	if server == nil || location == nil {
		return nil
	}

	req := &proto.ScanRequest{
		Region: &proto.RegionSpecifier{
			Type:  proto.RegionSpecifier_REGION_NAME.Enum(),
			Value: []byte(location.Name),
		},
		ScannerId:    pb.Uint64(id),
		CloseScanner: pb.Bool(true),
	}

	cl := newCall(req)
	err := server.call(cl)
	if err != nil {
		return errors.Trace(err)
	}

	// TODO: add exception check.
	<-cl.responseCh
	return nil
}

func (s *Scan) getServerAndLocation(table, startRow []byte) (*connection, *RegionInfo, error) {
	if s.server != nil && s.location != nil {
		return s.server, s.location, nil
	}

	var err error
	s.location, err = s.client.LocateRegion(table, startRow, true)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}

	s.server, err = s.client.getClientConn(s.location.Server)
	if err != nil {
		return nil, nil, errors.Trace(err)
	}
	return s.server, s.location, nil
}
