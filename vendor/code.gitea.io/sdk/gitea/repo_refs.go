// Copyright 2018 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// Reference represents a Git reference.
type Reference struct {
	Ref    string     `json:"ref"`
	URL    string     `json:"url"`
	Object *GitObject `json:"object"`
}

// GitObject represents a Git object.
type GitObject struct {
	Type string `json:"type"`
	SHA  string `json:"sha"`
	URL  string `json:"url"`
}

// GetRepoRef get one ref's information of one repository
func (c *Client) GetRepoRef(user, repo, ref string) (*Reference, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	ref = strings.TrimPrefix(ref, "refs/")
	ref = pathEscapeSegments(ref)
	r := new(Reference)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/git/refs/%s", user, repo, ref), nil, nil, &r)
	if _, ok := err.(*json.UnmarshalTypeError); ok {
		// Multiple refs
		return nil, resp, errors.New("no exact match found for this ref")
	} else if err != nil {
		return nil, resp, err
	}

	return r, resp, nil
}

// GetRepoRefs get list of ref's information of one repository
func (c *Client) GetRepoRefs(user, repo, ref string) ([]*Reference, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	ref = strings.TrimPrefix(ref, "refs/")
	ref = pathEscapeSegments(ref)

	data, resp, err := c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/git/refs/%s", user, repo, ref), nil, nil)
	if err != nil {
		return nil, resp, err
	}

	// Attempt to unmarshal single returned ref.
	r := new(Reference)
	refErr := json.Unmarshal(data, r)
	if refErr == nil {
		return []*Reference{r}, resp, nil
	}

	// Attempt to unmarshal multiple refs.
	var rs []*Reference
	refsErr := json.Unmarshal(data, &rs)
	if refsErr == nil {
		if len(rs) == 0 {
			return nil, resp, errors.New("unexpected response: an array of refs with length 0")
		}
		return rs, resp, nil
	}

	return nil, resp, fmt.Errorf("unmarshalling failed for both single and multiple refs: %s and %s", refErr, refsErr)
}
