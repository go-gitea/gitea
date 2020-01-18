// go implementation of upr client.
// See https://github.com/couchbaselabs/cbupr/blob/master/transport-spec.md
// TODO
// 1. Use a pool allocator to avoid garbage
package memcached

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/couchbase/gomemcached"
	"github.com/couchbase/goutils/logging"
	"strconv"
	"sync"
	"sync/atomic"
)

const uprMutationExtraLen = 30
const uprDeletetionExtraLen = 18
const uprDeletetionWithDeletionTimeExtraLen = 21
const uprSnapshotExtraLen = 20
const dcpSystemEventExtraLen = 13
const bufferAckThreshold = 0.2
const opaqueOpen = 0xBEAF0001
const opaqueFailover = 0xDEADBEEF
const uprDefaultNoopInterval = 120

// Counter on top of opaqueOpen that others can draw from for open and control msgs
var opaqueOpenCtrlWell uint32 = opaqueOpen

type PriorityType string

// high > medium > disabled > low
const (
	PriorityDisabled PriorityType = ""
	PriorityLow      PriorityType = "low"
	PriorityMed      PriorityType = "medium"
	PriorityHigh     PriorityType = "high"
)

type DcpStreamType int32

var UninitializedStream DcpStreamType = -1

const (
	NonCollectionStream    DcpStreamType = 0
	CollectionsNonStreamId DcpStreamType = iota
	CollectionsStreamId    DcpStreamType = iota
)

func (t DcpStreamType) String() string {
	switch t {
	case UninitializedStream:
		return "Un-Initialized Stream"
	case NonCollectionStream:
		return "Traditional Non-Collection Stream"
	case CollectionsNonStreamId:
		return "Collections Stream without StreamID"
	case CollectionsStreamId:
		return "Collection Stream with StreamID"
	default:
		return "Unknown Stream Type"
	}
}

// UprStream is per stream data structure over an UPR Connection.
type UprStream struct {
	Vbucket    uint16 // Vbucket id
	Vbuuid     uint64 // vbucket uuid
	StartSeq   uint64 // start sequence number
	EndSeq     uint64 // end sequence number
	connected  bool
	StreamType DcpStreamType
}

type FeedState int

const (
	FeedStateInitial = iota
	FeedStateOpened  = iota
	FeedStateClosed  = iota
)

func (fs FeedState) String() string {
	switch fs {
	case FeedStateInitial:
		return "Initial"
	case FeedStateOpened:
		return "Opened"
	case FeedStateClosed:
		return "Closed"
	default:
		return "Unknown"
	}
}

const (
	CompressionTypeStartMarker = iota // also means invalid
	CompressionTypeNone        = iota
	CompressionTypeSnappy      = iota
	CompressionTypeEndMarker   = iota // also means invalid
)

// kv_engine/include/mcbp/protocol/datatype.h
const (
	JSONDataType   uint8 = 1
	SnappyDataType uint8 = 2
	XattrDataType  uint8 = 4
)

type UprFeatures struct {
	Xattribute          bool
	CompressionType     int
	IncludeDeletionTime bool
	DcpPriority         PriorityType
	EnableExpiry        bool
	EnableStreamId      bool
}

/**
 * Used to handle multiple concurrent calls UprRequestStream() by UprFeed clients
 * It is expected that a client that calls UprRequestStream() more than once should issue
 * different "opaque" (version) numbers
 */
type opaqueStreamMap map[uint16]*UprStream // opaque -> stream

type vbStreamNegotiator struct {
	vbHandshakeMap map[uint16]opaqueStreamMap // vbno -> opaqueStreamMap
	mutex          sync.RWMutex
}

func (negotiator *vbStreamNegotiator) initialize() {
	negotiator.mutex.Lock()
	negotiator.vbHandshakeMap = make(map[uint16]opaqueStreamMap)
	negotiator.mutex.Unlock()
}

func (negotiator *vbStreamNegotiator) registerRequest(vbno, opaque uint16, vbuuid, startSequence, endSequence uint64) {
	negotiator.mutex.Lock()
	defer negotiator.mutex.Unlock()

	var osMap opaqueStreamMap
	var ok bool
	if osMap, ok = negotiator.vbHandshakeMap[vbno]; !ok {
		osMap = make(opaqueStreamMap)
		negotiator.vbHandshakeMap[vbno] = osMap
	}

	if _, ok = osMap[opaque]; !ok {
		osMap[opaque] = &UprStream{
			Vbucket:  vbno,
			Vbuuid:   vbuuid,
			StartSeq: startSequence,
			EndSeq:   endSequence,
		}
	}
}

func (negotiator *vbStreamNegotiator) getStreamsCntFromMap(vbno uint16) int {
	negotiator.mutex.RLock()
	defer negotiator.mutex.RUnlock()

	osmap, ok := negotiator.vbHandshakeMap[vbno]
	if !ok {
		return 0
	} else {
		return len(osmap)
	}
}

