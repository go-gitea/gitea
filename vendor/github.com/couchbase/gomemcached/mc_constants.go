// Package gomemcached is binary protocol packet formats and constants.
package gomemcached

import (
	"fmt"
)

const (
	REQ_MAGIC = 0x80
	RES_MAGIC = 0x81
)

// CommandCode for memcached packets.
type CommandCode uint8

const (
	GET        = CommandCode(0x00)
	SET        = CommandCode(0x01)
	ADD        = CommandCode(0x02)
	REPLACE    = CommandCode(0x03)
	DELETE     = CommandCode(0x04)
	INCREMENT  = CommandCode(0x05)
	DECREMENT  = CommandCode(0x06)
	QUIT       = CommandCode(0x07)
	FLUSH      = CommandCode(0x08)
	GETQ       = CommandCode(0x09)
	NOOP       = CommandCode(0x0a)
	VERSION    = CommandCode(0x0b)
	GETK       = CommandCode(0x0c)
	GETKQ      = CommandCode(0x0d)
	APPEND     = CommandCode(0x0e)
	PREPEND    = CommandCode(0x0f)
	STAT       = CommandCode(0x10)
	SETQ       = CommandCode(0x11)
	ADDQ       = CommandCode(0x12)
	REPLACEQ   = CommandCode(0x13)
	DELETEQ    = CommandCode(0x14)
	INCREMENTQ = CommandCode(0x15)
	DECREMENTQ = CommandCode(0x16)
	QUITQ      = CommandCode(0x17)
	FLUSHQ     = CommandCode(0x18)
	APPENDQ    = CommandCode(0x19)
	AUDIT      = CommandCode(0x27)
	PREPENDQ   = CommandCode(0x1a)
	GAT        = CommandCode(0x1d)
	HELLO      = CommandCode(0x1f)
	RGET       = CommandCode(0x30)
	RSET       = CommandCode(0x31)
	RSETQ      = CommandCode(0x32)
	RAPPEND    = CommandCode(0x33)
	RAPPENDQ   = CommandCode(0x34)
	RPREPEND   = CommandCode(0x35)
	RPREPENDQ  = CommandCode(0x36)
	RDELETE    = CommandCode(0x37)
	RDELETEQ   = CommandCode(0x38)
	RINCR      = CommandCode(0x39)
	RINCRQ     = CommandCode(0x3a)
	RDECR      = CommandCode(0x3b)
	RDECRQ     = CommandCode(0x3c)

	SASL_LIST_MECHS = CommandCode(0x20)
	SASL_AUTH       = CommandCode(0x21)
	SASL_STEP       = CommandCode(0x22)

	SET_VBUCKET = CommandCode(0x3d)

	TAP_CONNECT          = CommandCode(0x40) // Client-sent request to initiate Tap feed
	TAP_MUTATION         = CommandCode(0x41) // Notification of a SET/ADD/REPLACE/etc. on the server
	TAP_DELETE           = CommandCode(0x42) // Notification of a DELETE on the server
	TAP_FLUSH            = CommandCode(0x43) // Replicates a flush_all command
	TAP_OPAQUE           = CommandCode(0x44) // Opaque control data from the engine
	TAP_VBUCKET_SET      = CommandCode(0x45) // Sets state of vbucket in receiver (used in takeover)
	TAP_CHECKPOINT_START = CommandCode(0x46) // Notifies start of new checkpoint
	TAP_CHECKPOINT_END   = CommandCode(0x47) // Notifies end of checkpoint

	UPR_OPEN        = CommandCode(0x50) // Open a UPR connection with a name
	UPR_ADDSTREAM   = CommandCode(0x51) // Sent by ebucketMigrator to UPR Consumer
	UPR_CLOSESTREAM = CommandCode(0x52) // Sent by eBucketMigrator to UPR Consumer
	UPR_FAILOVERLOG = CommandCode(0x54) // Request failover logs
	UPR_STREAMREQ   = CommandCode(0x53) // Stream request from consumer to producer
	UPR_STREAMEND   = CommandCode(0x55) // Sent by producer when it has no more messages to stream
	UPR_SNAPSHOT    = CommandCode(0x56) // Start of a new snapshot
	UPR_MUTATION    = CommandCode(0x57) // Key mutation
	UPR_DELETION    = CommandCode(0x58) // Key deletion
	UPR_EXPIRATION  = CommandCode(0x59) // Key expiration
	UPR_FLUSH       = CommandCode(0x5a) // Delete all the data for a vbucket
	UPR_NOOP        = CommandCode(0x5c) // UPR NOOP
	UPR_BUFFERACK   = CommandCode(0x5d) // UPR Buffer Acknowledgement
	UPR_CONTROL     = CommandCode(0x5e) // Set flow control params

	SELECT_BUCKET = CommandCode(0x89) // Select bucket

	OBSERVE_SEQNO = CommandCode(0x91) // Sequence Number based Observe
	OBSERVE       = CommandCode(0x92)

	GET_META            = CommandCode(0xA0) // Get meta. returns with expiry, flags, cas etc
	SUBDOC_GET          = CommandCode(0xc5) // Get subdoc. Returns with xattrs
	SUBDOC_MULTI_LOOKUP = CommandCode(0xd0) // Multi lookup. Doc xattrs and meta.
)

