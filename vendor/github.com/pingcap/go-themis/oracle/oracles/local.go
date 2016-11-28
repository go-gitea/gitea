package oracles

import (
	"sync"
	"time"

	"github.com/pingcap/go-themis/oracle"
)

const epochShiftBits = 18

var _ oracle.Oracle = &localOracle{}

type localOracle struct {
	mu              sync.Mutex
	lastTimeStampTs int64
	n               int64
}

// NewLocalOracle creates an Oracle that use local time as data source.
func NewLocalOracle() oracle.Oracle {
	return &localOracle{}
}

func (l *localOracle) IsExpired(lockTs uint64, TTL uint64) bool {
	beginMs := lockTs >> epochShiftBits
	return uint64(time.Now().UnixNano()/int64(time.Millisecond)) >= (beginMs + TTL)
}

func (l *localOracle) GetTimestamp() (uint64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	ts := (time.Now().UnixNano() / int64(time.Millisecond)) << epochShiftBits
	if l.lastTimeStampTs == ts {
		l.n++
		return uint64(ts + l.n), nil
	} else {
		l.lastTimeStampTs = ts
		l.n = 0
	}
	return uint64(ts), nil
}