func (negotiator *vbStreamNegotiator) getStreamFromMap(vbno, opaque uint16) (*UprStream, error) {
	negotiator.mutex.RLock()
	defer negotiator.mutex.RUnlock()

	osmap, ok := negotiator.vbHandshakeMap[vbno]
	if !ok {
		return nil, fmt.Errorf("Error: stream for vb: %v does not exist", vbno)
	}

	stream, ok := osmap[opaque]
	if !ok {
		return nil, fmt.Errorf("Error: stream for vb: %v opaque: %v does not exist", vbno, opaque)
	}
	return stream, nil
}

func (negotiator *vbStreamNegotiator) deleteStreamFromMap(vbno, opaque uint16) {
	negotiator.mutex.Lock()
	defer negotiator.mutex.Unlock()

	osmap, ok := negotiator.vbHandshakeMap[vbno]
	if !ok {
		return
	}

	delete(osmap, opaque)
	if len(osmap) == 0 {
		delete(negotiator.vbHandshakeMap, vbno)
	}
}

func (negotiator *vbStreamNegotiator) handleStreamRequest(feed *UprFeed,
	headerBuf [gomemcached.HDR_LEN]byte, pktPtr *gomemcached.MCRequest, bytesReceivedFromDCP int,
	response *gomemcached.MCResponse) (*UprEvent, error) {
	var event *UprEvent

	if feed == nil || response == nil || pktPtr == nil {
		return nil, errors.New("Invalid inputs")
	}

	// Get Stream from negotiator map
	vbno := vbOpaque(response.Opaque)
	opaque := appOpaque(response.Opaque)

	stream, err := negotiator.getStreamFromMap(vbno, opaque)
	if err != nil {
		err = fmt.Errorf("Stream not found for vb %d: %#v", vbno, *pktPtr)
		logging.Errorf(err.Error())
		return nil, err
	}

	status, rb, flog, err := handleStreamRequest(response, headerBuf[:])

	if status == gomemcached.ROLLBACK {
		event = makeUprEvent(*pktPtr, stream, bytesReceivedFromDCP)
		event.Status = status
		// rollback stream
		logging.Infof("UPR_STREAMREQ with rollback %d for vb %d Failed: %v", rb, vbno, err)
		negotiator.deleteStreamFromMap(vbno, opaque)
	} else if status == gomemcached.SUCCESS {
		event = makeUprEvent(*pktPtr, stream, bytesReceivedFromDCP)
		event.Seqno = stream.StartSeq
		event.FailoverLog = flog
		event.Status = status
		feed.activateStream(vbno, opaque, stream)
		feed.negotiator.deleteStreamFromMap(vbno, opaque)
		logging.Infof("UPR_STREAMREQ for vb %d successful", vbno)

	} else if err != nil {
		logging.Errorf("UPR_STREAMREQ for vbucket %d erro %s", vbno, err.Error())
		event = &UprEvent{
			Opcode:  gomemcached.UPR_STREAMREQ,
			Status:  status,
			VBucket: vbno,
			Error:   err,
		}
		negotiator.deleteStreamFromMap(vbno, opaque)
	}
	return event, nil
}

func (negotiator *vbStreamNegotiator) cleanUpVbStreams(vbno uint16) {
	negotiator.mutex.Lock()
	defer negotiator.mutex.Unlock()

	delete(negotiator.vbHandshakeMap, vbno)
}

// UprFeed represents an UPR feed. A feed contains a connection to a single
// host and multiple vBuckets
type UprFeed struct {
	// lock for feed.vbstreams
	muVbstreams sync.RWMutex
	C           <-chan *UprEvent            // Exported channel for receiving UPR events
	negotiator  vbStreamNegotiator          // Used for pre-vbstreams, concurrent vb stream negotiation
	vbstreams   map[uint16]*UprStream       // official live vb->stream mapping
	closer      chan bool                   // closer
	conn        *Client                     // connection to UPR producer
	Error       error                       // error
	bytesRead   uint64                      // total bytes read on this connection
	toAckBytes  uint32                      // bytes client has read
	maxAckBytes uint32                      // Max buffer control ack bytes
	stats       UprStats                    // Stats for upr client
	transmitCh  chan *gomemcached.MCRequest // transmit command channel
	transmitCl  chan bool                   //  closer channel for transmit go-routine
	// if flag is true, upr feed will use ack from client to determine whether/when to send ack to DCP
	// if flag is false, upr feed will track how many bytes it has sent to client
	// and use that to determine whether/when to send ack to DCP
	ackByClient       bool
	feedState         FeedState
	muFeedState       sync.RWMutex
	activatedFeatures UprFeatures
	collectionEnabled bool // This is needed separately because parsing depends on this
	// DCP StreamID allows multiple filtered collection streams to share a single DCP Stream
	// It is not allowed once a regular/legacy stream was started originally
	streamsType        DcpStreamType
	initStreamTypeOnce sync.Once
}

