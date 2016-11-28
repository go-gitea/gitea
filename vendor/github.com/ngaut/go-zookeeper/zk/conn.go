package zk

/*
TODO:
* make sure a ping response comes back in a reasonable time

Possible watcher events:
* Event{Type: EventNotWatching, State: StateDisconnected, Path: path, Err: err}
*/

import (
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var ErrNoServer = errors.New("zk: could not connect to a server")

const (
	bufferSize      = 10 * 1024 * 1024
	eventChanSize   = 6
	sendChanSize    = 16
	protectedPrefix = "_c_"
)

type watchType int

const (
	watchTypeData  = iota
	watchTypeExist = iota
	watchTypeChild = iota
)

type watchPathType struct {
	path  string
	wType watchType
}

type Dialer func(network, address string, timeout time.Duration) (net.Conn, error)

type Conn struct {
	lastZxid  int64
	sessionID int64
	state     State // must be 32-bit aligned
	xid       int32
	timeout   int32 // session timeout in seconds
	passwd    []byte

	dialer         Dialer
	servers        []string
	serverIndex    int
	conn           net.Conn
	eventChan      chan Event
	shouldQuit     chan bool
	pingInterval   time.Duration
	recvTimeout    time.Duration
	connectTimeout time.Duration

	sendChan     chan *request
	requests     map[int32]*request // Xid -> pending request
	requestsLock sync.Mutex
	watchers     map[watchPathType][]chan Event
	watchersLock sync.Mutex

	// Debug (used by unit tests)
	reconnectDelay time.Duration
}

type request struct {
	xid        int32
	opcode     int32
	pkt        interface{}
	recvStruct interface{}
	recvChan   chan response

	// Because sending and receiving happen in separate go routines, there's
	// a possible race condition when creating watches from outside the read
	// loop. We must ensure that a watcher gets added to the list synchronously
	// with the response from the server on any request that creates a watch.
	// In order to not hard code the watch logic for each opcode in the recv
	// loop the caller can use recvFunc to insert some synchronously code
	// after a response.
	recvFunc func(*request, *responseHeader, error)
}

type response struct {
	zxid int64
	err  error
}

type Event struct {
	Type  EventType
	State State
	Path  string // For non-session events, the path of the watched node.
	Err   error
}

func Connect(servers []string, recvTimeout time.Duration) (*Conn, <-chan Event, error) {
	return ConnectWithDialer(servers, recvTimeout, nil)
}

func ConnectWithDialer(servers []string, recvTimeout time.Duration, dialer Dialer) (*Conn, <-chan Event, error) {
	// Randomize the order of the servers to avoid creating hotspots
	stringShuffle(servers)

	for i, addr := range servers {
		if !strings.Contains(addr, ":") {
			servers[i] = addr + ":" + strconv.Itoa(DefaultPort)
		}
	}
	ec := make(chan Event, eventChanSize)
	if dialer == nil {
		dialer = net.DialTimeout
	}
	conn := Conn{
		dialer:         dialer,
		servers:        servers,
		serverIndex:    0,
		conn:           nil,
		state:          StateDisconnected,
		eventChan:      ec,
		shouldQuit:     make(chan bool),
		recvTimeout:    recvTimeout,
		pingInterval:   time.Duration((int64(recvTimeout) / 2)),
		connectTimeout: 1 * time.Second,
		sendChan:       make(chan *request, sendChanSize),
		requests:       make(map[int32]*request),
		watchers:       make(map[watchPathType][]chan Event),
		passwd:         emptyPassword,
		timeout:        30000,

		// Debug
		reconnectDelay: time.Second,
	}
	go func() {
		conn.loop()
		conn.flushRequests(ErrClosing)
		conn.invalidateWatches(ErrClosing)
		close(conn.eventChan)
	}()
	return &conn, ec, nil
}

func (c *Conn) Close() {
	close(c.shouldQuit)

	select {
	case <-c.queueRequest(opClose, &closeRequest{}, &closeResponse{}, nil):
	case <-time.After(time.Second):
	}
}

func (c *Conn) State() State {
	return State(atomic.LoadInt32((*int32)(&c.state)))
}

func (c *Conn) setState(state State) {
	atomic.StoreInt32((*int32)(&c.state), int32(state))
	select {
	case c.eventChan <- Event{Type: EventSession, State: state}:
	default:
		// panic("zk: event channel full - it must be monitored and never allowed to be full")
	}
}

func (c *Conn) connect() {
	c.serverIndex = (c.serverIndex + 1) % len(c.servers)
	startIndex := c.serverIndex
	c.setState(StateConnecting)
	for {
		zkConn, err := c.dialer("tcp", c.servers[c.serverIndex], c.connectTimeout)
		if err == nil {
			c.conn = zkConn
			c.setState(StateConnected)
			return
		}

		log.Printf("Failed to connect to %s: %+v", c.servers[c.serverIndex], err)

		c.serverIndex = (c.serverIndex + 1) % len(c.servers)
		if c.serverIndex == startIndex {
			c.flushUnsentRequests(ErrNoServer)
			time.Sleep(time.Second)
		}
	}
}

func (c *Conn) loop() {
	for {
		c.connect()
		err := c.authenticate()
		switch {
		case err == ErrSessionExpired:
			c.invalidateWatches(err)
		case err != nil && c.conn != nil:
			c.conn.Close()
		case err == nil:
			closeChan := make(chan bool) // channel to tell send loop stop
			var wg sync.WaitGroup

			wg.Add(1)
			go func() {
				c.sendLoop(c.conn, closeChan)
				c.conn.Close() // causes recv loop to EOF/exit
				wg.Done()
			}()

			wg.Add(1)
			go func() {
				err = c.recvLoop(c.conn)
				if err == nil {
					panic("zk: recvLoop should never return nil error")
				}
				close(closeChan) // tell send loop to exit
				wg.Done()
			}()

			wg.Wait()
		}

		c.setState(StateDisconnected)

		// Yeesh
		if err != io.EOF && err != ErrSessionExpired && !strings.Contains(err.Error(), "use of closed network connection") {
			log.Println(err)
		}

		select {
		case <-c.shouldQuit:
			c.flushRequests(ErrClosing)
			return
		default:
		}

		if err != ErrSessionExpired {
			err = ErrConnectionClosed
		}
		c.flushRequests(err)

		if c.reconnectDelay > 0 {
			select {
			case <-c.shouldQuit:
				return
			case <-time.After(c.reconnectDelay):
			}
		}
	}
}

func (c *Conn) flushUnsentRequests(err error) {
	for {
		select {
		default:
			return
		case req := <-c.sendChan:
			req.recvChan <- response{-1, err}
		}
	}
}

// Send error to all pending requests and clear request map
func (c *Conn) flushRequests(err error) {
	c.requestsLock.Lock()
	for _, req := range c.requests {
		req.recvChan <- response{-1, err}
	}
	c.requests = make(map[int32]*request)
	c.requestsLock.Unlock()
}

// Send error to all watchers and clear watchers map
func (c *Conn) invalidateWatches(err error) {
	c.watchersLock.Lock()
	defer c.watchersLock.Unlock()

	if len(c.watchers) >= 0 {
		for pathType, watchers := range c.watchers {
			ev := Event{Type: EventNotWatching, State: StateDisconnected, Path: pathType.path, Err: err}
			for _, ch := range watchers {
				ch <- ev
				close(ch)
			}
		}
		c.watchers = make(map[watchPathType][]chan Event)
	}
}

func (c *Conn) sendSetWatches() {
	c.watchersLock.Lock()
	defer c.watchersLock.Unlock()

	if len(c.watchers) == 0 {
		return
	}

	req := &setWatchesRequest{
		RelativeZxid: c.lastZxid,
		DataWatches:  make([]string, 0),
		ExistWatches: make([]string, 0),
		ChildWatches: make([]string, 0),
	}
	n := 0
	for pathType, watchers := range c.watchers {
		if len(watchers) == 0 {
			continue
		}
		switch pathType.wType {
		case watchTypeData:
			req.DataWatches = append(req.DataWatches, pathType.path)
		case watchTypeExist:
			req.ExistWatches = append(req.ExistWatches, pathType.path)
		case watchTypeChild:
			req.ChildWatches = append(req.ChildWatches, pathType.path)
		}
		n++
	}
	if n == 0 {
		return
	}

	go func() {
		res := &setWatchesResponse{}
		_, err := c.request(opSetWatches, req, res, nil)
		if err != nil {
			log.Printf("Failed to set previous watches: %s", err.Error())
		}
	}()
}

func (c *Conn) authenticate() error {
	buf := make([]byte, 256)

	// connect request

	n, err := encodePacket(buf[4:], &connectRequest{
		ProtocolVersion: protocolVersion,
		LastZxidSeen:    c.lastZxid,
		TimeOut:         c.timeout,
		SessionID:       c.sessionID,
		Passwd:          c.passwd,
	})
	if err != nil {
		return err
	}

	binary.BigEndian.PutUint32(buf[:4], uint32(n))

	_, err = c.conn.Write(buf[:n+4])
	if err != nil {
		return err
	}

	c.sendSetWatches()

	// connect response

	// package length
	_, err = io.ReadFull(c.conn, buf[:4])
	if err != nil {
		return err
	}

	blen := int(binary.BigEndian.Uint32(buf[:4]))
	if cap(buf) < blen {
		buf = make([]byte, blen)
	}

	_, err = io.ReadFull(c.conn, buf[:blen])
	if err != nil {
		return err
	}

	r := connectResponse{}
	_, err = decodePacket(buf[:blen], &r)
	if err != nil {
		return err
	}
	if r.SessionID == 0 {
		c.sessionID = 0
		c.passwd = emptyPassword
		c.lastZxid = 0
		c.setState(StateExpired)
		return ErrSessionExpired
	}

	if c.sessionID != r.SessionID {
		atomic.StoreInt32(&c.xid, 0)
	}
	c.timeout = r.TimeOut
	c.sessionID = r.SessionID
	c.passwd = r.Passwd
	c.setState(StateHasSession)

	return nil
}

func (c *Conn) sendLoop(conn net.Conn, closeChan <-chan bool) error {
	pingTicker := time.NewTicker(c.pingInterval)
	defer pingTicker.Stop()

	buf := make([]byte, bufferSize)
	for {
		select {
		case req := <-c.sendChan:
			header := &requestHeader{req.xid, req.opcode}
			n, err := encodePacket(buf[4:], header)
			if err != nil {
				req.recvChan <- response{-1, err}
				continue
			}

			n2, err := encodePacket(buf[4+n:], req.pkt)
			if err != nil {
				req.recvChan <- response{-1, err}
				continue
			}

			n += n2

			binary.BigEndian.PutUint32(buf[:4], uint32(n))

			c.requestsLock.Lock()
			select {
			case <-closeChan:
				req.recvChan <- response{-1, ErrConnectionClosed}
				c.requestsLock.Unlock()
				return ErrConnectionClosed
			default:
			}
			c.requests[req.xid] = req
			c.requestsLock.Unlock()

			conn.SetWriteDeadline(time.Now().Add(c.recvTimeout))
			_, err = conn.Write(buf[:n+4])
			conn.SetWriteDeadline(time.Time{})
			if err != nil {
				req.recvChan <- response{-1, err}
				conn.Close()
				return err
			}
		case <-pingTicker.C:
			n, err := encodePacket(buf[4:], &requestHeader{Xid: -2, Opcode: opPing})
			if err != nil {
				panic("zk: opPing should never fail to serialize")
			}

			binary.BigEndian.PutUint32(buf[:4], uint32(n))

			conn.SetWriteDeadline(time.Now().Add(c.recvTimeout))
			_, err = conn.Write(buf[:n+4])
			conn.SetWriteDeadline(time.Time{})
			if err != nil {
				conn.Close()
				return err
			}
		case <-closeChan:
			return nil
		}
	}
}

func (c *Conn) recvLoop(conn net.Conn) error {
	buf := make([]byte, bufferSize)
	for {
		// package length
		conn.SetReadDeadline(time.Now().Add(c.recvTimeout))
		_, err := io.ReadFull(conn, buf[:4])
		if err != nil {
			return err
		}

		blen := int(binary.BigEndian.Uint32(buf[:4]))
		if cap(buf) < blen {
			buf = make([]byte, blen)
		}

		_, err = io.ReadFull(conn, buf[:blen])
		conn.SetReadDeadline(time.Time{})
		if err != nil {
			return err
		}

		res := responseHeader{}
		_, err = decodePacket(buf[:16], &res)
		if err != nil {
			return err
		}

		if res.Xid == -1 {
			res := &watcherEvent{}
			_, err := decodePacket(buf[16:16+blen], res)
			if err != nil {
				return err
			}
			ev := Event{
				Type:  res.Type,
				State: res.State,
				Path:  res.Path,
				Err:   nil,
			}
			select {
			case c.eventChan <- ev:
			default:
			}
			wTypes := make([]watchType, 0, 2)
			switch res.Type {
			case EventNodeCreated:
				wTypes = append(wTypes, watchTypeExist)
			case EventNodeDeleted, EventNodeDataChanged:
				wTypes = append(wTypes, watchTypeExist, watchTypeData, watchTypeChild)
			case EventNodeChildrenChanged:
				wTypes = append(wTypes, watchTypeChild)
			}
			c.watchersLock.Lock()
			for _, t := range wTypes {
				wpt := watchPathType{res.Path, t}
				if watchers := c.watchers[wpt]; watchers != nil && len(watchers) > 0 {
					for _, ch := range watchers {
						ch <- ev
						close(ch)
					}
					delete(c.watchers, wpt)
				}
			}
			c.watchersLock.Unlock()
		} else if res.Xid == -2 {
			// Ping response. Ignore.
		} else if res.Xid < 0 {
			log.Printf("Xid < 0 (%d) but not ping or watcher event", res.Xid)
		} else {
			if res.Zxid > 0 {
				c.lastZxid = res.Zxid
			}

			c.requestsLock.Lock()
			req, ok := c.requests[res.Xid]
			if ok {
				delete(c.requests, res.Xid)
			}
			c.requestsLock.Unlock()

			if !ok {
				log.Printf("Response for unknown request with xid %d", res.Xid)
			} else {
				if res.Err != 0 {
					err = res.Err.toError()
				} else {
					_, err = decodePacket(buf[16:16+blen], req.recvStruct)
				}
				if req.recvFunc != nil {
					req.recvFunc(req, &res, err)
				}
				req.recvChan <- response{res.Zxid, err}
				if req.opcode == opClose {
					return io.EOF
				}
			}
		}
	}
}

func (c *Conn) nextXid() int32 {
	return atomic.AddInt32(&c.xid, 1)
}

func (c *Conn) addWatcher(path string, watchType watchType) <-chan Event {
	c.watchersLock.Lock()
	defer c.watchersLock.Unlock()

	ch := make(chan Event, 1)
	wpt := watchPathType{path, watchType}
	c.watchers[wpt] = append(c.watchers[wpt], ch)
	return ch
}

func (c *Conn) queueRequest(opcode int32, req interface{}, res interface{}, recvFunc func(*request, *responseHeader, error)) <-chan response {
	rq := &request{
		xid:        c.nextXid(),
		opcode:     opcode,
		pkt:        req,
		recvStruct: res,
		recvChan:   make(chan response, 1),
		recvFunc:   recvFunc,
	}
	c.sendChan <- rq
	return rq.recvChan
}

func (c *Conn) request(opcode int32, req interface{}, res interface{}, recvFunc func(*request, *responseHeader, error)) (int64, error) {
	r := <-c.queueRequest(opcode, req, res, recvFunc)
	return r.zxid, r.err
}

func (c *Conn) AddAuth(scheme string, auth []byte) error {
	_, err := c.request(opSetAuth, &setAuthRequest{Type: 0, Scheme: scheme, Auth: auth}, &setAuthResponse{}, nil)
	return err
}

func (c *Conn) Children(path string) ([]string, Stat, error) {
	res := &getChildren2Response{}
	_, err := c.request(opGetChildren2, &getChildren2Request{Path: path, Watch: false}, res, nil)
	return res.Children, &res.Stat, err
}

func (c *Conn) ChildrenW(path string) ([]string, Stat, <-chan Event, error) {
	var ech <-chan Event
	res := &getChildren2Response{}
	_, err := c.request(opGetChildren2, &getChildren2Request{Path: path, Watch: true}, res, func(req *request, res *responseHeader, err error) {
		if err == nil {
			ech = c.addWatcher(path, watchTypeChild)
		}
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return res.Children, &res.Stat, ech, err
}

func (c *Conn) Get(path string) ([]byte, Stat, error) {
	res := &getDataResponse{}
	_, err := c.request(opGetData, &getDataRequest{Path: path, Watch: false}, res, nil)
	return res.Data, &res.Stat, err
}

// GetW returns the contents of a znode and sets a watch
func (c *Conn) GetW(path string) ([]byte, Stat, <-chan Event, error) {
	var ech <-chan Event
	res := &getDataResponse{}
	_, err := c.request(opGetData, &getDataRequest{Path: path, Watch: true}, res, func(req *request, res *responseHeader, err error) {
		if err == nil {
			ech = c.addWatcher(path, watchTypeData)
		}
	})
	if err != nil {
		return nil, nil, nil, err
	}
	return res.Data, &res.Stat, ech, err
}

func (c *Conn) Set(path string, data []byte, version int32) (Stat, error) {
	res := &setDataResponse{}
	_, err := c.request(opSetData, &SetDataRequest{path, data, version}, res, nil)
	return &res.Stat, err
}

func (c *Conn) Create(path string, data []byte, flags int32, acl []ACL) (string, error) {
	res := &createResponse{}
	_, err := c.request(opCreate, &CreateRequest{path, data, acl, flags}, res, nil)
	return res.Path, err
}

// CreateProtectedEphemeralSequential fixes a race condition if the server crashes
// after it creates the node. On reconnect the session may still be valid so the
// ephemeral node still exists. Therefore, on reconnect we need to check if a node
// with a GUID generated on create exists.
func (c *Conn) CreateProtectedEphemeralSequential(path string, data []byte, acl []ACL) (string, error) {
	var guid [16]byte
	_, err := io.ReadFull(rand.Reader, guid[:16])
	if err != nil {
		return "", err
	}
	guidStr := fmt.Sprintf("%x", guid)

	parts := strings.Split(path, "/")
	parts[len(parts)-1] = fmt.Sprintf("%s%s-%s", protectedPrefix, guidStr, parts[len(parts)-1])
	rootPath := strings.Join(parts[:len(parts)-1], "/")
	protectedPath := strings.Join(parts, "/")

	var newPath string
	for i := 0; i < 3; i++ {
		newPath, err = c.Create(protectedPath, data, FlagEphemeral|FlagSequence, acl)
		switch err {
		case ErrSessionExpired:
			// No need to search for the node since it can't exist. Just try again.
		case ErrConnectionClosed:
			children, _, err := c.Children(rootPath)
			if err != nil {
				return "", err
			}
			for _, p := range children {
				parts := strings.Split(p, "/")
				if pth := parts[len(parts)-1]; strings.HasPrefix(pth, protectedPrefix) {
					if g := pth[len(protectedPrefix) : len(protectedPrefix)+32]; g == guidStr {
						return rootPath + "/" + p, nil
					}
				}
			}
		case nil:
			return newPath, nil
		default:
			return "", err
		}
	}
	return "", err
}

func (c *Conn) Delete(path string, version int32) error {
	_, err := c.request(opDelete, &DeleteRequest{path, version}, &deleteResponse{}, nil)
	return err
}

func (c *Conn) Exists(path string) (bool, Stat, error) {
	res := &existsResponse{}
	_, err := c.request(opExists, &existsRequest{Path: path, Watch: false}, res, nil)
	exists := true
	if err == ErrNoNode {
		exists = false
		err = nil
	}
	return exists, &res.Stat, err
}

func (c *Conn) ExistsW(path string) (bool, Stat, <-chan Event, error) {
	var ech <-chan Event
	res := &existsResponse{}
	_, err := c.request(opExists, &existsRequest{Path: path, Watch: true}, res, func(req *request, res *responseHeader, err error) {
		if err == nil {
			ech = c.addWatcher(path, watchTypeData)
		} else if err == ErrNoNode {
			ech = c.addWatcher(path, watchTypeExist)
		}
	})
	exists := true
	if err == ErrNoNode {
		exists = false
		err = nil
	}
	if err != nil {
		return false, nil, nil, err
	}
	return exists, &res.Stat, ech, err
}

func (c *Conn) GetACL(path string) ([]ACL, Stat, error) {
	res := &getAclResponse{}
	_, err := c.request(opGetAcl, &getAclRequest{Path: path}, res, nil)
	return res.Acl, &res.Stat, err
}

func (c *Conn) SetACL(path string, acl []ACL, version int32) (Stat, error) {
	res := &setAclResponse{}
	_, err := c.request(opSetAcl, &setAclRequest{Path: path, Acl: acl, Version: version}, res, nil)
	return &res.Stat, err
}

func (c *Conn) Sync(path string) (string, error) {
	res := &syncResponse{}
	_, err := c.request(opSync, &syncRequest{Path: path}, res, nil)
	return res.Path, err
}

type MultiOps struct {
	Create  []CreateRequest
	Delete  []DeleteRequest
	SetData []SetDataRequest
	Check   []CheckVersionRequest
}

func (c *Conn) Multi(ops MultiOps) error {
	req := &multiRequest{
		Ops:        make([]multiRequestOp, 0, len(ops.Create)+len(ops.Delete)+len(ops.SetData)+len(ops.Check)),
		DoneHeader: multiHeader{Type: -1, Done: true, Err: -1},
	}
	for _, r := range ops.Create {
		req.Ops = append(req.Ops, multiRequestOp{multiHeader{opCreate, false, -1}, r})
	}
	for _, r := range ops.SetData {
		req.Ops = append(req.Ops, multiRequestOp{multiHeader{opSetData, false, -1}, r})
	}
	for _, r := range ops.Delete {
		req.Ops = append(req.Ops, multiRequestOp{multiHeader{opDelete, false, -1}, r})
	}
	for _, r := range ops.Check {
		req.Ops = append(req.Ops, multiRequestOp{multiHeader{opCheck, false, -1}, r})
	}
	res := &multiResponse{}
	_, err := c.request(opMulti, req, res, nil)
	return err
}
