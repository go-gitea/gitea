// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"bytes"
	"encoding/json"
	"fmt"
)

// Tag represents a repository tag
type Tag struct {
	Name       string      `json:"name"`
	Message    string      `json:"message"`
	ID         string      `json:"id"`
	Commit     *CommitMeta `json:"commit"`
	ZipballURL string      `json:"zipball_url"`
	TarballURL string      `json:"tarball_url"`
}

// AnnotatedTag represents an annotated tag
type AnnotatedTag struct {
	Tag          string                     `json:"tag"`
	SHA          string                     `json:"sha"`
	URL          string                     `json:"url"`
	Message      string                     `json:"message"`
	Tagger       *CommitUser                `json:"tagger"`
	Object       *AnnotatedTagObject        `json:"object"`
	Verification *PayloadCommitVerification `json:"verification"`
}

// AnnotatedTagObject contains meta information of the tag object
type AnnotatedTagObject struct {
	Type string `json:"type"`
	URL  string `json:"url"`
	SHA  string `json:"sha"`
}

// ListRepoTagsOptions options for listing a repository's tags
type ListRepoTagsOptions struct {
	ListOptions
}

// ListRepoTags list all the branches of one repository
func (c *Client) ListRepoTags(user, repo string, opt ListRepoTagsOptions) ([]*Tag, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	tags := make([]*Tag, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/tags?%s", user, repo, opt.getURLQuery().Encode()), nil, nil, &tags)
	return tags, resp, err
}

// GetTag get the tag of a repository
func (c *Client) GetTag(user, repo, tag string) (*Tag, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_15_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&user, &repo, &tag); err != nil {
		return nil, nil, err
	}
	t := new(Tag)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/tags/%s", user, repo, tag), nil, nil, &t)
	return t, resp, err
}

// GetAnnotatedTag get the tag object of an annotated tag (not lightweight tags) of a repository
func (c *Client) GetAnnotatedTag(user, repo, sha string) (*AnnotatedTag, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_15_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&user, &repo, &sha); err != nil {
		return nil, nil, err
	}
	t := new(AnnotatedTag)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/git/tags/%s", user, repo, sha), nil, nil, &t)
	return t, resp, err
}

// CreateTagOption options when creating a tag
type CreateTagOption struct {
	TagName string `json:"tag_name"`
	Message string `json:"message"`
	Target  string `json:"target"`
}

// Validate validates CreateTagOption
func (opt CreateTagOption) Validate() error {
	if len(opt.TagName) == 0 {
		return fmt.Errorf("TagName is required")
	}
	return nil
}

// CreateTag create a new git tag in a repository
func (c *Client) CreateTag(user, repo string, opt CreateTagOption) (*Tag, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_15_0); err != nil {
		return nil, nil, err
	}
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(opt)
	if err != nil {
		return nil, nil, err
	}
	t := new(Tag)
	resp, err := c.getParsedResponse("POST", fmt.Sprintf("/repos/%s/%s/tags", user, repo), jsonHeader, bytes.NewReader(body), &t)
	return t, resp, err
}

// DeleteTag deletes a tag from a repository, if no release refers to it
func (c *Client) DeleteTag(user, repo, tag string) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo, &tag); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_14_0); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE",
		fmt.Sprintf("/repos/%s/%s/tags/%s", user, repo, tag),
		nil, nil)
	return resp, err
}