// Exported interface - to allow for mocking
type UprFeedIface interface {
	Close()
	Closed() bool
	CloseStream(vbno, opaqueMSB uint16) error
	GetError() error
	GetUprStats() *UprStats
	ClientAck(event *UprEvent) error
	GetUprEventCh() <-chan *UprEvent
	StartFeed() error
	StartFeedWithConfig(datachan_len int) error
	UprOpen(name string, sequence uint32, bufSize uint32) error
	UprOpenWithXATTR(name string, sequence uint32, bufSize uint32) error
	UprOpenWithFeatures(name string, sequence uint32, bufSize uint32, features UprFeatures) (error, UprFeatures)
	UprRequestStream(vbno, opaqueMSB uint16, flags uint32, vuuid, startSequence, endSequence, snapStart, snapEnd uint64) error
	// Set DCP priority on an existing DCP connection. The command is sent asynchronously without waiting for a response
	SetPriorityAsync(p PriorityType) error

	// Various Collection-Type RequestStreams
	UprRequestCollectionsStream(vbno, opaqueMSB uint16, flags uint32, vbuuid, startSeq, endSeq, snapStart, snapEnd uint64, filter *CollectionsFilter) error
}

type UprStats struct {
	TotalBytes         uint64
	TotalMutation      uint64
	TotalBufferAckSent uint64
	TotalSnapShot      uint64
}

// error codes
var ErrorInvalidLog = errors.New("couchbase.errorInvalidLog")

func (flogp *FailoverLog) Latest() (vbuuid, seqno uint64, err error) {
	if flogp != nil {
		flog := *flogp
		latest := flog[len(flog)-1]
		return latest[0], latest[1], nil
	}
	return vbuuid, seqno, ErrorInvalidLog
}

func (feed *UprFeed) sendCommands(mc *Client) {
	transmitCh := feed.transmitCh
	transmitCl := feed.transmitCl
loop:
	for {
		select {
		case command := <-transmitCh:
			if err := mc.Transmit(command); err != nil {
				logging.Errorf("Failed to transmit command %s. Error %s", command.Opcode.String(), err.Error())
				// get feed to close and runFeed routine to exit
				feed.Close()
				break loop
			}

		case <-transmitCl:
			break loop
		}
	}

	// After sendCommands exits, write to transmitCh will block forever
	// when we write to transmitCh, e.g., at CloseStream(), we need to check feed closure to have an exit route

	logging.Infof("sendCommands exiting")
}

// Sets the specified stream as the connected stream for this vbno, and also cleans up negotiator
func (feed *UprFeed) activateStream(vbno, opaque uint16, stream *UprStream) error {
	feed.muVbstreams.Lock()
	defer feed.muVbstreams.Unlock()

	if feed.collectionEnabled {
		stream.StreamType = feed.streamsType
	}

	// Set this stream as the officially connected stream for this vb
	stream.connected = true
	feed.vbstreams[vbno] = stream
	return nil
}

func (feed *UprFeed) cleanUpVbStream(vbno uint16) {
	feed.muVbstreams.Lock()
	defer feed.muVbstreams.Unlock()

	delete(feed.vbstreams, vbno)
}

// NewUprFeed creates a new UPR Feed.
// TODO: Describe side-effects on bucket instance and its connection pool.
func (mc *Client) NewUprFeed() (*UprFeed, error) {
	return mc.NewUprFeedWithConfig(false /*ackByClient*/)
}

func (mc *Client) NewUprFeedWithConfig(ackByClient bool) (*UprFeed, error) {
	feed := &UprFeed{
		conn:              mc,
		closer:            make(chan bool, 1),
		vbstreams:         make(map[uint16]*UprStream),
		transmitCh:        make(chan *gomemcached.MCRequest),
		transmitCl:        make(chan bool),
		ackByClient:       ackByClient,
		collectionEnabled: mc.CollectionEnabled(),
		streamsType:       UninitializedStream,
	}

	feed.negotiator.initialize()

	go feed.sendCommands(mc)
	return feed, nil
}

func (mc *Client) NewUprFeedIface() (UprFeedIface, error) {
	return mc.NewUprFeed()
}

func (mc *Client) NewUprFeedWithConfigIface(ackByClient bool) (UprFeedIface, error) {
	return mc.NewUprFeedWithConfig(ackByClient)
}

func doUprOpen(mc *Client, name string, sequence uint32, features UprFeatures) error {
	rq := &gomemcached.MCRequest{
		Opcode: gomemcached.UPR_OPEN,
		Key:    []byte(name),
		Opaque: getUprOpenCtrlOpaque(),
	}

	rq.Extras = make([]byte, 8)
	binary.BigEndian.PutUint32(rq.Extras[:4], sequence)

	// opens a producer type connection
	flags := gomemcached.DCP_PRODUCER
	if features.Xattribute {
		flags = flags | gomemcached.DCP_OPEN_INCLUDE_XATTRS
	}
	if features.IncludeDeletionTime {
		flags = flags | gomemcached.DCP_OPEN_INCLUDE_DELETE_TIMES
	}
	binary.BigEndian.PutUint32(rq.Extras[4:], flags)

	return sendMcRequestSync(mc, rq)
}

