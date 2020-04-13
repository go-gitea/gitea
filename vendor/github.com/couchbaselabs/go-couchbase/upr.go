package couchbase

import (
	"log"
	"sync"
	"time"

	"fmt"
	"github.com/couchbase/gomemcached"
	"github.com/couchbase/gomemcached/client"
	"github.com/couchbase/goutils/logging"
)

// A UprFeed streams mutation events from a bucket.
//
// Events from the bucket can be read from the channel 'C'.  Remember
// to call Close() on it when you're done, unless its channel has
// closed itself already.
type UprFeed struct {
	C <-chan *memcached.UprEvent

	bucket          *Bucket
	nodeFeeds       map[string]*FeedInfo     // The UPR feeds of the individual nodes
	output          chan *memcached.UprEvent // Same as C but writeably-typed
	outputClosed    bool
	quit            chan bool
	name            string // name of this UPR feed
	sequence        uint32 // sequence number for this feed
	connected       bool
	killSwitch      chan bool
	closing         bool
	wg              sync.WaitGroup
	dcp_buffer_size uint32
	data_chan_size  int
}

// UprFeed from a single connection
type FeedInfo struct {
	uprFeed   *memcached.UprFeed // UPR feed handle
	host      string             // hostname
	connected bool               // connected
	quit      chan bool          // quit channel
}

type FailoverLog map[uint16]memcached.FailoverLog

// GetFailoverLogs, get the failover logs for a set of vbucket ids
func (b *Bucket) GetFailoverLogs(vBuckets []uint16) (FailoverLog, error) {

	// map vbids to their corresponding hosts
	vbHostList := make(map[string][]uint16)
	vbm := b.VBServerMap()
	if len(vbm.VBucketMap) < len(vBuckets) {
		return nil, fmt.Errorf("vbmap smaller than vbucket list: %v vs. %v",
			vbm.VBucketMap, vBuckets)
	}

	for _, vb := range vBuckets {
		masterID := vbm.VBucketMap[vb][0]
		master := b.getMasterNode(masterID)
		if master == "" {
			return nil, fmt.Errorf("No master found for vb %d", vb)
		}

		vbList := vbHostList[master]
		if vbList == nil {
			vbList = make([]uint16, 0)
		}
		vbList = append(vbList, vb)
		vbHostList[master] = vbList
	}

	failoverLogMap := make(FailoverLog)
	for _, serverConn := range b.getConnPools(false /* not already locked */) {

		vbList := vbHostList[serverConn.host]
		if vbList == nil {
			continue
		}

		mc, err := serverConn.Get()
		if err != nil {
			logging.Infof("No Free connections for vblist %v", vbList)
			return nil, fmt.Errorf("No Free connections for host %s",
				serverConn.host)

		}
		// close the connection so that it doesn't get reused for upr data
		// connection
		defer mc.Close()
		failoverlogs, err := mc.UprGetFailoverLog(vbList)
		if err != nil {
			return nil, fmt.Errorf("Error getting failover log %s host %s",
				err.Error(), serverConn.host)

		}

		for vb, log := range failoverlogs {
			failoverLogMap[vb] = *log
		}
	}

	return failoverLogMap, nil
}

func (b *Bucket) StartUprFeed(name string, sequence uint32) (*UprFeed, error) {
	return b.StartUprFeedWithConfig(name, sequence, 10, DEFAULT_WINDOW_SIZE)
}

// StartUprFeed creates and starts a new Upr feed
// No data will be sent on the channel unless vbuckets streams are requested
func (b *Bucket) StartUprFeedWithConfig(name string, sequence uint32, data_chan_size int, dcp_buffer_size uint32) (*UprFeed, error) {

	feed := &UprFeed{
		bucket:          b,
		output:          make(chan *memcached.UprEvent, data_chan_size),
		quit:            make(chan bool),
		nodeFeeds:       make(map[string]*FeedInfo, 0),
		name:            name,
		sequence:        sequence,
		killSwitch:      make(chan bool),
		dcp_buffer_size: dcp_buffer_size,
		data_chan_size:  data_chan_size,
	}

	err := feed.connectToNodes()
	if err != nil {
		return nil, fmt.Errorf("Cannot connect to bucket %s", err.Error())
	}
	feed.connected = true
	go feed.run()

	feed.C = feed.output
	return feed, nil
}

