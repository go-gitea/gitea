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

	"github.com/olivere/elastic/v7/uritemplates"
)

// ClusterStatsService is documented at
// https://www.elastic.co/guide/en/elasticsearch/reference/7.0/cluster-stats.html.
type ClusterStatsService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	nodeId       []string
	flatSettings *bool
}

// NewClusterStatsService creates a new ClusterStatsService.
func NewClusterStatsService(client *Client) *ClusterStatsService {
	return &ClusterStatsService{
		client: client,
		nodeId: make([]string, 0),
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *ClusterStatsService) Pretty(pretty bool) *ClusterStatsService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *ClusterStatsService) Human(human bool) *ClusterStatsService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *ClusterStatsService) ErrorTrace(errorTrace bool) *ClusterStatsService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *ClusterStatsService) FilterPath(filterPath ...string) *ClusterStatsService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *ClusterStatsService) Header(name string, value string) *ClusterStatsService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *ClusterStatsService) Headers(headers http.Header) *ClusterStatsService {
	s.headers = headers
	return s
}

// NodeId is documented as: A comma-separated list of node IDs or names to limit the returned information; use `_local` to return information from the node you're connecting to, leave empty to get information from all nodes.
func (s *ClusterStatsService) NodeId(nodeId []string) *ClusterStatsService {
	s.nodeId = nodeId
	return s
}

// FlatSettings is documented as: Return settings in flat format (default: false).
func (s *ClusterStatsService) FlatSettings(flatSettings bool) *ClusterStatsService {
	s.flatSettings = &flatSettings
	return s
}