// Synchronously send a memcached request and wait for the response
func sendMcRequestSync(mc *Client, req *gomemcached.MCRequest) error {
	if err := mc.Transmit(req); err != nil {
		return err
	}

	if res, err := mc.Receive(); err != nil {
		return err
	} else if req.Opcode != res.Opcode {
		return fmt.Errorf("unexpected #opcode sent %v received %v", req.Opcode, res.Opaque)
	} else if req.Opaque != res.Opaque {
		return fmt.Errorf("opaque mismatch, sent %v received %v", req.Opaque, res.Opaque)
	} else if res.Status != gomemcached.SUCCESS {
		return fmt.Errorf("error %v", res.Status)
	}
	return nil
}

// UprOpen to connect with a UPR producer.
// Name: name of te UPR connection
// sequence: sequence number for the connection
// bufsize: max size of the application
func (feed *UprFeed) UprOpen(name string, sequence uint32, bufSize uint32) error {
	var allFeaturesDisabled UprFeatures
	err, _ := feed.uprOpen(name, sequence, bufSize, allFeaturesDisabled)
	return err
}

// UprOpen with XATTR enabled.
func (feed *UprFeed) UprOpenWithXATTR(name string, sequence uint32, bufSize uint32) error {
	var onlyXattrEnabled UprFeatures
	onlyXattrEnabled.Xattribute = true
	err, _ := feed.uprOpen(name, sequence, bufSize, onlyXattrEnabled)
	return err
}

func (feed *UprFeed) UprOpenWithFeatures(name string, sequence uint32, bufSize uint32, features UprFeatures) (error, UprFeatures) {
	return feed.uprOpen(name, sequence, bufSize, features)
}

func (feed *UprFeed) SetPriorityAsync(p PriorityType) error {
	if !feed.isOpen() {
		// do not send this command if upr feed is not yet open, otherwise it may interfere with
		// feed start up process, which relies on synchronous message exchange with DCP.
		return fmt.Errorf("Upr feed is not open. State=%v", feed.getState())
	}

	return feed.setPriority(p, false /*sync*/)
}

func (feed *UprFeed) setPriority(p PriorityType, sync bool) error {
	rq := &gomemcached.MCRequest{
		Opcode: gomemcached.UPR_CONTROL,
		Key:    []byte("set_priority"),
		Body:   []byte(p),
		Opaque: getUprOpenCtrlOpaque(),
	}
	if sync {
		return sendMcRequestSync(feed.conn, rq)
	} else {
		return feed.writeToTransmitCh(rq)

	}
}

func (feed *UprFeed) uprOpen(name string, sequence uint32, bufSize uint32, features UprFeatures) (err error, activatedFeatures UprFeatures) {
	mc := feed.conn

	// First set this to an invalid value to state that the method hasn't gotten to executing this control yet
	activatedFeatures.CompressionType = CompressionTypeEndMarker

	if err = doUprOpen(mc, name, sequence, features); err != nil {
		return
	}

	activatedFeatures.Xattribute = features.Xattribute

	// send a UPR control message to set the window size for the this connection
	if bufSize > 0 {
		rq := &gomemcached.MCRequest{
			Opcode: gomemcached.UPR_CONTROL,
			Key:    []byte("connection_buffer_size"),
			Body:   []byte(strconv.Itoa(int(bufSize))),
			Opaque: getUprOpenCtrlOpaque(),
		}
		err = sendMcRequestSync(feed.conn, rq)
		if err != nil {
			return
		}
		feed.maxAckBytes = uint32(bufferAckThreshold * float32(bufSize))
	}

	// enable noop and set noop interval
	rq := &gomemcached.MCRequest{
		Opcode: gomemcached.UPR_CONTROL,
		Key:    []byte("enable_noop"),
		Body:   []byte("true"),
		Opaque: getUprOpenCtrlOpaque(),
	}
	err = sendMcRequestSync(feed.conn, rq)
	if err != nil {
		return
	}

	rq = &gomemcached.MCRequest{
		Opcode: gomemcached.UPR_CONTROL,
		Key:    []byte("set_noop_interval"),
		Body:   []byte(strconv.Itoa(int(uprDefaultNoopInterval))),
		Opaque: getUprOpenCtrlOpaque(),
	}
	err = sendMcRequestSync(feed.conn, rq)
	if err != nil {
		return
	}

	if features.CompressionType == CompressionTypeSnappy {
		activatedFeatures.CompressionType = CompressionTypeNone
		rq = &gomemcached.MCRequest{
			Opcode: gomemcached.UPR_CONTROL,
			Key:    []byte("force_value_compression"),
			Body:   []byte("true"),
			Opaque: getUprOpenCtrlOpaque(),
		}
		err = sendMcRequestSync(feed.conn, rq)
	} else if features.CompressionType == CompressionTypeEndMarker {
		err = fmt.Errorf("UPR_CONTROL Failed - Invalid CompressionType: %v", features.CompressionType)
	}
	if err != nil {
		return
	}
	activatedFeatures.CompressionType = features.CompressionType

	if features.DcpPriority != PriorityDisabled {
		err = feed.setPriority(features.DcpPriority, true /*sync*/)
		if err == nil {
			activatedFeatures.DcpPriority = features.DcpPriority
		} else {
			return
		}
	}

	if features.EnableExpiry {
		rq := &gomemcached.MCRequest{
			Opcode: gomemcached.UPR_CONTROL,
			Key:    []byte("enable_expiry_opcode"),
			Body:   []byte("true"),
			Opaque: getUprOpenCtrlOpaque(),
		}
		err = sendMcRequestSync(feed.conn, rq)
		if err != nil {
			return
		}
		activatedFeatures.EnableExpiry = true
	}

	if features.EnableStreamId {
		rq := &gomemcached.MCRequest{
			Opcode: gomemcached.UPR_CONTROL,
			Key:    []byte("enable_stream_id"),
			Body:   []byte("true"),
			Opaque: getUprOpenCtrlOpaque(),
		}
		err = sendMcRequestSync(feed.conn, rq)
		if err != nil {
			return
		}
		activatedFeatures.EnableStreamId = true
	}

	// everything is ok so far, set upr feed to open state
	feed.activatedFeatures = activatedFeatures
	feed.setOpen()
	return
}