// UprRequestStream starts a stream for a vb on a feed
func (feed *UprFeed) UprRequestStream(vb uint16, opaque uint16, flags uint32,
	vuuid, startSequence, endSequence, snapStart, snapEnd uint64) error {

	defer func() {
		if r := recover(); r != nil {
			log.Panicf("Panic in UprRequestStream. Feed %v Bucket %v", feed, feed.bucket)
		}
	}()

	vbm := feed.bucket.VBServerMap()
	if len(vbm.VBucketMap) < int(vb) {
		return fmt.Errorf("vbmap smaller than vbucket list: %v vs. %v",
			vb, vbm.VBucketMap)
	}

	if int(vb) >= len(vbm.VBucketMap) {
		return fmt.Errorf("Invalid vbucket id %d", vb)
	}

	masterID := vbm.VBucketMap[vb][0]
	master := feed.bucket.getMasterNode(masterID)
	if master == "" {
		return fmt.Errorf("Master node not found for vbucket %d", vb)
	}
	singleFeed := feed.nodeFeeds[master]
	if singleFeed == nil {
		return fmt.Errorf("UprFeed for this host not found")
	}

	if err := singleFeed.uprFeed.UprRequestStream(vb, opaque, flags,
		vuuid, startSequence, endSequence, snapStart, snapEnd); err != nil {
		return err
	}

	return nil
}

// UprCloseStream ends a vbucket stream.
func (feed *UprFeed) UprCloseStream(vb, opaqueMSB uint16) error {

	defer func() {
		if r := recover(); r != nil {
			log.Panicf("Panic in UprCloseStream. Feed %v Bucket %v ", feed, feed.bucket)
		}
	}()

	vbm := feed.bucket.VBServerMap()
	if len(vbm.VBucketMap) < int(vb) {
		return fmt.Errorf("vbmap smaller than vbucket list: %v vs. %v",
			vb, vbm.VBucketMap)
	}

	if int(vb) >= len(vbm.VBucketMap) {
		return fmt.Errorf("Invalid vbucket id %d", vb)
	}

	masterID := vbm.VBucketMap[vb][0]
	master := feed.bucket.getMasterNode(masterID)
	if master == "" {
		return fmt.Errorf("Master node not found for vbucket %d", vb)
	}
	singleFeed := feed.nodeFeeds[master]
	if singleFeed == nil {
		return fmt.Errorf("UprFeed for this host not found")
	}

	if err := singleFeed.uprFeed.CloseStream(vb, opaqueMSB); err != nil {
		return err
	}
	return nil
}

