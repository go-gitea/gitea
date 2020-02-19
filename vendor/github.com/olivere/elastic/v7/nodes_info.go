// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/olivere/elastic/v7/uritemplates"
)

// NodesInfoService allows to retrieve one or more or all of the
// cluster nodes information.
// It is documented at https://www.elastic.co/guide/en/elasticsearch/reference/7.0/cluster-nodes-info.html.
type NodesInfoService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	nodeId       []string
	metric       []string
	flatSettings *bool
}

// NewNodesInfoService creates a new NodesInfoService.
func NewNodesInfoService(client *Client) *NodesInfoService {
	return &NodesInfoService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *NodesInfoService) Pretty(pretty bool) *NodesInfoService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *NodesInfoService) Human(human bool) *NodesInfoService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *NodesInfoService) ErrorTrace(errorTrace bool) *NodesInfoService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *NodesInfoService) FilterPath(filterPath ...string) *NodesInfoService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *NodesInfoService) Header(name string, value string) *NodesInfoService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *NodesInfoService) Headers(headers http.Header) *NodesInfoService {
	s.headers = headers
	return s
}

// NodeId is a list of node IDs or names to limit the returned information.
// Use "_local" to return information from the node you're connecting to,
// leave empty to get information from all nodes.
func (s *NodesInfoService) NodeId(nodeId ...string) *NodesInfoService {
	s.nodeId = append(s.nodeId, nodeId...)
	return s
}

// Metric is a list of metrics you wish returned. Leave empty to return all.
// Valid metrics are: settings, os, process, jvm, thread_pool, network,
// transport, http, and plugins.
func (s *NodesInfoService) Metric(metric ...string) *NodesInfoService {
	s.metric = append(s.metric, metric...)
	return s
}

// FlatSettings returns settings in flat format (default: false).
func (s *NodesInfoService) FlatSettings(flatSettings bool) *NodesInfoService {
	s.flatSettings = &flatSettings
	return s
}