// UprGetFailoverLog for given list of vbuckets.
func (mc *Client) UprGetFailoverLog(
	vb []uint16) (map[uint16]*FailoverLog, error) {

	rq := &gomemcached.MCRequest{
		Opcode: gomemcached.UPR_FAILOVERLOG,
		Opaque: opaqueFailover,
	}

	var allFeaturesDisabled UprFeatures
	if err := doUprOpen(mc, "FailoverLog", 0, allFeaturesDisabled); err != nil {
		return nil, fmt.Errorf("UPR_OPEN Failed %s", err.Error())
	}

	failoverLogs := make(map[uint16]*FailoverLog)
	for _, vBucket := range vb {
		rq.VBucket = vBucket
		if err := mc.Transmit(rq); err != nil {
			return nil, err
		}
		res, err := mc.Receive()

		if err != nil {
			return nil, fmt.Errorf("failed to receive %s", err.Error())
		} else if res.Opcode != gomemcached.UPR_FAILOVERLOG || res.Status != gomemcached.SUCCESS {
			return nil, fmt.Errorf("unexpected #opcode %v", res.Opcode)
		}

		flog, err := parseFailoverLog(res.Body)
		if err != nil {
			return nil, fmt.Errorf("unable to parse failover logs for vb %d", vb)
		}
		failoverLogs[vBucket] = flog
	}

	return failoverLogs, nil
}

// UprRequestStream for a single vbucket.
func (feed *UprFeed) UprRequestStream(vbno, opaqueMSB uint16, flags uint32,
	vuuid, startSequence, endSequence, snapStart, snapEnd uint64) error {

	return feed.UprRequestCollectionsStream(vbno, opaqueMSB, flags, vuuid, startSequence, endSequence, snapStart, snapEnd, nil)
}

func (feed *UprFeed) initStreamType(filter *CollectionsFilter) (err error) {
	if filter != nil && filter.UseStreamId && !feed.activatedFeatures.EnableStreamId {
		err = fmt.Errorf("Cannot use streamID based filter if the feed was not started with the streamID feature")
		return
	}

	streamInitFunc := func() {
		if feed.streamsType != UninitializedStream {
			// Shouldn't happen
			err = fmt.Errorf("The current feed has already been started in %v mode", feed.streamsType.String())
		} else {
			if !feed.collectionEnabled {
				feed.streamsType = NonCollectionStream
			} else {
				if filter != nil && filter.UseStreamId {
					feed.streamsType = CollectionsStreamId
				} else {
					feed.streamsType = CollectionsNonStreamId
				}
			}
		}
	}
	feed.initStreamTypeOnce.Do(streamInitFunc)
	return
}

