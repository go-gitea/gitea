package hbase

import (
	"github.com/pingcap/go-hbase/proto"
	pb "github.com/golang/protobuf/proto"
)

type CoprocessorServiceCall struct {
	Row          []byte
	ServiceName  string
	MethodName   string
	RequestParam []byte
}

func (c *CoprocessorServiceCall) ToProto() pb.Message {
	return &proto.CoprocessorServiceCall{
		Row:         c.Row,
		ServiceName: pb.String(c.ServiceName),
		MethodName:  pb.String(c.MethodName),
		Request:     c.RequestParam,
	}
}
