package client

import (
	"container/list"
	"time"

	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/ngaut/tso/proto"
	"github.com/ngaut/tso/util"
	"github.com/ngaut/zkhelper"
)

const (
	maxPipelineRequest = 100000
)

// Client is a timestamp oracle client.
type Client struct {
	requests chan *PipelineRequest

	pending *list.List
	conf    *Conf

	addr string

	leaderCh chan string
}

// Conf is the configuration.
type Conf struct {
	// tso server address, it will be deprecated later.
	ServerAddr string

	// ZKAddr is for zookeeper address, if set, client will ignore ServerAddr
	// and find the leader tso server address in zookeeper.
	// Later ServerAddr is just for simple test and backward compatibility.
	ZKAddr string

	// root path is the tso server saving in zookeeper, like /zk/tso.
	RootPath string
}

// PipelineRequest let you get the timestamp with pipeline.
type PipelineRequest struct {
	done  chan error
	reply *proto.Response
}

func newPipelineRequest() *PipelineRequest {
	return &PipelineRequest{
		done: make(chan error, 1),
	}
}

// MarkDone sets the repsone for current request.
func (pr *PipelineRequest) MarkDone(reply *proto.Response, err error) {
	if err != nil {
		pr.reply = nil
	}
	pr.reply = reply
	pr.done <- errors.Trace(err)
}

// GetTS gets the timestamp.
func (pr *PipelineRequest) GetTS() (*proto.Timestamp, error) {
	err := <-pr.done
	if err != nil {
		return nil, errors.Trace(err)
	}

	return &pr.reply.Timestamp, nil
}

// NewClient creates a timestamp oracle client.
func NewClient(conf *Conf) *Client {
	c := &Client{
		requests: make(chan *PipelineRequest, maxPipelineRequest),
		pending:  list.New(),
		conf:     conf,
		leaderCh: make(chan string, 1),
	}

	if len(conf.ZKAddr) == 0 {
		c.leaderCh <- conf.ServerAddr
	} else {
		go c.watchLeader()
	}

	go c.workerLoop()

	return c
}

func (c *Client) cleanupPending(err error) {
	log.Warn(err)
	length := c.pending.Len()
	for i := 0; i < length; i++ {
		e := c.pending.Front()
		c.pending.Remove(e)
		e.Value.(*PipelineRequest).MarkDone(nil, err)
	}

	// clear request in channel too
	length = len(c.requests)
	for i := 0; i < length; i++ {
		req := <-c.requests
		req.MarkDone(nil, err)
	}
}

func (c *Client) notifyOne(reply *proto.Response) {
	e := c.pending.Front()
	c.pending.Remove(e)
	req := e.Value.(*PipelineRequest)
	req.MarkDone(reply, nil)
}

func (c *Client) writeRequests(session *Conn) error {
	var protoHdr [1]byte
	for i := 0; i < c.pending.Len(); i++ {
		session.Write(protoHdr[:])
	}
	return session.Flush()
}

func (c *Client) handleResponse(session *Conn) error {
	length := c.pending.Len()
	for i := 0; i < length; i++ {
		var resp proto.Response
		err := resp.Decode(session)
		if err != nil {
			return errors.Trace(err)
		}
		c.notifyOne(&resp)
	}

	return nil
}

func (c *Client) do() error {
	session, err := NewConnection(c.addr, time.Duration(1*time.Second))
	if err != nil {
		return errors.Trace(err)
	}

	log.Debugf("connect tso server %s ok", c.addr)

	defer session.Close()
	for {
		select {
		case req := <-c.requests:
			c.pending.PushBack(req)
			length := len(c.requests)
			for i := 0; i < length; i++ {
				req = <-c.requests
				c.pending.PushBack(req)
			}

			err = c.writeRequests(session)
			if err != nil {
				return errors.Trace(err)
			}
			err = c.handleResponse(session)
			if err != nil {
				return errors.Trace(err)
			}
		case addr := <-c.leaderCh:
			oldAddr := c.addr
			c.addr = addr
			return errors.Errorf("leader change %s -> %s", oldAddr, addr)
		}
	}
}

func (c *Client) workerLoop() {
	// first get tso leader
	c.addr = <-c.leaderCh
	log.Debugf("try to connect tso server %s", c.addr)

	for {
		err := c.do()
		if err != nil {
			c.cleanupPending(err)
		}
		select {
		case <-time.After(1 * time.Second):
		case addr := <-c.leaderCh:
			// If old tso server down, NewConnection will fail and return immediately in do function,
			// so we must check leader change here.
			log.Warnf("leader change %s -> %s", c.addr, addr)
			c.addr = addr
			// Wait some time to let tso server allow accepting connections.
			time.Sleep(1 * time.Second)
		}
	}
}

func (c *Client) watchLeader() {
	var (
		conn zkhelper.Conn
		err  error
	)

	for {
		conn, err = zkhelper.ConnectToZkWithTimeout(c.conf.ZKAddr, time.Second)
		if err != nil {
			log.Errorf("connect zk err %v, retry later", err)
			time.Sleep(3 * time.Second)
			continue
		}
		break
	}

	defer conn.Close()

	var lastAddr string

	for {
		addr, watcher, err := util.GetWatchLeader(conn, c.conf.RootPath)
		if err != nil {
			log.Errorf("get tso leader err %v, retry later", err)
			time.Sleep(3 * time.Second)
			continue
		}

		if lastAddr != addr {
			log.Warnf("leader change %s -> %s", lastAddr, addr)
			lastAddr = addr
			c.leaderCh <- addr
		}

		// watch the leader changes.
		<-watcher
	}
}

// GoGetTimestamp returns a PipelineRequest so you can get the timestamp later.
func (c *Client) GoGetTimestamp() *PipelineRequest {
	pr := newPipelineRequest()
	c.requests <- pr
	return pr
}
