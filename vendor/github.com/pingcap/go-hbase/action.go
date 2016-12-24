package hbase

import (
	pb "github.com/golang/protobuf/proto"
	"github.com/juju/errors"
	"github.com/ngaut/log"
	"github.com/pingcap/go-hbase/proto"
)

type action interface {
	ToProto() pb.Message
}

func (c *client) innerCall(table, row []byte, action action, useCache bool) (*call, error) {
	region, err := c.LocateRegion(table, row, useCache)
	if err != nil {
		return nil, errors.Trace(err)
	}

	conn, err := c.getClientConn(region.Server)
	if err != nil {
		return nil, errors.Trace(err)
	}

	regionSpecifier := &proto.RegionSpecifier{
		Type:  proto.RegionSpecifier_REGION_NAME.Enum(),
		Value: []byte(region.Name),
	}

	var cl *call
	switch a := action.(type) {
	case *Get:
		cl = newCall(&proto.GetRequest{
			Region: regionSpecifier,
			Get:    a.ToProto().(*proto.Get),
		})
	case *Put, *Delete:
		cl = newCall(&proto.MutateRequest{
			Region:   regionSpecifier,
			Mutation: a.ToProto().(*proto.MutationProto),
		})

	case *CoprocessorServiceCall:
		cl = newCall(&proto.CoprocessorServiceRequest{
			Region: regionSpecifier,
			Call:   a.ToProto().(*proto.CoprocessorServiceCall),
		})
	default:
		return nil, errors.Errorf("Unknown action - %T - %v", action, action)
	}

	err = conn.call(cl)
	if err != nil {
		// If failed, remove bad server conn cache.
		cachedKey := cachedConnKey(region.Server, ClientService)
		delete(c.cachedConns, cachedKey)
		return nil, errors.Trace(err)
	}

	return cl, nil
}

func (c *client) innerDo(table, row []byte, action action, useCache bool) (pb.Message, error) {
	// Try to create and send a new resuqest call.
	cl, err := c.innerCall(table, row, action, useCache)
	if err != nil {
		log.Warnf("inner call failed - %v", errors.ErrorStack(err))
		return nil, errors.Trace(err)
	}

	// Wait and receive the result.
	return <-cl.responseCh, nil
}

func (c *client) do(table, row []byte, action action, useCache bool) (pb.Message, error) {
	var (
		result pb.Message
		err    error
	)

LOOP:
	for i := 0; i < c.maxRetries; i++ {
		result, err = c.innerDo(table, row, action, useCache)
		if err == nil {
			switch r := result.(type) {
			case *exception:
				err = errors.New(r.msg)
				// If get an execption response, clean old region cache.
				c.CleanRegionCache(table)
			default:
				break LOOP
			}
		}

		useCache = false
		log.Warnf("Retrying action for the %d time(s), error - %v", i+1, errors.ErrorStack(err))
		retrySleep(i + 1)
	}

	return result, errors.Trace(err)
}
