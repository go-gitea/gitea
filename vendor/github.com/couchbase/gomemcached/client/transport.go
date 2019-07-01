package memcached

import (
	"errors"
	"io"

	"github.com/couchbase/gomemcached"
)

var errNoConn = errors.New("no connection")

// UnwrapMemcachedError converts memcached errors to normal responses.
//
// If the error is a memcached response, declare the error to be nil
// so a client can handle the status without worrying about whether it
// indicates success or failure.
func UnwrapMemcachedError(rv *gomemcached.MCResponse,
	err error) (*gomemcached.MCResponse, error) {

	if rv == err {
		return rv, nil
	}
	return rv, err
}

// ReceiveHook is called after every packet is received (or attempted to be)
var ReceiveHook func(*gomemcached.MCResponse, int, error)

func getResponse(s io.Reader, hdrBytes []byte) (rv *gomemcached.MCResponse, n int, err error) {
	if s == nil {
		return nil, 0, errNoConn
	}

	rv = &gomemcached.MCResponse{}
	n, err = rv.Receive(s, hdrBytes)

	if ReceiveHook != nil {
		ReceiveHook(rv, n, err)
	}

	if err == nil && (rv.Status != gomemcached.SUCCESS && rv.Status != gomemcached.AUTH_CONTINUE) {
		err = rv
	}
	return rv, n, err
}

// TransmitHook is called after each packet is transmitted.
var TransmitHook func(*gomemcached.MCRequest, int, error)

func transmitRequest(o io.Writer, req *gomemcached.MCRequest) (int, error) {
	if o == nil {
		return 0, errNoConn
	}
	n, err := req.Transmit(o)
	if TransmitHook != nil {
		TransmitHook(req, n, err)
	}
	return n, err
}

func transmitResponse(o io.Writer, res *gomemcached.MCResponse) (int, error) {
	if o == nil {
		return 0, errNoConn
	}
	n, err := res.Transmit(o)
	return n, err
}
