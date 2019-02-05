package couchbase

import (
	"github.com/couchbase/gomemcached/client"
	"github.com/couchbase/goutils/logging"
	"sync"
	"time"
)

const initialRetryInterval = 1 * time.Second
const maximumRetryInterval = 30 * time.Second

// A TapFeed streams mutation events from a bucket.
//
// Events from the bucket can be read from the channel 'C'.  Remember
// to call Close() on it when you're done, unless its channel has
// closed itself already.
type TapFeed struct {
	C <-chan memcached.TapEvent

	bucket    *Bucket
	args      *memcached.TapArguments
	nodeFeeds []*memcached.TapFeed    // The TAP feeds of the individual nodes
	output    chan memcached.TapEvent // Same as C but writeably-typed
	wg        sync.WaitGroup
	quit      chan bool
}

// StartTapFeed creates and starts a new Tap feed
func (b *Bucket) StartTapFeed(args *memcached.TapArguments) (*TapFeed, error) {
	if args == nil {
		defaultArgs := memcached.DefaultTapArguments()
		args = &defaultArgs
	}

	feed := &TapFeed{
		bucket: b,
		args:   args,
		output: make(chan memcached.TapEvent, 10),
		quit:   make(chan bool),
	}

	go feed.run()

	feed.C = feed.output
	return feed, nil
}

// Goroutine that runs the feed
func (feed *TapFeed) run() {
	retryInterval := initialRetryInterval
	bucketOK := true
	for {
		// Connect to the TAP feed of each server node:
		if bucketOK {
			killSwitch, err := feed.connectToNodes()
			if err == nil {
				// Run until one of the sub-feeds fails:
				select {
				case <-killSwitch:
				case <-feed.quit:
					return
				}
				feed.closeNodeFeeds()
				retryInterval = initialRetryInterval
			}
		}

		// On error, try to refresh the bucket in case the list of nodes changed:
		logging.Infof("go-couchbase: TAP connection lost; reconnecting to bucket %q in %v",
			feed.bucket.Name, retryInterval)
		err := feed.bucket.Refresh()
		bucketOK = err == nil

		select {
		case <-time.After(retryInterval):
		case <-feed.quit:
			return
		}
		if retryInterval *= 2; retryInterval > maximumRetryInterval {
			retryInterval = maximumRetryInterval
		}
	}
}

func (feed *TapFeed) connectToNodes() (killSwitch chan bool, err error) {
	killSwitch = make(chan bool)
	for _, serverConn := range feed.bucket.getConnPools(false /* not already locked */) {
		var singleFeed *memcached.TapFeed
		singleFeed, err = serverConn.StartTapFeed(feed.args)
		if err != nil {
			logging.Errorf("go-couchbase: Error connecting to tap feed of %s: %v", serverConn.host, err)
			feed.closeNodeFeeds()
			return
		}
		feed.nodeFeeds = append(feed.nodeFeeds, singleFeed)
		go feed.forwardTapEvents(singleFeed, killSwitch, serverConn.host)
		feed.wg.Add(1)
	}
	return
}

// Goroutine that forwards Tap events from a single node's feed to the aggregate feed.
func (feed *TapFeed) forwardTapEvents(singleFeed *memcached.TapFeed, killSwitch chan bool, host string) {
	defer feed.wg.Done()
	for {
		select {
		case event, ok := <-singleFeed.C:
			if !ok {
				if singleFeed.Error != nil {
					logging.Errorf("go-couchbase: Tap feed from %s failed: %v", host, singleFeed.Error)
				}
				killSwitch <- true
				return
			}
			feed.output <- event
		case <-feed.quit:
			return
		}
	}
}

func (feed *TapFeed) closeNodeFeeds() {
	for _, f := range feed.nodeFeeds {
		f.Close()
	}
	feed.nodeFeeds = nil
}

// Close a Tap feed.
func (feed *TapFeed) Close() error {
	select {
	case <-feed.quit:
		return nil
	default:
	}

	feed.closeNodeFeeds()
	close(feed.quit)
	feed.wg.Wait()
	close(feed.output)
	return nil
}
