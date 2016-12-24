package localstore

import (
	"errors"
	"sync"
	"time"

	"github.com/ngaut/log"
	"github.com/pingcap/tidb/kv"
)

// ErrOverflow is the error returned by CurrentVersion, it describes if
// there're too many versions allocations in a very short period of time, ID
// may conflict.
var ErrOverflow = errors.New("overflow when allocating new version")

// LocalVersionProvider uses local timestamp for version.
type LocalVersionProvider struct {
	mu            sync.Mutex
	lastTimestamp uint64
	// logical guaranteed version's monotonic increasing for calls when lastTimestamp
	// are equal.
	logical uint64
}

const (
	timePrecisionOffset = 18
)

func time2TsPhysical(t time.Time) uint64 {
	return uint64((t.UnixNano() / int64(time.Millisecond)) << timePrecisionOffset)
}

func version2Second(v kv.Version) int64 {
	return int64(v.Ver>>timePrecisionOffset) / 1000
}

// CurrentVersion implements the VersionProvider's GetCurrentVer interface.
func (l *LocalVersionProvider) CurrentVersion() (kv.Version, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	for {
		var ts uint64
		ts = time2TsPhysical(time.Now())

		if l.lastTimestamp > ts {
			log.Error("[kv] invalid physical time stamp")
			continue
		}

		if l.lastTimestamp == uint64(ts) {
			l.logical++
			if l.logical >= 1<<timePrecisionOffset {
				return kv.Version{}, ErrOverflow
			}
			return kv.Version{Ver: ts + l.logical}, nil
		}
		l.lastTimestamp = ts
		l.logical = 0
		return kv.Version{Ver: ts}, nil
	}
}

func localVersionToTimestamp(ver kv.Version) uint64 {
	return ver.Ver >> timePrecisionOffset
}