// buildURL builds the URL for the operation.
func (s *NodesInfoService) buildURL() (string, url.Values, error) {
	var nodeId, metric string

	if len(s.nodeId) > 0 {
		nodeId = strings.Join(s.nodeId, ",")
	} else {
		nodeId = "_all"
	}

	if len(s.metric) > 0 {
		metric = strings.Join(s.metric, ",")
	} else {
		metric = "_all"
	}

	// Build URL
	path, err := uritemplates.Expand("/_nodes/{node_id}/{metric}", map[string]string{
		"node_id": nodeId,
		"metric":  metric,
	})
	if err != nil {
		return "", url.Values{}, err
	}

	// Add query string parameters
	params := url.Values{}
	if v := s.pretty; v != nil {
		params.Set("pretty", fmt.Sprint(*v))
	}
	if v := s.human; v != nil {
		params.Set("human", fmt.Sprint(*v))
	}
	if v := s.errorTrace; v != nil {
		params.Set("error_trace", fmt.Sprint(*v))
	}
	if len(s.filterPath) > 0 {
		params.Set("filter_path", strings.Join(s.filterPath, ","))
	}
	if s.flatSettings != nil {
		params.Set("flat_settings", fmt.Sprintf("%v", *s.flatSettings))
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *NodesInfoService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *NodesInfoService) Do(ctx context.Context) (*NodesInfoResponse, error) {
	// Check pre-conditions
	if err := s.Validate(); err != nil {
		return nil, err
	}

	// Get URL for request
	path, params, err := s.buildURL()
	if err != nil {
		return nil, err
	}

	// Get HTTP response
	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:  "GET",
		Path:    path,
		Params:  params,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(NodesInfoResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// NodesInfoResponse is the response of NodesInfoService.Do.
type NodesInfoResponse struct {
	ClusterName string                    `json:"cluster_name"`
	Nodes       map[string]*NodesInfoNode `json:"nodes"`
}

// NodesInfoNode represents information about a node in the cluster.
type NodesInfoNode struct {
	// Name of the node, e.g. "Mister Fear"
	Name string `json:"name"`
	// TransportAddress, e.g. "127.0.0.1:9300"
	TransportAddress string `json:"transport_address"`
	// Host is the host name, e.g. "macbookair"
	Host string `json:"host"`
	// IP is the IP address, e.g. "192.168.1.2"
	IP string `json:"ip"`
	// Version is the Elasticsearch version running on the node, e.g. "1.4.3"
	Version string `json:"version"`
	// BuildHash is the Elasticsearch build bash, e.g. "36a29a7"
	BuildHash string `json:"build_hash"`

	// TotalIndexingBuffer represents the total heap allowed to be used to
	// hold recently indexed documents before they must be written to disk.
	TotalIndexingBuffer int64 `json:"total_indexing_buffer"` // e.g. 16gb
	// TotalIndexingBufferInBytes is the same as TotalIndexingBuffer, but
	// expressed in bytes.
	TotalIndexingBufferInBytes string `json:"total_indexing_buffer_in_bytes"`

	// Roles of the node, e.g. [master, ingest, data]
	Roles []string `json:"roles"`

	// Attributes of the node.
	Attributes map[string]string `json:"attributes"`

	// Settings of the node, e.g. paths and pidfile.
	Settings map[string]interface{} `json:"settings"`

	// OS information, e.g. CPU and memory.
	OS *NodesInfoNodeOS `json:"os"`

	// Process information, e.g. max file descriptors.
	Process *NodesInfoNodeProcess `json:"process"`

	// JVM information, e.g. VM version.
	JVM *NodesInfoNodeJVM `json:"jvm"`

	// ThreadPool information.
	ThreadPool *NodesInfoNodeThreadPool `json:"thread_pool"`

	// Network information.
	Transport *NodesInfoNodeTransport `json:"transport"`

	// HTTP information.
	HTTP *NodesInfoNodeHTTP `json:"http"`

	// Plugins information.
	Plugins []*NodesInfoNodePlugin `json:"plugins"`

	// Modules information.
	Modules []*NodesInfoNodeModule `json:"modules"`

	// Ingest information.
	Ingest *NodesInfoNodeIngest `json:"ingest"`
}

// HasRole returns true if the node fulfills the given role.
func (n *NodesInfoNode) HasRole(role string) bool {
	for _, r := range n.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// IsMaster returns true if the node is a master node.
func (n *NodesInfoNode) IsMaster() bool {
	return n.HasRole("master")
}

// IsData returns true if the node is a data node.
func (n *NodesInfoNode) IsData() bool {
	return n.HasRole("data")
}

// IsIngest returns true if the node is an ingest node.
func (n *NodesInfoNode) IsIngest() bool {
	return n.HasRole("ingest")
}

// NodesInfoNodeOS represents OS-specific details about a node.
type NodesInfoNodeOS struct {
	RefreshInterval         string `json:"refresh_interval"`           // e.g. 1s
	RefreshIntervalInMillis int    `json:"refresh_interval_in_millis"` // e.g. 1000
	Name                    string `json:"name"`                       // e.g. Linux
	Arch                    string `json:"arch"`                       // e.g. amd64
	Version                 string `json:"version"`                    // e.g. 4.9.87-linuxkit-aufs
	AvailableProcessors     int    `json:"available_processors"`       // e.g. 4
	AllocatedProcessors     int    `json:"allocated_processors"`       // e.g. 4
}

// NodesInfoNodeProcess represents process-related information.
type NodesInfoNodeProcess struct {
	RefreshInterval         string `json:"refresh_interval"`           // e.g. 1s
	RefreshIntervalInMillis int64  `json:"refresh_interval_in_millis"` // e.g. 1000
	ID                      int    `json:"id"`                         // process id, e.g. 87079
	Mlockall                bool   `json:"mlockall"`                   // e.g. false
}

// NodesInfoNodeJVM represents JVM-related information.
type NodesInfoNodeJVM struct {
	PID               int       `json:"pid"`                  // process id, e.g. 87079
	Version           string    `json:"version"`              // e.g. "1.8.0_161"
	VMName            string    `json:"vm_name"`              // e.g. "OpenJDK 64-Bit Server VM"
	VMVersion         string    `json:"vm_version"`           // e.g. "25.161-b14"
	VMVendor          string    `json:"vm_vendor"`            // e.g. "Oracle Corporation"
	StartTime         time.Time `json:"start_time"`           // e.g. "2018-03-30T11:06:36.644Z"
	StartTimeInMillis int64     `json:"start_time_in_millis"` // e.g. 1522407996644

	// Mem information
	Mem struct {
		HeapInit           string `json:"heap_init"`              // e.g. "1gb"
		HeapInitInBytes    int    `json:"heap_init_in_bytes"`     // e.g. 1073741824
		HeapMax            string `json:"heap_max"`               // e.g. "1007.3mb"
		HeapMaxInBytes     int    `json:"heap_max_in_bytes"`      // e.g. 1056309248
		NonHeapInit        string `json:"non_heap_init"`          // e.g. "2.4mb"
		NonHeapInitInBytes int    `json:"non_heap_init_in_bytes"` // e.g. 2555904
		NonHeapMax         string `json:"non_heap_max"`           // e.g. "0b"
		NonHeapMaxInBytes  int    `json:"non_heap_max_in_bytes"`  // e.g. 0
		DirectMax          string `json:"direct_max"`             // e.g. "1007.3mb"
		DirectMaxInBytes   int    `json:"direct_max_in_bytes"`    // e.g. 1056309248
	} `json:"mem"`

	GCCollectors []string `json:"gc_collectors"` // e.g. ["ParNew", "ConcurrentMarkSweep"]
	MemoryPools  []string `json:"memory_pools"`  // e.g. ["Code Cache", "Metaspace", "Compressed Class Space", "Par Eden Space", "Par Survivor Space", "CMS Old Gen"]

	// UsingCompressedOrdinaryObjectPointers should be a bool, but is a
	// string in 6.2.3. We use an interface{} for now so that it won't break
	// when this will be fixed in later versions of Elasticsearch.
	UsingCompressedOrdinaryObjectPointers interface{} `json:"using_compressed_ordinary_object_pointers"`

	InputArguments []string `json:"input_arguments"` // e.g. ["-Xms1g", "-Xmx1g" ...]
}

// NodesInfoNodeThreadPool represents information about the thread pool.
type NodesInfoNodeThreadPool struct {
	ForceMerge        *NodesInfoNodeThreadPoolSection `json:"force_merge"`
	FetchShardStarted *NodesInfoNodeThreadPoolSection `json:"fetch_shard_started"`
	Listener          *NodesInfoNodeThreadPoolSection `json:"listener"`
	Index             *NodesInfoNodeThreadPoolSection `json:"index"`
	Refresh           *NodesInfoNodeThreadPoolSection `json:"refresh"`
	Generic           *NodesInfoNodeThreadPoolSection `json:"generic"`
	Warmer            *NodesInfoNodeThreadPoolSection `json:"warmer"`
	Search            *NodesInfoNodeThreadPoolSection `json:"search"`
	Flush             *NodesInfoNodeThreadPoolSection `json:"flush"`
	FetchShardStore   *NodesInfoNodeThreadPoolSection `json:"fetch_shard_store"`
	Management        *NodesInfoNodeThreadPoolSection `json:"management"`
	Get               *NodesInfoNodeThreadPoolSection `json:"get"`
	Bulk              *NodesInfoNodeThreadPoolSection `json:"bulk"`
	Snapshot          *NodesInfoNodeThreadPoolSection `json:"snapshot"`

	Percolate *NodesInfoNodeThreadPoolSection `json:"percolate"` // check
	Bench     *NodesInfoNodeThreadPoolSection `json:"bench"`     // check
	Suggest   *NodesInfoNodeThreadPoolSection `json:"suggest"`   // deprecated
	Optimize  *NodesInfoNodeThreadPoolSection `json:"optimize"`  // deprecated
	Merge     *NodesInfoNodeThreadPoolSection `json:"merge"`     // deprecated
}

// NodesInfoNodeThreadPoolSection represents information about a certain
// type of thread pool, e.g. for indexing or searching.
type NodesInfoNodeThreadPoolSection struct {
	Type      string      `json:"type"`       // e.g. fixed, scaling, or fixed_auto_queue_size
	Min       int         `json:"min"`        // e.g. 4
	Max       int         `json:"max"`        // e.g. 4
	KeepAlive string      `json:"keep_alive"` // e.g. "5m"
	QueueSize interface{} `json:"queue_size"` // e.g. "1k" or -1
}

// NodesInfoNodeTransport represents transport-related information.
type NodesInfoNodeTransport struct {
	BoundAddress   []string                                  `json:"bound_address"`
	PublishAddress string                                    `json:"publish_address"`
	Profiles       map[string]*NodesInfoNodeTransportProfile `json:"profiles"`
}

// NodesInfoNodeTransportProfile represents a transport profile.
type NodesInfoNodeTransportProfile struct {
	BoundAddress   []string `json:"bound_address"`
	PublishAddress string   `json:"publish_address"`
}

// NodesInfoNodeHTTP represents HTTP-related information.
type NodesInfoNodeHTTP struct {
	BoundAddress            []string `json:"bound_address"`      // e.g. ["127.0.0.1:9200", "[fe80::1]:9200", "[::1]:9200"]
	PublishAddress          string   `json:"publish_address"`    // e.g. "127.0.0.1:9300"
	MaxContentLength        string   `json:"max_content_length"` // e.g. "100mb"
	MaxContentLengthInBytes int64    `json:"max_content_length_in_bytes"`
}

// NodesInfoNodePlugin represents information about a plugin.
type NodesInfoNodePlugin struct {
	Name                 string   `json:"name"`    // e.g. "ingest-geoip"
	Version              string   `json:"version"` // e.g. "6.2.3"
	ElasticsearchVersion string   `json:"elasticsearch_version"`
	JavaVersion          string   `json:"java_version"`
	Description          string   `json:"description"` // e.g. "Ingest processor ..."
	Classname            string   `json:"classname"`   // e.g. "org.elasticsearch.ingest.geoip.IngestGeoIpPlugin"
	ExtendedPlugins      []string `json:"extended_plugins"`
	HasNativeController  bool     `json:"has_native_controller"`
	RequiresKeystore     bool     `json:"requires_keystore"`
}

// NodesInfoNodeModule represents information about a module.
type NodesInfoNodeModule struct {
	Name                 string   `json:"name"`    // e.g. "ingest-geoip"
	Version              string   `json:"version"` // e.g. "6.2.3"
	ElasticsearchVersion string   `json:"elasticsearch_version"`
	JavaVersion          string   `json:"java_version"`
	Description          string   `json:"description"` // e.g. "Ingest processor ..."
	Classname            string   `json:"classname"`   // e.g. "org.elasticsearch.ingest.geoip.IngestGeoIpPlugin"
	ExtendedPlugins      []string `json:"extended_plugins"`
	HasNativeController  bool     `json:"has_native_controller"`
	RequiresKeystore     bool     `json:"requires_keystore"`
}

// NodesInfoNodeIngest represents information about the ingester.
type NodesInfoNodeIngest struct {
	Processors []*NodesInfoNodeIngestProcessorInfo `json:"processors"`
}

// NodesInfoNodeIngestProcessorInfo represents ingest processor info.
type NodesInfoNodeIngestProcessorInfo struct {
	Type string `json:"type"` // e.g. append, convert, date etc.
}
