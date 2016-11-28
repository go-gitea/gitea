package hbase

import (
	"bufio"
	"bytes"
	"io"
	"net"
	"strings"
	"sync"

	pb "github.com/golang/protobuf/proto"
	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/go-hbase/iohelper"
	"github.com/pingcap/go-hbase/proto"
)

type ServiceType byte

const (
	MasterMonitorService = iota + 1
	MasterService
	MasterAdminService
	AdminService
	ClientService
	RegionServerStatusService
)

// convert above const to protobuf string
var ServiceString = map[ServiceType]string{
	MasterMonitorService:      "MasterMonitorService",
	MasterService:             "MasterService",
	MasterAdminService:        "MasterAdminService",
	AdminService:              "AdminService",
	ClientService:             "ClientService",
	RegionServerStatusService: "RegionServerStatusService",
}

type idGenerator struct {
	n  int
	mu *sync.RWMutex
}

func newIdGenerator() *idGenerator {
	return &idGenerator{
		n:  0,
		mu: &sync.RWMutex{},
	}
}

func (a *idGenerator) get() int {
	a.mu.RLock()
	v := a.n
	a.mu.RUnlock()
	return v
}

func (a *idGenerator) incrAndGet() int {
	a.mu.Lock()
	a.n++
	v := a.n
	a.mu.Unlock()
	return v
}

type connection struct {
	mu           sync.Mutex
	addr         string
	conn         net.Conn
	bw           *bufio.Writer
	idGen        *idGenerator
	serviceType  ServiceType
	in           chan *iohelper.PbBuffer
	ongoingCalls map[int]*call
}

func processMessage(msg []byte) ([][]byte, error) {
	buf := pb.NewBuffer(msg)
	payloads := make([][]byte, 0)

	// Question: why can we ignore this error?
	for {
		hbytes, err := buf.DecodeRawBytes(true)
		if err != nil {
			// Check whether error is `unexpected EOF`.
			if strings.Contains(err.Error(), "unexpected EOF") {
				break
			}

			log.Errorf("Decode raw bytes error - %v", errors.ErrorStack(err))
			return nil, errors.Trace(err)
		}

		payloads = append(payloads, hbytes)
	}

	return payloads, nil
}

func readPayloads(r io.Reader) ([][]byte, error) {
	nBytesExpecting, err := iohelper.ReadInt32(r)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if nBytesExpecting > 0 {
		buf, err := iohelper.ReadN(r, nBytesExpecting)
		// Question: why should we return error only when we get an io.EOF error?
		if err != nil && ErrorEqual(err, io.EOF) {
			return nil, errors.Trace(err)
		}

		payloads, err := processMessage(buf)
		if err != nil {
			return nil, errors.Trace(err)
		}

		if len(payloads) > 0 {
			return payloads, nil
		}
	}
	return nil, errors.New("unexpected payload")
}

func newConnection(addr string, srvType ServiceType) (*connection, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, errors.Trace(err)
	}
	if _, ok := ServiceString[srvType]; !ok {
		return nil, errors.Errorf("unexpected service type [serviceType=%d]", srvType)
	}
	c := &connection{
		addr:         addr,
		bw:           bufio.NewWriter(conn),
		conn:         conn,
		in:           make(chan *iohelper.PbBuffer, 20),
		serviceType:  srvType,
		idGen:        newIdGenerator(),
		ongoingCalls: map[int]*call{},
	}

	err = c.init()
	if err != nil {
		return nil, errors.Trace(err)
	}

	return c, nil
}

func (c *connection) init() error {
	err := c.writeHead()
	if err != nil {
		return errors.Trace(err)
	}

	err = c.writeConnectionHeader()
	if err != nil {
		return errors.Trace(err)
	}

	go func() {
		err := c.processMessages()
		if err != nil {
			log.Warnf("process messages failed - %v", errors.ErrorStack(err))
			return
		}
	}()
	go c.dispatch()
	return nil
}

func (c *connection) processMessages() error {
	for {
		msgs, err := readPayloads(c.conn)
		if err != nil {
			return errors.Trace(err)
		}

		var rh proto.ResponseHeader
		err = pb.Unmarshal(msgs[0], &rh)
		if err != nil {
			return errors.Trace(err)
		}

		callId := rh.GetCallId()
		c.mu.Lock()
		call, ok := c.ongoingCalls[int(callId)]
		if !ok {
			c.mu.Unlock()
			return errors.Errorf("Invalid call id: %d", callId)
		}
		delete(c.ongoingCalls, int(callId))
		c.mu.Unlock()

		exception := rh.GetException()
		if exception != nil {
			call.complete(errors.Errorf("Exception returned: %s\n%s", exception.GetExceptionClassName(), exception.GetStackTrace()), nil)
		} else if len(msgs) == 2 {
			call.complete(nil, msgs[1])
		}
	}
}

func (c *connection) writeHead() error {
	buf := bytes.NewBuffer(nil)
	buf.Write(hbaseHeaderBytes)
	buf.WriteByte(0)
	buf.WriteByte(80)
	_, err := c.conn.Write(buf.Bytes())
	return errors.Trace(err)
}

func (c *connection) writeConnectionHeader() error {
	buf := iohelper.NewPbBuffer()
	service := pb.String(ServiceString[c.serviceType])

	err := buf.WritePBMessage(&proto.ConnectionHeader{
		UserInfo: &proto.UserInformation{
			EffectiveUser: pb.String("pingcap"),
		},
		ServiceName: service,
	})
	if err != nil {
		return errors.Trace(err)
	}

	err = buf.PrependSize()
	if err != nil {
		return errors.Trace(err)
	}

	_, err = c.conn.Write(buf.Bytes())
	if err != nil {
		return errors.Trace(err)
	}

	return nil
}

func (c *connection) dispatch() {
	for {
		select {
		case buf := <-c.in:
			// TODO: add error check.
			c.bw.Write(buf.Bytes())
			if len(c.in) == 0 {
				c.bw.Flush()
			}
		}
	}
}

func (c *connection) call(request *call) error {
	id := c.idGen.incrAndGet()
	rh := &proto.RequestHeader{
		CallId:       pb.Uint32(uint32(id)),
		MethodName:   pb.String(request.methodName),
		RequestParam: pb.Bool(true),
	}

	request.id = uint32(id)

	bfrh := iohelper.NewPbBuffer()
	err := bfrh.WritePBMessage(rh)
	if err != nil {
		return errors.Trace(err)
	}

	bfr := iohelper.NewPbBuffer()
	err = bfr.WritePBMessage(request.request)
	if err != nil {
		return errors.Trace(err)
	}

	// Buf =>
	// | total size | pb1 size | pb1 | pb2 size | pb2 | ...
	buf := iohelper.NewPbBuffer()
	buf.WriteDelimitedBuffers(bfrh, bfr)

	c.mu.Lock()
	c.ongoingCalls[id] = request
	c.in <- buf
	c.mu.Unlock()

	return nil
}

func (c *connection) close() error {
	return c.conn.Close()
}