func (feed *UprFeed) UprRequestCollectionsStream(vbno, opaqueMSB uint16, flags uint32,
	vbuuid, startSequence, endSequence, snapStart, snapEnd uint64, filter *CollectionsFilter) error {

	err := feed.initStreamType(filter)
	if err != nil {
		return err
	}

	var mcRequestBody []byte
	if filter != nil {
		err = filter.IsValid()
		if err != nil {
			return err
		}
		mcRequestBody, err = filter.ToStreamReqBody()
		if err != nil {
			return err
		}
	}

	rq := &gomemcached.MCRequest{
		Opcode:  gomemcached.UPR_STREAMREQ,
		VBucket: vbno,
		Opaque:  composeOpaque(vbno, opaqueMSB),
		Body:    mcRequestBody,
	}

	rq.Extras = make([]byte, 48) // #Extras
	binary.BigEndian.PutUint32(rq.Extras[:4], flags)
	binary.BigEndian.PutUint32(rq.Extras[4:8], uint32(0))
	binary.BigEndian.PutUint64(rq.Extras[8:16], startSequence)
	binary.BigEndian.PutUint64(rq.Extras[16:24], endSequence)
	binary.BigEndian.PutUint64(rq.Extras[24:32], vbuuid)
	binary.BigEndian.PutUint64(rq.Extras[32:40], snapStart)
	binary.BigEndian.PutUint64(rq.Extras[40:48], snapEnd)

	feed.negotiator.registerRequest(vbno, opaqueMSB, vbuuid, startSequence, endSequence)
	// Any client that has ever called this method, regardless of return code,
	// should expect a potential UPR_CLOSESTREAM message due to this new map entry prior to Transmit.

	if err = feed.conn.Transmit(rq); err != nil {
		logging.Errorf("Error in StreamRequest %s", err.Error())
		// If an error occurs during transmit, then the UPRFeed will keep the stream
		// in the vbstreams map. This is to prevent nil lookup from any previously
		// sent stream requests.
		return err
	}

	return nil
}

// CloseStream for specified vbucket.
func (feed *UprFeed) CloseStream(vbno, opaqueMSB uint16) error {

	err := feed.validateCloseStream(vbno)
	if err != nil {
		logging.Infof("CloseStream for %v has been skipped because of error %v", vbno, err)
		return err
	}

	closeStream := &gomemcached.MCRequest{
		Opcode:  gomemcached.UPR_CLOSESTREAM,
		VBucket: vbno,
		Opaque:  composeOpaque(vbno, opaqueMSB),
	}

	feed.writeToTransmitCh(closeStream)

	return nil
}

func (feed *UprFeed) GetUprEventCh() <-chan *UprEvent {
	return feed.C
}

func (feed *UprFeed) GetError() error {
	return feed.Error
}

func (feed *UprFeed) validateCloseStream(vbno uint16) error {
	feed.muVbstreams.RLock()
	nilVbStream := feed.vbstreams[vbno] == nil
	feed.muVbstreams.RUnlock()

	if nilVbStream && (feed.negotiator.getStreamsCntFromMap(vbno) == 0) {
		return fmt.Errorf("Stream for vb %d has not been requested", vbno)
	}

	return nil
}

func (feed *UprFeed) writeToTransmitCh(rq *gomemcached.MCRequest) error {
	// write to transmitCh may block forever if sendCommands has exited
	// check for feed closure to have an exit route in this case
	select {
	case <-feed.closer:
		errMsg := fmt.Sprintf("Abort sending request to transmitCh because feed has been closed. request=%v", rq)
		logging.Infof(errMsg)
		return errors.New(errMsg)
	case feed.transmitCh <- rq:
	}
	return nil
}

// StartFeed to start the upper feed.
func (feed *UprFeed) StartFeed() error {
	return feed.StartFeedWithConfig(10)
}

func (feed *UprFeed) StartFeedWithConfig(datachan_len int) error {
	ch := make(chan *UprEvent, datachan_len)
	feed.C = ch
	go feed.runFeed(ch)
	return nil
}

func parseFailoverLog(body []byte) (*FailoverLog, error) {

	if len(body)%16 != 0 {
		err := fmt.Errorf("invalid body length %v, in failover-log", len(body))
		return nil, err
	}
	log := make(FailoverLog, len(body)/16)
	for i, j := 0, 0; i < len(body); i += 16 {
		vuuid := binary.BigEndian.Uint64(body[i : i+8])
		seqno := binary.BigEndian.Uint64(body[i+8 : i+16])
		log[j] = [2]uint64{vuuid, seqno}
		j++
	}
	return &log, nil
}

func handleStreamRequest(
	res *gomemcached.MCResponse,
	headerBuf []byte,
) (gomemcached.Status, uint64, *FailoverLog, error) {

	var rollback uint64
	var err error

	switch {
	case res.Status == gomemcached.ROLLBACK:
		logging.Infof("Rollback response. body=%v, headerBuf=%v\n", res.Body, headerBuf)
		rollback = binary.BigEndian.Uint64(res.Body)
		logging.Infof("Rollback seqno is %v for response with opaque %v\n", rollback, res.Opaque)
		return res.Status, rollback, nil, nil

	case res.Status != gomemcached.SUCCESS:
		err = fmt.Errorf("unexpected status %v for response with opaque %v", res.Status, res.Opaque)
		return res.Status, 0, nil, err
	}

	flog, err := parseFailoverLog(res.Body[:])
	return res.Status, rollback, flog, err
}