// command codes that are counted toward DCP control buffer
// when DCP clients receive DCP messages with these command codes, they need to provide acknowledgement
var BufferedCommandCodeMap = map[CommandCode]bool{
	SET_VBUCKET:    true,
	UPR_STREAMEND:  true,
	UPR_SNAPSHOT:   true,
	UPR_MUTATION:   true,
	UPR_DELETION:   true,
	UPR_EXPIRATION: true}

// Status field for memcached response.
type Status uint16

// Matches with protocol_binary.h as source of truth
const (
	SUCCESS         = Status(0x00)
	KEY_ENOENT      = Status(0x01)
	KEY_EEXISTS     = Status(0x02)
	E2BIG           = Status(0x03)
	EINVAL          = Status(0x04)
	NOT_STORED      = Status(0x05)
	DELTA_BADVAL    = Status(0x06)
	NOT_MY_VBUCKET  = Status(0x07)
	NO_BUCKET       = Status(0x08)
	LOCKED          = Status(0x09)
	AUTH_STALE      = Status(0x1f)
	AUTH_ERROR      = Status(0x20)
	AUTH_CONTINUE   = Status(0x21)
	ERANGE          = Status(0x22)
	ROLLBACK        = Status(0x23)
	EACCESS         = Status(0x24)
	NOT_INITIALIZED = Status(0x25)
	UNKNOWN_COMMAND = Status(0x81)
	ENOMEM          = Status(0x82)
	NOT_SUPPORTED   = Status(0x83)
	EINTERNAL       = Status(0x84)
	EBUSY           = Status(0x85)
	TMPFAIL         = Status(0x86)

	// SUBDOC
	SUBDOC_PATH_NOT_FOUND             = Status(0xc0)
	SUBDOC_BAD_MULTI                  = Status(0xcc)
	SUBDOC_MULTI_PATH_FAILURE_DELETED = Status(0xd3)
)

// for log redaction
const (
	UdTagBegin = "<ud>"
	UdTagEnd   = "</ud>"
)

var isFatal = map[Status]bool{
	DELTA_BADVAL:  true,
	NO_BUCKET:     true,
	AUTH_STALE:    true,
	AUTH_ERROR:    true,
	ERANGE:        true,
	ROLLBACK:      true,
	EACCESS:       true,
	ENOMEM:        true,
	NOT_SUPPORTED: true,
}

// the producer/consumer bit in dcp flags
var DCP_PRODUCER uint32 = 0x01

// the include XATTRS bit in dcp flags
var DCP_OPEN_INCLUDE_XATTRS uint32 = 0x04

// the include deletion time bit in dcp flags
var DCP_OPEN_INCLUDE_DELETE_TIMES uint32 = 0x20

