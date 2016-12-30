package hbase

import (
	"github.com/juju/errors"
	"github.com/pingcap/go-hbase/proto"
)

func (c *client) Delete(table string, del *Delete) (bool, error) {
	response, err := c.do([]byte(table), del.GetRow(), del, true)
	if err != nil {
		return false, errors.Trace(err)
	}

	switch r := response.(type) {
	case *proto.MutateResponse:
		return r.GetProcessed(), nil
	}
	return false, errors.Errorf("Invalid response seen [response: %#v]", response)
}

func (c *client) Get(table string, get *Get) (*ResultRow, error) {
	response, err := c.do([]byte(table), get.GetRow(), get, true)
	if err != nil {
		return nil, errors.Trace(err)
	}

	switch r := response.(type) {
	case *proto.GetResponse:
		res := r.GetResult()
		if res == nil {
			return nil, errors.Errorf("Empty response: [table=%s] [row=%q]", table, get.GetRow())
		}

		return NewResultRow(res), nil
	case *exception:
		return nil, errors.New(r.msg)
	}
	return nil, errors.Errorf("Invalid response seen [response: %#v]", response)
}

func (c *client) Put(table string, put *Put) (bool, error) {
	response, err := c.do([]byte(table), put.GetRow(), put, true)
	if err != nil {
		return false, errors.Trace(err)
	}

	switch r := response.(type) {
	case *proto.MutateResponse:
		return r.GetProcessed(), nil
	}
	return false, errors.Errorf("Invalid response seen [response: %#v]", response)
}

func (c *client) ServiceCall(table string, call *CoprocessorServiceCall) (*proto.CoprocessorServiceResponse, error) {
	response, err := c.do([]byte(table), call.Row, call, true)
	if err != nil {
		return nil, errors.Trace(err)
	}

	switch r := response.(type) {
	case *proto.CoprocessorServiceResponse:
		return r, nil
	case *exception:
		return nil, errors.New(r.msg)
	}
	return nil, errors.Errorf("Invalid response seen [response: %#v]", response)
}