// generate stream end responses for all active vb streams
func (feed *UprFeed) doStreamClose(ch chan *UprEvent) {
	feed.muVbstreams.RLock()

	uprEvents := make([]*UprEvent, len(feed.vbstreams))
	index := 0
	for vbno, stream := range feed.vbstreams {
		uprEvent := &UprEvent{
			VBucket: vbno,
			VBuuid:  stream.Vbuuid,
			Opcode:  gomemcached.UPR_STREAMEND,
		}
		uprEvents[index] = uprEvent
		index++
	}

	// release the lock before sending uprEvents to ch, which may block
	feed.muVbstreams.RUnlock()

loop:
	for _, uprEvent := range uprEvents {
		select {
		case ch <- uprEvent:
		case <-feed.closer:
			logging.Infof("Feed has been closed. Aborting doStreamClose.")
			break loop
		}
	}
}

func (feed *UprFeed) runFeed(ch chan *UprEvent) {
	defer close(ch)
	var headerBuf [gomemcached.HDR_LEN]byte
	var pkt gomemcached.MCRequest
	var event *UprEvent

	mc := feed.conn.Hijack()
	uprStats := &feed.stats

loop:
	for {
		select {
		case <-feed.closer:
			logging.Infof("Feed has been closed. Exiting.")
			break loop
		default:
			bytes, err := pkt.Receive(mc, headerBuf[:])
			if err != nil {
				logging.Errorf("Error in receive %s", err.Error())
				feed.Error = err
				// send all the stream close messages to the client
				feed.doStreamClose(ch)
				break loop
			} else {
				event = nil
				res := &gomemcached.MCResponse{
					Opcode: pkt.Opcode,
					Cas:    pkt.Cas,
					Opaque: pkt.Opaque,
					Status: gomemcached.Status(pkt.VBucket),
					Extras: pkt.Extras,
					Key:    pkt.Key,
					Body:   pkt.Body,
				}

				vb := vbOpaque(pkt.Opaque)
				appOpaque := appOpaque(pkt.Opaque)
				uprStats.TotalBytes = uint64(bytes)

				feed.muVbstreams.RLock()
				stream := feed.vbstreams[vb]
				feed.muVbstreams.RUnlock()

				switch pkt.Opcode {
				case gomemcached.UPR_STREAMREQ:
					event, err = feed.negotiator.handleStreamRequest(feed, headerBuf, &pkt, bytes, res)
					if err != nil {
						logging.Infof(err.Error())
						break loop
					}
				case gomemcached.UPR_MUTATION,
					gomemcached.UPR_DELETION,
					gomemcached.UPR_EXPIRATION:
					if stream == nil {
						logging.Infof("Stream not found for vb %d: %#v", vb, pkt)
						break loop
					}
					event = makeUprEvent(pkt, stream, bytes)
					uprStats.TotalMutation++

				case gomemcached.UPR_STREAMEND:
					if stream == nil {
						logging.Infof("Stream not found for vb %d: %#v", vb, pkt)
						break loop
					}
					//stream has ended
					event = makeUprEvent(pkt, stream, bytes)
					logging.Infof("Stream Ended for vb %d", vb)

					feed.negotiator.deleteStreamFromMap(vb, appOpaque)
					feed.cleanUpVbStream(vb)

				case gomemcached.UPR_SNAPSHOT:
					if stream == nil {
						logging.Infof("Stream not found for vb %d: %#v", vb, pkt)
						break loop
					}
					// snapshot marker
					event = makeUprEvent(pkt, stream, bytes)
					uprStats.TotalSnapShot++

				case gomemcached.UPR_FLUSH:
					if stream == nil {
						logging.Infof("Stream not found for vb %d: %#v", vb, pkt)
						break loop
					}
					// special processing for flush ?
					event = makeUprEvent(pkt, stream, bytes)

				case gomemcached.UPR_CLOSESTREAM:
					if stream == nil {
						logging.Infof("Stream not found for vb %d: %#v", vb, pkt)
						break loop
					}
					event = makeUprEvent(pkt, stream, bytes)
					event.Opcode = gomemcached.UPR_STREAMEND // opcode re-write !!
					logging.Infof("Stream Closed for vb %d StreamEnd simulated", vb)

					feed.negotiator.deleteStreamFromMap(vb, appOpaque)
					feed.cleanUpVbStream(vb)

				case gomemcached.UPR_ADDSTREAM:
					logging.Infof("Opcode %v not implemented", pkt.Opcode)

				case gomemcached.UPR_CONTROL, gomemcached.UPR_BUFFERACK:
					if res.Status != gomemcached.SUCCESS {
						logging.Infof("Opcode %v received status %d", pkt.Opcode.String(), res.Status)
					}

				case gomemcached.UPR_NOOP:
					// send a NOOP back
					noop := &gomemcached.MCResponse{
						Opcode: gomemcached.UPR_NOOP,
						Opaque: pkt.Opaque,
					}

					if err := feed.conn.TransmitResponse(noop); err != nil {
						logging.Warnf("failed to transmit command %s. Error %s", noop.Opcode.String(), err.Error())
					}
				case gomemcached.DCP_SYSTEM_EVENT:
					if stream == nil {
						logging.Infof("Stream not found for vb %d: %#v", vb, pkt)
						break loop
					}
					event = makeUprEvent(pkt, stream, bytes)
				default:
					logging.Infof("Recived an unknown response for vbucket %d", vb)
				}
			}

			if event != nil {
				select {
				case ch <- event:
				case <-feed.closer:
					logging.Infof("Feed has been closed. Skip sending events. Exiting.")
					break loop
				}

				feed.muVbstreams.RLock()
				l := len(feed.vbstreams)
				feed.muVbstreams.RUnlock()

				if event.Opcode == gomemcached.UPR_CLOSESTREAM && l == 0 {
					logging.Infof("No more streams")
				}
			}

			if !feed.ackByClient {
				// if client does not ack, do the ack check now
				feed.sendBufferAckIfNeeded(event)
			}
		}
	}

	// make sure that feed is closed before we signal transmitCl and exit runFeed
	feed.Close()

	close(feed.transmitCl)
	logging.Infof("runFeed exiting")
}

