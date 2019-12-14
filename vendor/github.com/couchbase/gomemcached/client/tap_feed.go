package memcached

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"

	"github.com/couchbase/gomemcached"
	"github.com/couchbase/goutils/logging"
)

// TAP protocol docs: <http://www.couchbase.com/wiki/display/couchbase/TAP+Protocol>

// TapOpcode is the tap operation type (found in TapEvent)
type TapOpcode uint8

// Tap opcode values.
const (
	TapBeginBackfill = TapOpcode(iota)
	TapEndBackfill
	TapMutation
	TapDeletion
	TapCheckpointStart
	TapCheckpointEnd
	tapEndStream
)

const tapMutationExtraLen = 16

var tapOpcodeNames map[TapOpcode]string

func init() {
	tapOpcodeNames = map[TapOpcode]string{
		TapBeginBackfill:   "BeginBackfill",
		TapEndBackfill:     "EndBackfill",
		TapMutation:        "Mutation",
		TapDeletion:        "Deletion",
		TapCheckpointStart: "TapCheckpointStart",
		TapCheckpointEnd:   "TapCheckpointEnd",
		tapEndStream:       "EndStream",
	}
}

func (opcode TapOpcode) String() string {
	name := tapOpcodeNames[opcode]
	if name == "" {
		name = fmt.Sprintf("#%d", opcode)
	}
	return name
}

// TapEvent is a TAP notification of an operation on the server.
type TapEvent struct {
	Opcode     TapOpcode // Type of event
	VBucket    uint16    // VBucket this event applies to
	Flags      uint32    // Item flags
	Expiry     uint32    // Item expiration time
	Key, Value []byte    // Item key/value
	Cas        uint64
}

func makeTapEvent(req gomemcached.MCRequest) *TapEvent {
	event := TapEvent{
		VBucket: req.VBucket,
	}
	switch req.Opcode {
	case gomemcached.TAP_MUTATION:
		event.Opcode = TapMutation
		event.Key = req.Key
		event.Value = req.Body
		event.Cas = req.Cas
	case gomemcached.TAP_DELETE:
		event.Opcode = TapDeletion
		event.Key = req.Key
		event.Cas = req.Cas
	case gomemcached.TAP_CHECKPOINT_START:
		event.Opcode = TapCheckpointStart
	case gomemcached.TAP_CHECKPOINT_END:
		event.Opcode = TapCheckpointEnd
	case gomemcached.TAP_OPAQUE:
		if len(req.Extras) < 8+4 {
			return nil
		}
		switch op := int(binary.BigEndian.Uint32(req.Extras[8:])); op {
		case gomemcached.TAP_OPAQUE_INITIAL_VBUCKET_STREAM:
			event.Opcode = TapBeginBackfill
		case gomemcached.TAP_OPAQUE_CLOSE_BACKFILL:
			event.Opcode = TapEndBackfill
		case gomemcached.TAP_OPAQUE_CLOSE_TAP_STREAM:
			event.Opcode = tapEndStream
		case gomemcached.TAP_OPAQUE_ENABLE_AUTO_NACK:
			return nil
		case gomemcached.TAP_OPAQUE_ENABLE_CHECKPOINT_SYNC:
			return nil
		default:
			logging.Infof("TapFeed: Ignoring TAP_OPAQUE/%d", op)
			return nil // unknown opaque event
		}
	case gomemcached.NOOP:
		return nil // ignore
	default:
		logging.Infof("TapFeed: Ignoring %s", req.Opcode)
		return nil // unknown event
	}

	if len(req.Extras) >= tapMutationExtraLen &&
		(event.Opcode == TapMutation || event.Opcode == TapDeletion) {

		event.Flags = binary.BigEndian.Uint32(req.Extras[8:])
		event.Expiry = binary.BigEndian.Uint32(req.Extras[12:])
	}

	return &event
}

func (event TapEvent) String() string {
	switch event.Opcode {
	case TapBeginBackfill, TapEndBackfill, TapCheckpointStart, TapCheckpointEnd:
		return fmt.Sprintf("<TapEvent %s, vbucket=%d>",
			event.Opcode, event.VBucket)
	default:
		return fmt.Sprintf("<TapEvent %s, key=%q (%d bytes) flags=%x, exp=%d>",
			event.Opcode, event.Key, len(event.Value),
			event.Flags, event.Expiry)
	}
}

// TapArguments are parameters for requesting a TAP feed.
//
// Call DefaultTapArguments to get a default one.
type TapArguments struct {
	// Timestamp of oldest item to send.
	//
	// Use TapNoBackfill to suppress all past items.
	Backfill uint64
	// If set, server will disconnect after sending existing items.
	Dump bool
	// The indices of the vbuckets to watch; empty/nil to watch all.
	VBuckets []uint16
	// Transfers ownership of vbuckets during cluster rebalance.
	Takeover bool
	// If true, server will wait for client ACK after every notification.
	SupportAck bool
	// If true, client doesn't want values so server shouldn't send them.
	KeysOnly bool
	// If true, client wants the server to send checkpoint events.
	Checkpoint bool
	// Optional identifier to use for this client, to allow reconnects
	ClientName string
	// Registers this client (by name) till explicitly deregistered.
	RegisteredClient bool
}

