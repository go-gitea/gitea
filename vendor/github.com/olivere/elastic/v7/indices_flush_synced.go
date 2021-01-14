// Copyright 2012-present Oliver Eilhard. All rights reserved.
// Use of this source code is governed by a MIT-license.
// See http://olivere.mit-license.org/license.txt for details.

package elastic

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/olivere/elastic/v7/uritemplates"
)

// IndicesSyncedFlushService performs a normal flush, then adds a generated
// unique marked (sync_id) to all shards.
//
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-synced-flush.html
// for details.
type IndicesSyncedFlushService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index             []string
	ignoreUnavailable *bool
	allowNoIndices    *bool
	expandWildcards   string
}

// NewIndicesSyncedFlushService creates a new IndicesSyncedFlushService.
func NewIndicesSyncedFlushService(client *Client) *IndicesSyncedFlushService {
	return &IndicesSyncedFlushService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *IndicesSyncedFlushService) Pretty(pretty bool) *IndicesSyncedFlushService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *IndicesSyncedFlushService) Human(human bool) *IndicesSyncedFlushService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *IndicesSyncedFlushService) ErrorTrace(errorTrace bool) *IndicesSyncedFlushService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *IndicesSyncedFlushService) FilterPath(filterPath ...string) *IndicesSyncedFlushService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *IndicesSyncedFlushService) Header(name string, value string) *IndicesSyncedFlushService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *IndicesSyncedFlushService) Headers(headers http.Header) *IndicesSyncedFlushService {
	s.headers = headers
	return s
}

// Index is a list of index names; use `_all` or empty string for all indices.
func (s *IndicesSyncedFlushService) Index(indices ...string) *IndicesSyncedFlushService {
	s.index = append(s.index, indices...)
	return s
}

// IgnoreUnavailable indicates whether specified concrete indices should be
// ignored when unavailable (missing or closed).
func (s *IndicesSyncedFlushService) IgnoreUnavailable(ignoreUnavailable bool) *IndicesSyncedFlushService {
	s.ignoreUnavailable = &ignoreUnavailable
	return s
}

// AllowNoIndices indicates whether to ignore if a wildcard indices expression
// resolves into no concrete indices. (This includes `_all` string or when
// no indices have been specified).
func (s *IndicesSyncedFlushService) AllowNoIndices(allowNoIndices bool) *IndicesSyncedFlushService {
	s.allowNoIndices = &allowNoIndices
	return s
}

// ExpandWildcards specifies whether to expand wildcard expression to
// concrete indices that are open, closed or both..
func (s *IndicesSyncedFlushService) ExpandWildcards(expandWildcards string) *IndicesSyncedFlushService {
	s.expandWildcards = expandWildcards
	return s
}

// buildURL builds the URL for the operation.
func (s *IndicesSyncedFlushService) buildURL() (string, url.Values, error) {
	// Build URL
	var err error
	var path string

	if len(s.index) > 0 {
		path, err = uritemplates.Expand("/{index}/_flush/synced", map[string]string{
			"index": strings.Join(s.index, ","),
		})
	} else {
		path = "/_flush/synced"
	}
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
	if s.ignoreUnavailable != nil {
		params.Set("ignore_unavailable", fmt.Sprintf("%v", *s.ignoreUnavailable))
	}
	if s.allowNoIndices != nil {
		params.Set("allow_no_indices", fmt.Sprintf("%v", *s.allowNoIndices))
	}
	if s.expandWildcards != "" {
		params.Set("expand_wildcards", s.expandWildcards)
	}
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *IndicesSyncedFlushService) Validate() error {
	return nil
}

// Do executes the service.
//
// Deprecated: Synced flush is deprecated and will be removed in 8.0.
// Use flush at _/flush or /{index}/_flush instead.
func (s *IndicesSyncedFlushService) Do(ctx context.Context) (*IndicesSyncedFlushResponse, error) {
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
		Method:  "POST",
		Path:    path,
		Params:  params,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// Return operation response
	ret := new(IndicesSyncedFlushResponse)
	if err := s.client.decoder.Decode(res.Body, ret); err != nil {
		return nil, err
	}
	return ret, nil
}

// -- Result of a flush request.

// IndicesSyncedFlushResponse is the outcome of a synched flush call.
type IndicesSyncedFlushResponse struct {
	Shards *ShardsInfo                                `json:"_shards"`
	Index  map[string]*IndicesShardsSyncedFlushResult `json:"-"`

	// TODO Add information about the indices here from the root level
	// It looks like this:
	// {
	// 	"_shards" : {
	// 	  "total" : 4,
	// 	  "successful" : 4,
	// 	  "failed" : 0
	// 	},
	// 	"elastic-test" : {
	// 	  "total" : 1,
	// 	  "successful" : 1,
	// 	  "failed" : 0
	// 	},
	// 	"elastic-test2" : {
	// 	  "total" : 1,
	// 	  "successful" : 1,
	// 	  "failed" : 0
	// 	},
	// 	"elastic-orders" : {
	// 	  "total" : 1,
	// 	  "successful" : 1,
	// 	  "failed" : 0
	// 	},
	// 	"elastic-nosource-test" : {
	// 	  "total" : 1,
	// 	  "successful" : 1,
	// 	  "failed" : 0
	// 	}
	// }
}

// IndicesShardsSyncedFlushResult represents synced flush information about
// a specific index.
type IndicesShardsSyncedFlushResult struct {
	Total      int                                     `json:"total"`
	Successful int                                     `json:"successful"`
	Failed     int                                     `json:"failed"`
	Failures   []IndicesShardsSyncedFlushResultFailure `json:"failures,omitempty"`
}

// IndicesShardsSyncedFlushResultFailure represents a failure of a synced
// flush operation.
type IndicesShardsSyncedFlushResultFailure struct {
	Shard   int    `json:"shard"`
	Reason  string `json:"reason"`
	Routing struct {
		State                    string  `json:"state"`
		Primary                  bool    `json:"primary"`
		Node                     string  `json:"node"`
		RelocatingNode           *string `json:"relocating_node"`
		Shard                    int     `json:"shard"`
		Index                    string  `json:"index"`
		ExpectedShardSizeInBytes int64   `json:"expected_shard_size_in_bytes,omitempty"`
		// recoverySource
		// allocationId
		// unassignedInfo
	} `json:"routing"`
}

// UnmarshalJSON parses the output from Synced Flush API.
func (resp *IndicesSyncedFlushResponse) UnmarshalJSON(data []byte) error {
	m := make(map[string]json.RawMessage)
	err := json.Unmarshal(data, &m)
	if err != nil {
		return err
	}
	resp.Index = make(map[string]*IndicesShardsSyncedFlushResult)
	for k, v := range m {
		if k == "_shards" {
			if err := json.Unmarshal(v, &resp.Shards); err != nil {
				return err
			}
		} else {
			ix := new(IndicesShardsSyncedFlushResult)
			if err := json.Unmarshal(v, &ix); err != nil {
				return err
			}
			resp.Index[k] = ix
		}
	}
	return nil
}
