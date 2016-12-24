package oracles

import (
	"time"

	"github.com/juju/errors"
	"github.com/ngaut/tso/client"
	"github.com/pingcap/go-themis/oracle"
)

const maxRetryCnt = 3

var _ oracle.Oracle = &remoteOracle{}

// remoteOracle is an oracle that use a remote data source.
type remoteOracle struct {
	c *client.Client
}

// NewRemoteOracle creates an oracle that use a remote data source.
// Refer https://github.com/ngaut/tso for more details.
func NewRemoteOracle(zks, path string) oracle.Oracle {
	return &remoteOracle{
		c: client.NewClient(&client.Conf{
			ZKAddr:   zks,
			RootPath: path,
		}),
	}
}

func (t *remoteOracle) IsExpired(lockTs uint64, TTL uint64) bool {
	beginMs := lockTs >> epochShiftBits
	// TODO records the local wall time when getting beginMs from TSO
	return uint64(time.Now().UnixNano()/int64(time.Millisecond)) >= (beginMs + TTL)
}

// GetTimestamp gets timestamp from remote data source.
func (t *remoteOracle) GetTimestamp() (uint64, error) {
	var err error
	for i := 0; i < maxRetryCnt; i++ {
		ts, e := t.c.GoGetTimestamp().GetTS()
		if e == nil {
			return uint64((ts.Physical << epochShiftBits) + ts.Logical), nil
		}
		err = errors.Trace(e)
	}
	return 0, err
}