// Goroutine that runs the feed
func (feed *UprFeed) run() {
	retryInterval := initialRetryInterval
	bucketOK := true
	for {
		// Connect to the UPR feed of each server node:
		if bucketOK {
			// Run until one of the sub-feeds fails:
			select {
			case <-feed.killSwitch:
			case <-feed.quit:
				return
			}
			//feed.closeNodeFeeds()
			retryInterval = initialRetryInterval
		}

		if feed.closing == true {
			// we have been asked to shut down
			return
		}

		// On error, try to refresh the bucket in case the list of nodes changed:
		logging.Infof("go-couchbase: UPR connection lost; reconnecting to bucket %q in %v",
			feed.bucket.Name, retryInterval)

		if err := feed.bucket.Refresh(); err != nil {
			// if we fail to refresh the bucket, exit the feed
			// MB-14917
			logging.Infof("Unable to refresh bucket %s ", err.Error())
			close(feed.output)
			feed.outputClosed = true
			feed.closeNodeFeeds()
			return
		}

		// this will only connect to nodes that are not connected or changed
		// user will have to reconnect the stream
		err := feed.connectToNodes()
		if err != nil {
			logging.Infof("Unable to connect to nodes..exit ")
			close(feed.output)
			feed.outputClosed = true
			feed.closeNodeFeeds()
			return
		}
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

func (feed *UprFeed) connectToNodes() (err error) {
	nodeCount := 0
	for _, serverConn := range feed.bucket.getConnPools(false /* not already locked */) {

		// this maybe a reconnection, so check if the connection to the node
		// already exists. Connect only if the node is not found in the list
		// or connected == false
		nodeFeed := feed.nodeFeeds[serverConn.host]

		if nodeFeed != nil && nodeFeed.connected == true {
			continue
		}

		var singleFeed *memcached.UprFeed
		var name string
		if feed.name == "" {
			name = "DefaultUprClient"
		} else {
			name = feed.name
		}
		singleFeed, err = serverConn.StartUprFeed(name, feed.sequence, feed.dcp_buffer_size, feed.data_chan_size)
		if err != nil {
			logging.Errorf("go-couchbase: Error connecting to upr feed of %s: %v", serverConn.host, err)
			feed.closeNodeFeeds()
			return
		}
		// add the node to the connection map
		feedInfo := &FeedInfo{
			uprFeed:   singleFeed,
			connected: true,
			host:      serverConn.host,
			quit:      make(chan bool),
		}
		feed.nodeFeeds[serverConn.host] = feedInfo
		go feed.forwardUprEvents(feedInfo, feed.killSwitch, serverConn.host)
		feed.wg.Add(1)
		nodeCount++
	}
	if nodeCount == 0 {
		return fmt.Errorf("No connection to bucket")
	}

	return nil
}

// Goroutine that forwards Upr events from a single node's feed to the aggregate feed.
func (feed *UprFeed) forwardUprEvents(nodeFeed *FeedInfo, killSwitch chan bool, host string) {
	singleFeed := nodeFeed.uprFeed

	defer func() {
		feed.wg.Done()
		if r := recover(); r != nil {
			//if feed is not closing, re-throw the panic
			if feed.outputClosed != true && feed.closing != true {
				panic(r)
			} else {
				logging.Errorf("Panic is recovered. Since feed is closed, exit gracefully")

			}
		}
	}()

	for {
		select {
		case <-nodeFeed.quit:
			nodeFeed.connected = false
			return

		case event, ok := <-singleFeed.C:
			if !ok {
				if singleFeed.Error != nil {
					logging.Errorf("go-couchbase: Upr feed from %s failed: %v", host, singleFeed.Error)
				}
				killSwitch <- true
				return
			}
			if feed.outputClosed == true {
				// someone closed the node feed
				logging.Infof("Node need closed, returning from forwardUprEvent")
				return
			}
			feed.output <- event
			if event.Status == gomemcached.NOT_MY_VBUCKET {
				logging.Infof(" Got a not my vbucket error !! ")
				if err := feed.bucket.Refresh(); err != nil {
					logging.Errorf("Unable to refresh bucket %s ", err.Error())
					feed.closeNodeFeeds()
					return
				}
				// this will only connect to nodes that are not connected or changed
				// user will have to reconnect the stream
				if err := feed.connectToNodes(); err != nil {
					logging.Errorf("Unable to connect to nodes %s", err.Error())
					return
				}

			}
		}
	}
}

func (feed *UprFeed) closeNodeFeeds() {
	for _, f := range feed.nodeFeeds {
		logging.Infof(" Sending close to forwardUprEvent ")
		close(f.quit)
		f.uprFeed.Close()
	}
	feed.nodeFeeds = nil
}

// Close a Upr feed.
func (feed *UprFeed) Close() error {
	select {
	case <-feed.quit:
		return nil
	default:
	}

	feed.closing = true
	feed.closeNodeFeeds()
	close(feed.quit)

	feed.wg.Wait()
	if feed.outputClosed == false {
		feed.outputClosed = true
		close(feed.output)
	}

	return nil
}