// buildURL builds the URL for the operation.
func (s *ClusterStatsService) buildURL() (string, url.Values, error) {
	// Build URL
	var err error
	var path string

	if len(s.nodeId) > 0 {
		path, err = uritemplates.Expand("/_cluster/stats/nodes/{node_id}", map[string]string{
			"node_id": strings.Join(s.nodeId, ","),
		})
		if err != nil {
			return "", url.Values{}, err
		}
	} else {
		path, err = uritemplates.Expand("/_cluster/stats", map[string]string{})
		if err != nil {
			return "", url.Values{}, err
		}
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
func (s *ClusterStatsService) Validate() error {
	return nil
}

// Do executes the operation.
func (s *ClusterStatsService) Do(ctx context.Context) (*ClusterStatsResponse, error) {
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
	ret := new(ClusterStatsResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// ClusterStatsResponse is the response of ClusterStatsService.Do.
type ClusterStatsResponse struct {
	NodesStats  *ShardsInfo          `json:"_nodes,omitempty"`
	Timestamp   int64                `json:"timestamp"`
	ClusterName string               `json:"cluster_name"`
	ClusterUUID string               `json:"cluster_uuid"`
	Status      string               `json:"status,omitempty"` // e.g. green
	Indices     *ClusterStatsIndices `json:"indices"`
	Nodes       *ClusterStatsNodes   `json:"nodes"`
}

type ClusterStatsIndices struct {
	Count      int                            `json:"count"` // number of indices
	Shards     *ClusterStatsIndicesShards     `json:"shards"`
	Docs       *ClusterStatsIndicesDocs       `json:"docs"`
	Store      *ClusterStatsIndicesStore      `json:"store"`
	FieldData  *ClusterStatsIndicesFieldData  `json:"fielddata"`
	QueryCache *ClusterStatsIndicesQueryCache `json:"query_cache"`
	Completion *ClusterStatsIndicesCompletion `json:"completion"`
	Segments   *IndexStatsSegments            `json:"segments"`
}

type ClusterStatsIndicesShards struct {
	Total       int                             `json:"total"`
	Primaries   int                             `json:"primaries"`
	Replication float64                         `json:"replication"`
	Index       *ClusterStatsIndicesShardsIndex `json:"index"`
}

type ClusterStatsIndicesShardsIndex struct {
	Shards      *ClusterStatsIndicesShardsIndexIntMinMax     `json:"shards"`
	Primaries   *ClusterStatsIndicesShardsIndexIntMinMax     `json:"primaries"`
	Replication *ClusterStatsIndicesShardsIndexFloat64MinMax `json:"replication"`
}

type ClusterStatsIndicesShardsIndexIntMinMax struct {
	Min int     `json:"min"`
	Max int     `json:"max"`
	Avg float64 `json:"avg"`
}

type ClusterStatsIndicesShardsIndexFloat64MinMax struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
	Avg float64 `json:"avg"`
}

type ClusterStatsIndicesDocs struct {
	Count   int `json:"count"`
	Deleted int `json:"deleted"`
}

type ClusterStatsIndicesStore struct {
	Size        string `json:"size"` // e.g. "5.3gb"
	SizeInBytes int64  `json:"size_in_bytes"`
}

type ClusterStatsIndicesFieldData struct {
	MemorySize        string `json:"memory_size"` // e.g. "61.3kb"
	MemorySizeInBytes int64  `json:"memory_size_in_bytes"`
	Evictions         int64  `json:"evictions"`
	Fields            map[string]struct {
		MemorySize        string `json:"memory_size"` // e.g. "61.3kb"
		MemorySizeInBytes int64  `json:"memory_size_in_bytes"`
	} `json:"fields,omitempty"`
}

type ClusterStatsIndicesQueryCache struct {
	MemorySize        string `json:"memory_size"` // e.g. "61.3kb"
	MemorySizeInBytes int64  `json:"memory_size_in_bytes"`
	TotalCount        int64  `json:"total_count"`
	HitCount          int64  `json:"hit_count"`
	MissCount         int64  `json:"miss_count"`
	CacheSize         int64  `json:"cache_size"`
	CacheCount        int64  `json:"cache_count"`
	Evictions         int64  `json:"evictions"`
}

type ClusterStatsIndicesCompletion struct {
	Size        string `json:"size"` // e.g. "61.3kb"
	SizeInBytes int64  `json:"size_in_bytes"`
	Fields      map[string]struct {
		Size        string `json:"size"` // e.g. "61.3kb"
		SizeInBytes int64  `json:"size_in_bytes"`
	} `json:"fields,omitempty"`
}

type ClusterStatsIndicesSegmentsFile struct {
	Size        string `json:"size"` // e.g. "61.3kb"
	SizeInBytes int64  `json:"size_in_bytes"`
	Description string `json:"description,omitempty"`
}

// ---

type ClusterStatsNodes struct {
	Count    *ClusterStatsNodesCount        `json:"count"`
	Versions []string                       `json:"versions"`
	OS       *ClusterStatsNodesOsStats      `json:"os"`
	Process  *ClusterStatsNodesProcessStats `json:"process"`
	JVM      *ClusterStatsNodesJvmStats     `json:"jvm"`
	FS       *ClusterStatsNodesFsStats      `json:"fs"`
	Plugins  []*ClusterStatsNodesPlugin     `json:"plugins"`

	NetworkTypes   *ClusterStatsNodesNetworkTypes   `json:"network_types"`
	DiscoveryTypes *ClusterStatsNodesDiscoveryTypes `json:"discovery_types"`
	PackagingTypes *ClusterStatsNodesPackagingTypes `json:"packaging_types"`
}

type ClusterStatsNodesCount struct {
	Total            int `json:"total"`
	Data             int `json:"data"`
	CoordinatingOnly int `json:"coordinating_only"`
	Master           int `json:"master"`
	Ingest           int `json:"ingest"`
}

type ClusterStatsNodesOsStats struct {
	AvailableProcessors int `json:"available_processors"`
	AllocatedProcessors int `json:"allocated_processors"`
	Names               []struct {
		Name  string `json:"name"`
		Value int    `json:"count"`
	} `json:"names"`
	PrettyNames []struct {
		PrettyName string `json:"pretty_name"`
		Value      int    `json:"count"`
	} `json:"pretty_names"`
	Mem *ClusterStatsNodesOsStatsMem `json:"mem"`
	// CPU []*ClusterStatsNodesOsStatsCPU `json:"cpu"`
}

type ClusterStatsNodesOsStatsMem struct {
	Total        string `json:"total"` // e.g. "16gb"
	TotalInBytes int64  `json:"total_in_bytes"`
	Free         string `json:"free"` // e.g. "12gb"
	FreeInBytes  int64  `json:"free_in_bytes"`
	Used         string `json:"used"` // e.g. "4gb"
	UsedInBytes  int64  `json:"used_in_bytes"`
	FreePercent  int    `json:"free_percent"`
	UsedPercent  int    `json:"used_percent"`
}

type ClusterStatsNodesOsStatsCPU struct {
	Vendor           string `json:"vendor"`
	Model            string `json:"model"`
	MHz              int    `json:"mhz"`
	TotalCores       int    `json:"total_cores"`
	TotalSockets     int    `json:"total_sockets"`
	CoresPerSocket   int    `json:"cores_per_socket"`
	CacheSize        string `json:"cache_size"` // e.g. "256b"
	CacheSizeInBytes int64  `json:"cache_size_in_bytes"`
	Count            int    `json:"count"`
}

type ClusterStatsNodesProcessStats struct {
	CPU                 *ClusterStatsNodesProcessStatsCPU                 `json:"cpu"`
	OpenFileDescriptors *ClusterStatsNodesProcessStatsOpenFileDescriptors `json:"open_file_descriptors"`
}

type ClusterStatsNodesProcessStatsCPU struct {
	Percent float64 `json:"percent"`
}

type ClusterStatsNodesProcessStatsOpenFileDescriptors struct {
	Min int64 `json:"min"`
	Max int64 `json:"max"`
	Avg int64 `json:"avg"`
}

type ClusterStatsNodesJvmStats struct {
	MaxUptime         string                              `json:"max_uptime"` // e.g. "5h"
	MaxUptimeInMillis int64                               `json:"max_uptime_in_millis"`
	Versions          []*ClusterStatsNodesJvmStatsVersion `json:"versions"`
	Mem               *ClusterStatsNodesJvmStatsMem       `json:"mem"`
	Threads           int64                               `json:"threads"`
}

type ClusterStatsNodesJvmStatsVersion struct {
	Version         string `json:"version"`    // e.g. "1.8.0_45"
	VMName          string `json:"vm_name"`    // e.g. "Java HotSpot(TM) 64-Bit Server VM"
	VMVersion       string `json:"vm_version"` // e.g. "25.45-b02"
	VMVendor        string `json:"vm_vendor"`  // e.g. "Oracle Corporation"
	BundledJDK      bool   `json:"bundled_jdk"`
	UsingBundledJDK bool   `json:"using_bundled_jdk"`
	Count           int    `json:"count"`
}

type ClusterStatsNodesJvmStatsMem struct {
	HeapUsed        string `json:"heap_used"`
	HeapUsedInBytes int64  `json:"heap_used_in_bytes"`
	HeapMax         string `json:"heap_max"`
	HeapMaxInBytes  int64  `json:"heap_max_in_bytes"`
}

type ClusterStatsNodesFsStats struct {
	Path                 string `json:"path"`
	Mount                string `json:"mount"`
	Dev                  string `json:"dev"`
	Total                string `json:"total"` // e.g. "930.7gb"`
	TotalInBytes         int64  `json:"total_in_bytes"`
	Free                 string `json:"free"` // e.g. "930.7gb"`
	FreeInBytes          int64  `json:"free_in_bytes"`
	Available            string `json:"available"` // e.g. "930.7gb"`
	AvailableInBytes     int64  `json:"available_in_bytes"`
	DiskReads            int64  `json:"disk_reads"`
	DiskWrites           int64  `json:"disk_writes"`
	DiskIOOp             int64  `json:"disk_io_op"`
	DiskReadSize         string `json:"disk_read_size"` // e.g. "0b"`
	DiskReadSizeInBytes  int64  `json:"disk_read_size_in_bytes"`
	DiskWriteSize        string `json:"disk_write_size"` // e.g. "0b"`
	DiskWriteSizeInBytes int64  `json:"disk_write_size_in_bytes"`
	DiskIOSize           string `json:"disk_io_size"` // e.g. "0b"`
	DiskIOSizeInBytes    int64  `json:"disk_io_size_in_bytes"`
	DiskQueue            string `json:"disk_queue"`
	DiskServiceTime      string `json:"disk_service_time"`
}

type ClusterStatsNodesPlugin struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	URL         string `json:"url"`
	JVM         bool   `json:"jvm"`
	Site        bool   `json:"site"`
}

type ClusterStatsNodesNetworkTypes struct {
	TransportTypes map[string]interface{} `json:"transport_types"` // e.g. "netty4": 1
	HTTPTypes      map[string]interface{} `json:"http_types"`      // e.g. "netty4": 1
}

type ClusterStatsNodesDiscoveryTypes interface{}

type ClusterStatsNodesPackagingTypes []*ClusterStatsNodesPackagingType

type ClusterStatsNodesPackagingType struct {
	Flavor string `json:"flavor"` // e.g. "oss"
	Type   string `json:"type"`   // e.g. "docker"
	Count  int    `json:"count"`  // e.g. 1
}