// Datatype to Include XATTRS in SUBDOC GET
var SUBDOC_FLAG_XATTR uint8 = 0x04

// MCItem is an internal representation of an item.
type MCItem struct {
	Cas               uint64
	Flags, Expiration uint32
	Data              []byte
}

// Number of bytes in a binary protocol header.
const HDR_LEN = 24

// Mapping of CommandCode -> name of command (not exhaustive)
var CommandNames map[CommandCode]string

// StatusNames human readable names for memcached response.
var StatusNames map[Status]string

func init() {
	CommandNames = make(map[CommandCode]string)
	CommandNames[GET] = "GET"
	CommandNames[SET] = "SET"
	CommandNames[ADD] = "ADD"
	CommandNames[REPLACE] = "REPLACE"
	CommandNames[DELETE] = "DELETE"
	CommandNames[INCREMENT] = "INCREMENT"
	CommandNames[DECREMENT] = "DECREMENT"
	CommandNames[QUIT] = "QUIT"
	CommandNames[FLUSH] = "FLUSH"
	CommandNames[GETQ] = "GETQ"
	CommandNames[NOOP] = "NOOP"
	CommandNames[VERSION] = "VERSION"
	CommandNames[GETK] = "GETK"
	CommandNames[GETKQ] = "GETKQ"
	CommandNames[APPEND] = "APPEND"
	CommandNames[PREPEND] = "PREPEND"
	CommandNames[STAT] = "STAT"
	CommandNames[SETQ] = "SETQ"
	CommandNames[ADDQ] = "ADDQ"
	CommandNames[REPLACEQ] = "REPLACEQ"
	CommandNames[DELETEQ] = "DELETEQ"
	CommandNames[INCREMENTQ] = "INCREMENTQ"
	CommandNames[DECREMENTQ] = "DECREMENTQ"
	CommandNames[QUITQ] = "QUITQ"
	CommandNames[FLUSHQ] = "FLUSHQ"
	CommandNames[APPENDQ] = "APPENDQ"
	CommandNames[PREPENDQ] = "PREPENDQ"
	CommandNames[RGET] = "RGET"
	CommandNames[RSET] = "RSET"
	CommandNames[RSETQ] = "RSETQ"
	CommandNames[RAPPEND] = "RAPPEND"
	CommandNames[RAPPENDQ] = "RAPPENDQ"
	CommandNames[RPREPEND] = "RPREPEND"
	CommandNames[RPREPENDQ] = "RPREPENDQ"
	CommandNames[RDELETE] = "RDELETE"
	CommandNames[RDELETEQ] = "RDELETEQ"
	CommandNames[RINCR] = "RINCR"
	CommandNames[RINCRQ] = "RINCRQ"
	CommandNames[RDECR] = "RDECR"
	CommandNames[RDECRQ] = "RDECRQ"

	CommandNames[SASL_LIST_MECHS] = "SASL_LIST_MECHS"
	CommandNames[SASL_AUTH] = "SASL_AUTH"
	CommandNames[SASL_STEP] = "SASL_STEP"

	CommandNames[TAP_CONNECT] = "TAP_CONNECT"
	CommandNames[TAP_MUTATION] = "TAP_MUTATION"
	CommandNames[TAP_DELETE] = "TAP_DELETE"
	CommandNames[TAP_FLUSH] = "TAP_FLUSH"
	CommandNames[TAP_OPAQUE] = "TAP_OPAQUE"
	CommandNames[TAP_VBUCKET_SET] = "TAP_VBUCKET_SET"
	CommandNames[TAP_CHECKPOINT_START] = "TAP_CHECKPOINT_START"
	CommandNames[TAP_CHECKPOINT_END] = "TAP_CHECKPOINT_END"

	CommandNames[UPR_OPEN] = "UPR_OPEN"
	CommandNames[UPR_ADDSTREAM] = "UPR_ADDSTREAM"
	CommandNames[UPR_CLOSESTREAM] = "UPR_CLOSESTREAM"
	CommandNames[UPR_FAILOVERLOG] = "UPR_FAILOVERLOG"
	CommandNames[UPR_STREAMREQ] = "UPR_STREAMREQ"
	CommandNames[UPR_STREAMEND] = "UPR_STREAMEND"
	CommandNames[UPR_SNAPSHOT] = "UPR_SNAPSHOT"
	CommandNames[UPR_MUTATION] = "UPR_MUTATION"
	CommandNames[UPR_DELETION] = "UPR_DELETION"
	CommandNames[UPR_EXPIRATION] = "UPR_EXPIRATION"
	CommandNames[UPR_FLUSH] = "UPR_FLUSH"
	CommandNames[UPR_NOOP] = "UPR_NOOP"
	CommandNames[UPR_BUFFERACK] = "UPR_BUFFERACK"
	CommandNames[UPR_CONTROL] = "UPR_CONTROL"
	CommandNames[SUBDOC_GET] = "SUBDOC_GET"
	CommandNames[SUBDOC_MULTI_LOOKUP] = "SUBDOC_MULTI_LOOKUP"

	StatusNames = make(map[Status]string)
	StatusNames[SUCCESS] = "SUCCESS"
	StatusNames[KEY_ENOENT] = "KEY_ENOENT"
	StatusNames[KEY_EEXISTS] = "KEY_EEXISTS"
	StatusNames[E2BIG] = "E2BIG"
	StatusNames[EINVAL] = "EINVAL"
	StatusNames[NOT_STORED] = "NOT_STORED"
	StatusNames[DELTA_BADVAL] = "DELTA_BADVAL"
	StatusNames[NOT_MY_VBUCKET] = "NOT_MY_VBUCKET"
	StatusNames[NO_BUCKET] = "NO_BUCKET"
	StatusNames[AUTH_STALE] = "AUTH_STALE"
	StatusNames[AUTH_ERROR] = "AUTH_ERROR"
	StatusNames[AUTH_CONTINUE] = "AUTH_CONTINUE"
	StatusNames[ERANGE] = "ERANGE"
	StatusNames[ROLLBACK] = "ROLLBACK"
	StatusNames[EACCESS] = "EACCESS"
	StatusNames[NOT_INITIALIZED] = "NOT_INITIALIZED"
	StatusNames[UNKNOWN_COMMAND] = "UNKNOWN_COMMAND"
	StatusNames[ENOMEM] = "ENOMEM"
	StatusNames[NOT_SUPPORTED] = "NOT_SUPPORTED"
	StatusNames[EINTERNAL] = "EINTERNAL"
	StatusNames[EBUSY] = "EBUSY"
	StatusNames[TMPFAIL] = "TMPFAIL"
	StatusNames[SUBDOC_PATH_NOT_FOUND] = "SUBDOC_PATH_NOT_FOUND"
	StatusNames[SUBDOC_BAD_MULTI] = "SUBDOC_BAD_MULTI"

}

// String an op code.
func (o CommandCode) String() (rv string) {
	rv = CommandNames[o]
	if rv == "" {
		rv = fmt.Sprintf("0x%02x", int(o))
	}
	return rv
}

// String an op code.
func (s Status) String() (rv string) {
	rv = StatusNames[s]
	if rv == "" {
		rv = fmt.Sprintf("0x%02x", int(s))
	}
	return rv
}

// IsQuiet will return true if a command is a "quiet" command.
func (o CommandCode) IsQuiet() bool {
	switch o {
	case GETQ,
		GETKQ,
		SETQ,
		ADDQ,
		REPLACEQ,
		DELETEQ,
		INCREMENTQ,
		DECREMENTQ,
		QUITQ,
		FLUSHQ,
		APPENDQ,
		PREPENDQ,
		RSETQ,
		RAPPENDQ,
		RPREPENDQ,
		RDELETEQ,
		RINCRQ,
		RDECRQ:
		return true
	}
	return false
}
