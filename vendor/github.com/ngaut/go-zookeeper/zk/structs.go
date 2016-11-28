package zk

import (
	"encoding/binary"
	"errors"
	"reflect"
	"runtime"
	"time"
)

var (
	ErrUnhandledFieldType = errors.New("zk: unhandled field type")
	ErrPtrExpected        = errors.New("zk: encode/decode expect a non-nil pointer to struct")
	ErrShortBuffer        = errors.New("zk: buffer too small")
)

type ACL struct {
	Perms  int32
	Scheme string
	ID     string
}

type zkstat struct {
	ZCzxid          int64 // The zxid of the change that caused this znode to be created.
	ZMzxid          int64 // The zxid of the change that last modified this znode.
	ZCtime          int64 // The time in milliseconds from epoch when this znode was created.
	ZMtime          int64 // The time in milliseconds from epoch when this znode was last modified.
	ZVersion        int32 // The number of changes to the data of this znode.
	ZCversion       int32 // The number of changes to the children of this znode.
	ZAversion       int32 // The number of changes to the ACL of this znode.
	ZEphemeralOwner int64 // The session id of the owner of this znode if the znode is an ephemeral node. If it is not an ephemeral node, it will be zero.
	ZDataLength     int32 // The length of the data field of this znode.
	ZNumChildren    int32 // The number of children of this znode.
	ZPzxid          int64 // last modified children
}

type Stat interface {
	Czxid() int64
	Mzxid() int64
	CTime() time.Time
	MTime() time.Time
	Version() int
	CVersion() int
	AVersion() int
	EphemeralOwner() int64
	DataLength() int
	NumChildren() int
	Pzxid() int64
}

// Czxid returns the zxid of the change that caused the node to be created.
func (s *zkstat) Czxid() int64 {
	return s.ZCzxid
}

// Mzxid returns the zxid of the change that last modified the node.
func (s *zkstat) Mzxid() int64 {
	return s.ZMzxid
}

func millisec2time(ms int64) time.Time {
	return time.Unix(ms/1e3, ms%1e3*1e6)
}

// CTime returns the time (at millisecond resolution)  when the node was
// created.
func (s *zkstat) CTime() time.Time {
	return millisec2time(s.ZCtime)
}

// MTime returns the time (at millisecond resolution) when the node was
// last modified.
func (s *zkstat) MTime() time.Time {
	return millisec2time(int64(s.ZMtime))
}

// Version returns the number of changes to the data of the node.
func (s *zkstat) Version() int {
	return int(s.ZVersion)
}

// CVersion returns the number of changes to the children of the node.
// This only changes when children are created or removed.
func (s *zkstat) CVersion() int {
	return int(s.ZCversion)
}

// AVersion returns the number of changes to the ACL of the node.
func (s *zkstat) AVersion() int {
	return int(s.ZAversion)
}

// If the node is an ephemeral node, EphemeralOwner returns the session id
// of the owner of the node; otherwise it will return zero.
func (s *zkstat) EphemeralOwner() int64 {
	return int64(s.ZEphemeralOwner)
}

// DataLength returns the length of the data in the node in bytes.
func (s *zkstat) DataLength() int {
	return int(s.ZDataLength)
}

// NumChildren returns the number of children of the node.
func (s *zkstat) NumChildren() int {
	return int(s.ZNumChildren)
}

// Pzxid returns the Pzxid of the node, whatever that is.
func (s *zkstat) Pzxid() int64 {
	return int64(s.ZPzxid)
}

type requestHeader struct {
	Xid    int32
	Opcode int32
}

type responseHeader struct {
	Xid  int32
	Zxid int64
	Err  ErrCode
}

type multiHeader struct {
	Type int32
	Done bool
	Err  ErrCode
}

type auth struct {
	Type   int32
	Scheme string
	Auth   []byte
}

// Generic request structs

type pathRequest struct {
	Path string
}

type PathVersionRequest struct {
	Path    string
	Version int32
}

type pathWatchRequest struct {
	Path  string
	Watch bool
}

