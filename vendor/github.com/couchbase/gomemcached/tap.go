package gomemcached

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
)

type TapConnectFlag uint32

// Tap connect option flags
const (
	BACKFILL           = TapConnectFlag(0x01)
	DUMP               = TapConnectFlag(0x02)
	LIST_VBUCKETS      = TapConnectFlag(0x04)
	TAKEOVER_VBUCKETS  = TapConnectFlag(0x08)
	SUPPORT_ACK        = TapConnectFlag(0x10)
	REQUEST_KEYS_ONLY  = TapConnectFlag(0x20)
	CHECKPOINT         = TapConnectFlag(0x40)
	REGISTERED_CLIENT  = TapConnectFlag(0x80)
	FIX_FLAG_BYTEORDER = TapConnectFlag(0x100)
)

// Tap opaque event subtypes
const (
	TAP_OPAQUE_ENABLE_AUTO_NACK       = 0
	TAP_OPAQUE_INITIAL_VBUCKET_STREAM = 1
	TAP_OPAQUE_ENABLE_CHECKPOINT_SYNC = 2
	TAP_OPAQUE_CLOSE_TAP_STREAM       = 7
	TAP_OPAQUE_CLOSE_BACKFILL         = 8
)

// Tap item flags
const (
	TAP_ACK                     = 1
	TAP_NO_VALUE                = 2
	TAP_FLAG_NETWORK_BYTE_ORDER = 4
)

// TapConnectFlagNames for TapConnectFlag
var TapConnectFlagNames = map[TapConnectFlag]string{
	BACKFILL:           "BACKFILL",
	DUMP:               "DUMP",
	LIST_VBUCKETS:      "LIST_VBUCKETS",
	TAKEOVER_VBUCKETS:  "TAKEOVER_VBUCKETS",
	SUPPORT_ACK:        "SUPPORT_ACK",
	REQUEST_KEYS_ONLY:  "REQUEST_KEYS_ONLY",
	CHECKPOINT:         "CHECKPOINT",
	REGISTERED_CLIENT:  "REGISTERED_CLIENT",
	FIX_FLAG_BYTEORDER: "FIX_FLAG_BYTEORDER",
}

// TapItemParser is a function to parse a single tap extra.
type TapItemParser func(io.Reader) (interface{}, error)

// TapParseUint64 is a function to parse a single tap uint64.
func TapParseUint64(r io.Reader) (interface{}, error) {
	var rv uint64
	err := binary.Read(r, binary.BigEndian, &rv)
	return rv, err
}

// TapParseUint16 is a function to parse a single tap uint16.
func TapParseUint16(r io.Reader) (interface{}, error) {
	var rv uint16
	err := binary.Read(r, binary.BigEndian, &rv)
	return rv, err
}

// TapParseBool is a function to parse a single tap boolean.
func TapParseBool(r io.Reader) (interface{}, error) {
	return true, nil
}

// TapParseVBList parses a list of vBucket numbers as []uint16.
func TapParseVBList(r io.Reader) (interface{}, error) {
	num, err := TapParseUint16(r)
	if err != nil {
		return nil, err
	}
	n := int(num.(uint16))

	rv := make([]uint16, n)
	for i := 0; i < n; i++ {
		x, err := TapParseUint16(r)
		if err != nil {
			return nil, err
		}
		rv[i] = x.(uint16)
	}

	return rv, err
}

// TapFlagParsers parser functions for TAP fields.
var TapFlagParsers = map[TapConnectFlag]TapItemParser{
	BACKFILL:      TapParseUint64,
	LIST_VBUCKETS: TapParseVBList,
}

// SplitFlags will split the ORed flags into the individual bit flags.
func (f TapConnectFlag) SplitFlags() []TapConnectFlag {
	rv := []TapConnectFlag{}
	for i := uint32(1); f != 0; i = i << 1 {
		if uint32(f)&i == i {
			rv = append(rv, TapConnectFlag(i))
		}
		f = TapConnectFlag(uint32(f) & (^i))
	}
	return rv
}

func (f TapConnectFlag) String() string {
	parts := []string{}
	for _, x := range f.SplitFlags() {
		p := TapConnectFlagNames[x]
		if p == "" {
			p = fmt.Sprintf("0x%x", int(x))
		}
		parts = append(parts, p)
	}
	return strings.Join(parts, "|")
}

type TapConnect struct {
	Flags         map[TapConnectFlag]interface{}
	RemainingBody []byte
	Name          string
}

// ParseTapCommands parse the tap request into the interesting bits we may
// need to do something with.
func (req *MCRequest) ParseTapCommands() (TapConnect, error) {
	rv := TapConnect{
		Flags: map[TapConnectFlag]interface{}{},
		Name:  string(req.Key),
	}

	if len(req.Extras) < 4 {
		return rv, fmt.Errorf("not enough extra bytes: %x", req.Extras)
	}

	flags := TapConnectFlag(binary.BigEndian.Uint32(req.Extras))

	r := bytes.NewReader(req.Body)

	for _, f := range flags.SplitFlags() {
		fun := TapFlagParsers[f]
		if fun == nil {
			fun = TapParseBool
		}

		val, err := fun(r)
		if err != nil {
			return rv, err
		}

		rv.Flags[f] = val
	}

	var err error
	rv.RemainingBody, err = ioutil.ReadAll(r)

	return rv, err
}
