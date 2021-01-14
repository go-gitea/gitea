// Copyright 2012-2018 Oliver Eilhard. All rights reserved.
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

// XPackInfoService retrieves xpack info.
// See https://www.elastic.co/guide/en/elasticsearch/reference/7.0/info-api.html.
type XPackInfoService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers
}

// NewXPackInfoService creates a new XPackInfoService.
func NewXPackInfoService(client *Client) *XPackInfoService {
	return &XPackInfoService{
		client: client,
	}
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *XPackInfoService) Pretty(pretty bool) *XPackInfoService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *XPackInfoService) Human(human bool) *XPackInfoService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *XPackInfoService) ErrorTrace(errorTrace bool) *XPackInfoService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *XPackInfoService) FilterPath(filterPath ...string) *XPackInfoService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *XPackInfoService) Header(name string, value string) *XPackInfoService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *XPackInfoService) Headers(headers http.Header) *XPackInfoService {
	s.headers = headers
	return s
}

// buildURL builds the URL for the operation.
func (s *XPackInfoService) buildURL() (string, url.Values, error) {
	// Build URL
	path, err := uritemplates.Expand("/_xpack", map[string]string{})
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
	return path, params, nil
}

// Validate checks if the operation is valid.
func (s *XPackInfoService) Validate() error {
	var invalid []string
	if len(invalid) > 0 {
		return fmt.Errorf("missing required fields: %v", invalid)
	}
	return nil
}

// Do executes the operation.
func (s *XPackInfoService) Do(ctx context.Context) (*XPackInfoServiceResponse, error) {
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
	ret := XPackInfoServiceResponse{}
	if err := json.Unmarshal(res.Body, &ret); err != nil {
		return nil, err
	}
	return &ret, nil
}

// XPackInfoServiceResponse is the response of XPackInfoService.Do.
type XPackInfoServiceResponse struct {
	Build    XPackInfoBuild    `json:"build"`
	License  XPackInfoLicense  `json:"license"`
	Features XPackInfoFeatures `json:"features"`
	Tagline  string            `json:"tagline"`
}

// XPackInfoBuild is the xpack build info
type XPackInfoBuild struct {
	Hash string `json:"hash"`
	Date string `json:"date"`
}

// XPackInfoLicense is the xpack license info
type XPackInfoLicense struct {
	UID         string `json:"uid"`
	Type        string `json:"type"`
	Mode        string `json:"mode"`
	Status      string `json:"status"`
	ExpiryMilis int    `json:"expiry_date_in_millis"`
}

// XPackInfoFeatures is the xpack feature info object
type XPackInfoFeatures struct {
	Graph           XPackInfoGraph      `json:"graph"`
	Logstash        XPackInfoLogstash   `json:"logstash"`
	MachineLearning XPackInfoML         `json:"ml"`
	Monitoring      XPackInfoMonitoring `json:"monitoring"`
	Rollup          XPackInfoRollup     `json:"rollup"`
	Security        XPackInfoSecurity   `json:"security"`
	Watcher         XPackInfoWatcher    `json:"watcher"`
}

// XPackInfoGraph is the xpack graph plugin info
type XPackInfoGraph struct {
	Description string `json:"description"`
	Available   bool   `json:"available"`
	Enabled     bool   `json:"enabled"`
}

// XPackInfoLogstash is the xpack logstash plugin info
type XPackInfoLogstash struct {
	Description string `json:"description"`
	Available   bool   `json:"available"`
	Enabled     bool   `json:"enabled"`
}

// XPackInfoML is the xpack machine learning plugin info
type XPackInfoML struct {
	Description    string            `json:"description"`
	Available      bool              `json:"available"`
	Enabled        bool              `json:"enabled"`
	NativeCodeInfo map[string]string `json:"native_code_info"`
}

// XPackInfoMonitoring is the xpack monitoring plugin info
type XPackInfoMonitoring struct {
	Description string `json:"description"`
	Available   bool   `json:"available"`
	Enabled     bool   `json:"enabled"`
}

// XPackInfoRollup is the xpack rollup plugin info
type XPackInfoRollup struct {
	Description string `json:"description"`
	Available   bool   `json:"available"`
	Enabled     bool   `json:"enabled"`
}

// XPackInfoSecurity is the xpack security plugin info
type XPackInfoSecurity struct {
	Description string `json:"description"`
	Available   bool   `json:"available"`
	Enabled     bool   `json:"enabled"`
}

// XPackInfoWatcher is the xpack watcher plugin info
type XPackInfoWatcher struct {
	Description string `json:"description"`
	Available   bool   `json:"available"`
	Enabled     bool   `json:"enabled"`
}