type pathResponse struct {
	Path string
}

type statResponse struct {
	Stat zkstat
}

//

type CheckVersionRequest PathVersionRequest
type closeRequest struct{}
type closeResponse struct{}

type connectRequest struct {
	ProtocolVersion int32
	LastZxidSeen    int64
	TimeOut         int32
	SessionID       int64
	Passwd          []byte
}

type connectResponse struct {
	ProtocolVersion int32
	TimeOut         int32
	SessionID       int64
	Passwd          []byte
}

type CreateRequest struct {
	Path  string
	Data  []byte
	Acl   []ACL
	Flags int32
}

type createResponse pathResponse
type DeleteRequest PathVersionRequest
type deleteResponse struct{}

type errorResponse struct {
	Err int32
}

type existsRequest pathWatchRequest
type existsResponse statResponse
type getAclRequest pathRequest

type getAclResponse struct {
	Acl  []ACL
	Stat zkstat
}

type getChildrenRequest pathRequest

type getChildrenResponse struct {
	Children []string
}

type getChildren2Request pathWatchRequest

type getChildren2Response struct {
	Children []string
	Stat     zkstat
}

type getDataRequest pathWatchRequest

type getDataResponse struct {
	Data []byte
	Stat zkstat
}

type getMaxChildrenRequest pathRequest

type getMaxChildrenResponse struct {
	Max int32
}

type getSaslRequest struct {
	Token []byte
}

type pingRequest struct{}
type pingResponse struct{}

type setAclRequest struct {
	Path    string
	Acl     []ACL
	Version int32
}

type setAclResponse statResponse

type SetDataRequest struct {
	Path    string
	Data    []byte
	Version int32
}

type setDataResponse statResponse

type setMaxChildren struct {
	Path string
	Max  int32
}

type setSaslRequest struct {
	Token string
}

type setSaslResponse struct {
	Token string
}

type setWatchesRequest struct {
	RelativeZxid int64
	DataWatches  []string
	ExistWatches []string
	ChildWatches []string
}

type setWatchesResponse struct{}

type syncRequest pathRequest
type syncResponse pathResponse

type setAuthRequest auth
type setAuthResponse struct{}

type multiRequestOp struct {
	Header multiHeader
	Op     interface{}
}
type multiRequest struct {
	Ops        []multiRequestOp
	DoneHeader multiHeader
}
type multiResponseOp struct {
	Header multiHeader
	String string
	Stat   *zkstat
}
type multiResponse struct {
	Ops        []multiResponseOp
	DoneHeader multiHeader
}

func (r *multiRequest) Encode(buf []byte) (int, error) {
	total := 0
	for _, op := range r.Ops {
		op.Header.Done = false
		n, err := encodePacketValue(buf[total:], reflect.ValueOf(op))
		if err != nil {
			return total, err
		}
		total += n
	}
	r.DoneHeader.Done = true
	n, err := encodePacketValue(buf[total:], reflect.ValueOf(r.DoneHeader))
	if err != nil {
		return total, err
	}
	total += n

	return total, nil
}

func (r *multiRequest) Decode(buf []byte) (int, error) {
	r.Ops = make([]multiRequestOp, 0)
	r.DoneHeader = multiHeader{-1, true, -1}
	total := 0
	for {
		header := &multiHeader{}
		n, err := decodePacketValue(buf[total:], reflect.ValueOf(header))
		if err != nil {
			return total, err
		}
		total += n
		if header.Done {
			r.DoneHeader = *header
			break
		}

		req := requestStructForOp(header.Type)
		if req == nil {
			return total, ErrAPIError
		}
		n, err = decodePacketValue(buf[total:], reflect.ValueOf(req))
		if err != nil {
			return total, err
		}
		total += n
		r.Ops = append(r.Ops, multiRequestOp{*header, req})
	}
	return total, nil
}