// Value for TapArguments.Backfill denoting that no past events at all
// should be sent.
const TapNoBackfill = math.MaxUint64

// DefaultTapArguments returns a default set of parameter values to
// pass to StartTapFeed.
func DefaultTapArguments() TapArguments {
	return TapArguments{
		Backfill: TapNoBackfill,
	}
}

func (args *TapArguments) flags() []byte {
	var flags gomemcached.TapConnectFlag
	if args.Backfill != 0 {
		flags |= gomemcached.BACKFILL
	}
	if args.Dump {
		flags |= gomemcached.DUMP
	}
	if len(args.VBuckets) > 0 {
		flags |= gomemcached.LIST_VBUCKETS
	}
	if args.Takeover {
		flags |= gomemcached.TAKEOVER_VBUCKETS
	}
	if args.SupportAck {
		flags |= gomemcached.SUPPORT_ACK
	}
	if args.KeysOnly {
		flags |= gomemcached.REQUEST_KEYS_ONLY
	}
	if args.Checkpoint {
		flags |= gomemcached.CHECKPOINT
	}
	if args.RegisteredClient {
		flags |= gomemcached.REGISTERED_CLIENT
	}
	encoded := make([]byte, 4)
	binary.BigEndian.PutUint32(encoded, uint32(flags))
	return encoded
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func (args *TapArguments) bytes() (rv []byte) {
	buf := bytes.NewBuffer([]byte{})

	if args.Backfill > 0 {
		must(binary.Write(buf, binary.BigEndian, uint64(args.Backfill)))
	}

	if len(args.VBuckets) > 0 {
		must(binary.Write(buf, binary.BigEndian, uint16(len(args.VBuckets))))
		for i := 0; i < len(args.VBuckets); i++ {
			must(binary.Write(buf, binary.BigEndian, uint16(args.VBuckets[i])))
		}
	}
	return buf.Bytes()
}

// TapFeed represents a stream of events from a server.
type TapFeed struct {
	C      <-chan TapEvent
	Error  error
	closer chan bool
}

// StartTapFeed starts a TAP feed on a client connection.
//
// The events can be read from the returned channel.  The connection
// can no longer be used for other purposes; it's now reserved for
// receiving the TAP messages. To stop receiving events, close the
// client connection.
func (mc *Client) StartTapFeed(args TapArguments) (*TapFeed, error) {
	rq := &gomemcached.MCRequest{
		Opcode: gomemcached.TAP_CONNECT,
		Key:    []byte(args.ClientName),
		Extras: args.flags(),
		Body:   args.bytes()}

	err := mc.Transmit(rq)
	if err != nil {
		return nil, err
	}

	ch := make(chan TapEvent)
	feed := &TapFeed{
		C:      ch,
		closer: make(chan bool),
	}
	go mc.runFeed(ch, feed)
	return feed, nil
}

// TapRecvHook is called after every incoming tap packet is received.
var TapRecvHook func(*gomemcached.MCRequest, int, error)

// Internal goroutine that reads from the socket and writes events to
// the channel
func (mc *Client) runFeed(ch chan TapEvent, feed *TapFeed) {
	defer close(ch)
	var headerBuf [gomemcached.HDR_LEN]byte
loop:
	for {
		// Read the next request from the server.
		//
		//  (Can't call mc.Receive() because it reads a
		//  _response_ not a request.)
		var pkt gomemcached.MCRequest
		n, err := pkt.Receive(mc.conn, headerBuf[:])
		if TapRecvHook != nil {
			TapRecvHook(&pkt, n, err)
		}

		if err != nil {
			if err != io.EOF {
				feed.Error = err
			}
			break loop
		}

		//logging.Infof("** TapFeed received %#v : %q", pkt, pkt.Body)

		if pkt.Opcode == gomemcached.TAP_CONNECT {
			// This is not an event from the server; it's
			// an error response to my connect request.
			feed.Error = fmt.Errorf("tap connection failed: %s", pkt.Body)
			break loop
		}

		event := makeTapEvent(pkt)
		if event != nil {
			if event.Opcode == tapEndStream {
				break loop
			}

			select {
			case ch <- *event:
			case <-feed.closer:
				break loop
			}
		}

		if len(pkt.Extras) >= 4 {
			reqFlags := binary.BigEndian.Uint16(pkt.Extras[2:])
			if reqFlags&gomemcached.TAP_ACK != 0 {
				if _, err := mc.sendAck(&pkt); err != nil {
					feed.Error = err
					break loop
				}
			}
		}
	}
	if err := mc.Close(); err != nil {
		logging.Errorf("Error closing memcached client:  %v", err)
	}
}

func (mc *Client) sendAck(pkt *gomemcached.MCRequest) (int, error) {
	res := gomemcached.MCResponse{
		Opcode: pkt.Opcode,
		Opaque: pkt.Opaque,
		Status: gomemcached.SUCCESS,
	}
	return res.Transmit(mc.conn)
}

// Close terminates a TapFeed.
//
//  Call this if you stop using a TapFeed before its channel ends.
func (feed *TapFeed) Close() {
	close(feed.closer)
}