// Client, after completing processing of an UprEvent, need to call this API to notify UprFeed,
// so that UprFeed can update its ack bytes stats and send ack to DCP if needed
// Client needs to set ackByClient flag to true in NewUprFeedWithConfig() call as a prerequisite for this call to work
// This API is not thread safe. Caller should NOT have more than one go rountine calling this API
func (feed *UprFeed) ClientAck(event *UprEvent) error {
	if !feed.ackByClient {
		return errors.New("Upr feed does not have ackByclient flag set")
	}
	feed.sendBufferAckIfNeeded(event)
	return nil
}

// increment ack bytes if the event needs to be acked to DCP
// send buffer ack if enough ack bytes have been accumulated
func (feed *UprFeed) sendBufferAckIfNeeded(event *UprEvent) {
	if event == nil || event.AckSize == 0 {
		// this indicates that there is no need to ack to DCP
		return
	}

	totalBytes := feed.toAckBytes + event.AckSize
	if totalBytes > feed.maxAckBytes {
		feed.toAckBytes = 0
		feed.sendBufferAck(totalBytes)
	} else {
		feed.toAckBytes = totalBytes
	}
}

// send buffer ack to dcp
func (feed *UprFeed) sendBufferAck(sendSize uint32) {
	bufferAck := &gomemcached.MCRequest{
		Opcode: gomemcached.UPR_BUFFERACK,
	}
	bufferAck.Extras = make([]byte, 4)
	binary.BigEndian.PutUint32(bufferAck.Extras[:4], uint32(sendSize))
	feed.writeToTransmitCh(bufferAck)
	feed.stats.TotalBufferAckSent++
}

func (feed *UprFeed) GetUprStats() *UprStats {
	return &feed.stats
}

func composeOpaque(vbno, opaqueMSB uint16) uint32 {
	return (uint32(opaqueMSB) << 16) | uint32(vbno)
}

func getUprOpenCtrlOpaque() uint32 {
	return atomic.AddUint32(&opaqueOpenCtrlWell, 1)
}

func appOpaque(opq32 uint32) uint16 {
	return uint16((opq32 & 0xFFFF0000) >> 16)
}

func vbOpaque(opq32 uint32) uint16 {
	return uint16(opq32 & 0xFFFF)
}

// Close this UprFeed.
func (feed *UprFeed) Close() {
	feed.muFeedState.Lock()
	defer feed.muFeedState.Unlock()
	if feed.feedState != FeedStateClosed {
		close(feed.closer)
		feed.feedState = FeedStateClosed
		feed.negotiator.initialize()
	}
}

// check if the UprFeed has been closed
func (feed *UprFeed) Closed() bool {
	feed.muFeedState.RLock()
	defer feed.muFeedState.RUnlock()
	return feed.feedState == FeedStateClosed
}

// set upr feed to opened state after initialization is done
func (feed *UprFeed) setOpen() {
	feed.muFeedState.Lock()
	defer feed.muFeedState.Unlock()
	feed.feedState = FeedStateOpened
}

func (feed *UprFeed) isOpen() bool {
	feed.muFeedState.RLock()
	defer feed.muFeedState.RUnlock()
	return feed.feedState == FeedStateOpened
}

func (feed *UprFeed) getState() FeedState {
	feed.muFeedState.RLock()
	defer feed.muFeedState.RUnlock()
	return feed.feedState
}