func (r *multiResponse) Decode(buf []byte) (int, error) {
	r.Ops = make([]multiResponseOp, 0)
	r.DoneHeader = multiHeader{-1, true, -1}
	total := 0
	for {
		header := &multiHeader{}
		n, err := decodePacketValue(buf[total:], reflect.ValueOf(header))
		if err != nil {
			return total, err
		}
		total += n
		if header.Done {
			r.DoneHeader = *header
			break
		}

		res := multiResponseOp{Header: *header}
		var w reflect.Value
		switch header.Type {
		default:
			return total, ErrAPIError
		case opCreate:
			w = reflect.ValueOf(&res.String)
		case opSetData:
			res.Stat = new(zkstat)
			w = reflect.ValueOf(res.Stat)
		case opCheck, opDelete:
		}
		if w.IsValid() {
			n, err := decodePacketValue(buf[total:], w)
			if err != nil {
				return total, err
			}
			total += n
		}
		r.Ops = append(r.Ops, res)
	}
	return total, nil
}

type watcherEvent struct {
	Type  EventType
	State State
	Path  string
}

type decoder interface {
	Decode(buf []byte) (int, error)
}

type encoder interface {
	Encode(buf []byte) (int, error)
}

func decodePacket(buf []byte, st interface{}) (n int, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(runtime.Error); ok && e.Error() == "runtime error: slice bounds out of range" {
				err = ErrShortBuffer
			} else {
				panic(r)
			}
		}
	}()

	v := reflect.ValueOf(st)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return 0, ErrPtrExpected
	}
	return decodePacketValue(buf, v)
}

func decodePacketValue(buf []byte, v reflect.Value) (int, error) {
	rv := v
	kind := v.Kind()
	if kind == reflect.Ptr {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
		kind = v.Kind()
	}

	n := 0
	switch kind {
	default:
		return n, ErrUnhandledFieldType
	case reflect.Struct:
		if de, ok := rv.Interface().(decoder); ok {
			return de.Decode(buf)
		} else if de, ok := v.Interface().(decoder); ok {
			return de.Decode(buf)
		} else {
			for i := 0; i < v.NumField(); i++ {
				field := v.Field(i)
				n2, err := decodePacketValue(buf[n:], field)
				n += n2
				if err != nil {
					return n, err
				}
			}
		}
	case reflect.Bool:
		v.SetBool(buf[n] != 0)
		n++
	case reflect.Int32:
		v.SetInt(int64(binary.BigEndian.Uint32(buf[n : n+4])))
		n += 4
	case reflect.Int64:
		v.SetInt(int64(binary.BigEndian.Uint64(buf[n : n+8])))
		n += 8
	case reflect.String:
		ln := int(binary.BigEndian.Uint32(buf[n : n+4]))
		v.SetString(string(buf[n+4 : n+4+ln]))
		n += 4 + ln
	case reflect.Slice:
		switch v.Type().Elem().Kind() {
		default:
			count := int(binary.BigEndian.Uint32(buf[n : n+4]))
			n += 4
			values := reflect.MakeSlice(v.Type(), count, count)
			v.Set(values)
			for i := 0; i < count; i++ {
				n2, err := decodePacketValue(buf[n:], values.Index(i))
				n += n2
				if err != nil {
					return n, err
				}
			}
		case reflect.Uint8:
			ln := int(int32(binary.BigEndian.Uint32(buf[n : n+4])))
			if ln < 0 {
				n += 4
				v.SetBytes(nil)
			} else {
				bytes := make([]byte, ln)
				copy(bytes, buf[n+4:n+4+ln])
				v.SetBytes(bytes)
				n += 4 + ln
			}
		}
	}
	return n, nil
}

func encodePacket(buf []byte, st interface{}) (n int, err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(runtime.Error); ok && e.Error() == "runtime error: slice bounds out of range" {
				err = ErrShortBuffer
			} else {
				panic(r)
			}
		}
	}()

	v := reflect.ValueOf(st)
	if v.Kind() != reflect.Ptr || v.IsNil() {
		return 0, ErrPtrExpected
	}
	return encodePacketValue(buf, v)
}

