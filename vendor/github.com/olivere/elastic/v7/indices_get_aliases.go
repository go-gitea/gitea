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

// AliasesService returns the aliases associated with one or more indices, or the
// indices associated with one or more aliases, or a combination of those filters.
// See http://www.elastic.co/guide/en/elasticsearch/reference/7.0/indices-aliases.html.
type AliasesService struct {
	client *Client

	pretty     *bool       // pretty format the returned JSON response
	human      *bool       // return human readable values for statistics
	errorTrace *bool       // include the stack trace of returned errors
	filterPath []string    // list of filters used to reduce the response
	headers    http.Header // custom request-level HTTP headers

	index []string
	alias []string
}

// NewAliasesService instantiates a new AliasesService.
func NewAliasesService(client *Client) *AliasesService {
	builder := &AliasesService{
		client: client,
	}
	return builder
}

// Pretty tells Elasticsearch whether to return a formatted JSON response.
func (s *AliasesService) Pretty(pretty bool) *AliasesService {
	s.pretty = &pretty
	return s
}

// Human specifies whether human readable values should be returned in
// the JSON response, e.g. "7.5mb".
func (s *AliasesService) Human(human bool) *AliasesService {
	s.human = &human
	return s
}

// ErrorTrace specifies whether to include the stack trace of returned errors.
func (s *AliasesService) ErrorTrace(errorTrace bool) *AliasesService {
	s.errorTrace = &errorTrace
	return s
}

// FilterPath specifies a list of filters used to reduce the response.
func (s *AliasesService) FilterPath(filterPath ...string) *AliasesService {
	s.filterPath = filterPath
	return s
}

// Header adds a header to the request.
func (s *AliasesService) Header(name string, value string) *AliasesService {
	if s.headers == nil {
		s.headers = http.Header{}
	}
	s.headers.Add(name, value)
	return s
}

// Headers specifies the headers of the request.
func (s *AliasesService) Headers(headers http.Header) *AliasesService {
	s.headers = headers
	return s
}

// Index adds one or more indices.
func (s *AliasesService) Index(index ...string) *AliasesService {
	s.index = append(s.index, index...)
	return s
}

// Alias adds one or more aliases.
func (s *AliasesService) Alias(alias ...string) *AliasesService {
	s.alias = append(s.alias, alias...)
	return s
}

// buildURL builds the URL for the operation.
func (s *AliasesService) buildURL() (string, url.Values, error) {
	var err error
	var path string

	if len(s.index) > 0 {
		path, err = uritemplates.Expand("/{index}/_alias/{alias}", map[string]string{
			"index": strings.Join(s.index, ","),
			"alias": strings.Join(s.alias, ","),
		})
	} else {
		path, err = uritemplates.Expand("/_alias/{alias}", map[string]string{
			"alias": strings.Join(s.alias, ","),
		})
	}
	if err != nil {
		return "", url.Values{}, err
	}
	path = strings.TrimSuffix(path, "/")

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

func (s *AliasesService) Do(ctx context.Context) (*AliasesResult, error) {
	path, params, err := s.buildURL()
	if err != nil {
		return nil, err
	}

	// Get response
	res, err := s.client.PerformRequest(ctx, PerformRequestOptions{
		Method:  "GET",
		Path:    path,
		Params:  params,
		Headers: s.headers,
	})
	if err != nil {
		return nil, err
	}

	// {
	//   "indexName" : {
	//     "aliases" : {
	//       "alias1" : { },
	//       "alias2" : { }
	//     }
	//   },
	//   "indexName2" : {
	//     ...
	//   },
	// }
	indexMap := make(map[string]struct {
		Aliases map[string]struct {
			IsWriteIndex bool `json:"is_write_index"`
		} `json:"aliases"`
	})
	if err := s.client.decoder.Decode(res.Body, &indexMap); err != nil {
		return nil, err
	}

	// Each (indexName, _)
	ret := &AliasesResult{
		Indices: make(map[string]indexResult),
	}
	for indexName, indexData := range indexMap {
		if indexData.Aliases == nil {
			continue
		}

		indexOut, found := ret.Indices[indexName]
		if !found {
			indexOut = indexResult{Aliases: make([]aliasResult, 0)}
		}

		// { "aliases" : { ... } }
		for aliasName, aliasData := range indexData.Aliases {
			aliasRes := aliasResult{AliasName: aliasName, IsWriteIndex: aliasData.IsWriteIndex}
			indexOut.Aliases = append(indexOut.Aliases, aliasRes)
		}

		ret.Indices[indexName] = indexOut
	}

	return ret, nil
}

// -- Result of an alias request.

// AliasesResult is the outcome of calling AliasesService.Do.
type AliasesResult struct {
	Indices map[string]indexResult
}

type indexResult struct {
	Aliases []aliasResult
}

type aliasResult struct {
	AliasName    string
	IsWriteIndex bool
}

// IndicesByAlias returns all indices given a specific alias name.
func (ar AliasesResult) IndicesByAlias(aliasName string) []string {
	var indices []string
	for indexName, indexInfo := range ar.Indices {
		for _, aliasInfo := range indexInfo.Aliases {
			if aliasInfo.AliasName == aliasName {
				indices = append(indices, indexName)
			}
		}
	}
	return indices
}

// HasAlias returns true if the index has a specific alias.
func (ir indexResult) HasAlias(aliasName string) bool {
	for _, alias := range ir.Aliases {
		if alias.AliasName == aliasName {
			return true
		}
	}
	return false
}
