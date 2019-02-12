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
func (c *Client) GetRepoRef(user, repo, ref string) (*Reference, error) {
	ref = strings.TrimPrefix(ref, "refs/")
	r := new(Reference)
	err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/git/refs/%s", user, repo, ref), nil, nil, &r)
	if _, ok := err.(*json.UnmarshalTypeError); ok {
		// Multiple refs
		return nil, errors.New("no exact match found for this ref")
	} else if err != nil {
		return nil, err
	}

	return r, nil
}

// GetRepoRefs get list of ref's information of one repository
func (c *Client) GetRepoRefs(user, repo, ref string) ([]*Reference, error) {
	ref = strings.TrimPrefix(ref, "refs/")
	resp, err := c.getResponse("GET", fmt.Sprintf("/repos/%s/%s/git/refs/%s", user, repo, ref), nil, nil)
	if err != nil {
		return nil, err
	}

	// Attempt to unmarshal single returned ref.
	r := new(Reference)
	refErr := json.Unmarshal(resp, r)
	if refErr == nil {
		return []*Reference{r}, nil
	}

	// Attempt to unmarshal multiple refs.
	var rs []*Reference
	refsErr := json.Unmarshal(resp, &rs)
	if refsErr == nil {
		if len(rs) == 0 {
			return nil, errors.New("unexpected response: an array of refs with length 0")
		}
		return rs, nil
	}

	return nil, fmt.Errorf("unmarshalling failed for both single and multiple refs: %s and %s", refErr, refsErr)
}