func encodePacketValue(buf []byte, v reflect.Value) (int, error) {
	rv := v
	for v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	n := 0
	switch v.Kind() {
	default:
		return n, ErrUnhandledFieldType
	case reflect.Struct:
		if en, ok := rv.Interface().(encoder); ok {
			return en.Encode(buf)
		} else if en, ok := v.Interface().(encoder); ok {
			return en.Encode(buf)
		} else {
			for i := 0; i < v.NumField(); i++ {
				field := v.Field(i)
				n2, err := encodePacketValue(buf[n:], field)
				n += n2
				if err != nil {
					return n, err
				}
			}
		}
	case reflect.Bool:
		if v.Bool() {
			buf[n] = 1
		} else {
			buf[n] = 0
		}
		n++
	case reflect.Int32:
		binary.BigEndian.PutUint32(buf[n:n+4], uint32(v.Int()))
		n += 4
	case reflect.Int64:
		binary.BigEndian.PutUint64(buf[n:n+8], uint64(v.Int()))
		n += 8
	case reflect.String:
		str := v.String()
		binary.BigEndian.PutUint32(buf[n:n+4], uint32(len(str)))
		copy(buf[n+4:n+4+len(str)], []byte(str))
		n += 4 + len(str)
	case reflect.Slice:
		switch v.Type().Elem().Kind() {
		default:
			count := v.Len()
			startN := n
			n += 4
			for i := 0; i < count; i++ {
				n2, err := encodePacketValue(buf[n:], v.Index(i))
				n += n2
				if err != nil {
					return n, err
				}
			}
			binary.BigEndian.PutUint32(buf[startN:startN+4], uint32(count))
		case reflect.Uint8:
			if v.IsNil() {
				binary.BigEndian.PutUint32(buf[n:n+4], uint32(0xffffffff))
				n += 4
			} else {
				bytes := v.Bytes()
				binary.BigEndian.PutUint32(buf[n:n+4], uint32(len(bytes)))
				copy(buf[n+4:n+4+len(bytes)], bytes)
				n += 4 + len(bytes)
			}
		}
	}
	return n, nil
}

func requestStructForOp(op int32) interface{} {
	switch op {
	case opClose:
		return &closeRequest{}
	case opCreate:
		return &CreateRequest{}
	case opDelete:
		return &DeleteRequest{}
	case opExists:
		return &existsRequest{}
	case opGetAcl:
		return &getAclRequest{}
	case opGetChildren:
		return &getChildrenRequest{}
	case opGetChildren2:
		return &getChildren2Request{}
	case opGetData:
		return &getDataRequest{}
	case opPing:
		return &pingRequest{}
	case opSetAcl:
		return &setAclRequest{}
	case opSetData:
		return &SetDataRequest{}
	case opSetWatches:
		return &setWatchesRequest{}
	case opSync:
		return &syncRequest{}
	case opSetAuth:
		return &setAuthRequest{}
	case opCheck:
		return &CheckVersionRequest{}
	case opMulti:
		return &multiRequest{}
	}
	return nil
}

func responseStructForOp(op int32) interface{} {
	switch op {
	case opClose:
		return &closeResponse{}
	case opCreate:
		return &createResponse{}
	case opDelete:
		return &deleteResponse{}
	case opExists:
		return &existsResponse{}
	case opGetAcl:
		return &getAclResponse{}
	case opGetChildren:
		return &getChildrenResponse{}
	case opGetChildren2:
		return &getChildren2Response{}
	case opGetData:
		return &getDataResponse{}
	case opPing:
		return &pingResponse{}
	case opSetAcl:
		return &setAclResponse{}
	case opSetData:
		return &setDataResponse{}
	case opSetWatches:
		return &setWatchesResponse{}
	case opSync:
		return &syncResponse{}
	case opWatcherEvent:
		return &watcherEvent{}
	case opSetAuth:
		return &setAuthResponse{}
	// case opCheck:
	// 	return &checkVersionResponse{}
	case opMulti:
		return &multiResponse{}
	}
	return nil
}
