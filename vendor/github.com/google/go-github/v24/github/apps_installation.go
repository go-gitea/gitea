// Copyright 2016 The go-github AUTHORS. All rights reserved.
//
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package github

import (
	"context"
	"fmt"
)

// ListRepos lists the repositories that are accessible to the authenticated installation.
//
// GitHub API docs: https://developer.github.com/v3/apps/installations/#list-repositories
func (s *AppsService) ListRepos(ctx context.Context, opt *ListOptions) ([]*Repository, *Response, error) {
	u, err := addOptions("installation/repositories", opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	req.Header.Set("Accept", mediaTypeIntegrationPreview)

	var r struct {
		Repositories []*Repository `json:"repositories"`
	}
	resp, err := s.client.Do(ctx, req, &r)
	if err != nil {
		return nil, resp, err
	}

	return r.Repositories, resp, nil
}

// ListUserRepos lists repositories that are accessible
// to the authenticated user for an installation.
//
// GitHub API docs: https://developer.github.com/v3/apps/installations/#list-repositories-accessible-to-the-user-for-an-installation
func (s *AppsService) ListUserRepos(ctx context.Context, id int64, opt *ListOptions) ([]*Repository, *Response, error) {
	u := fmt.Sprintf("user/installations/%v/repositories", id)
	u, err := addOptions(u, opt)
	if err != nil {
		return nil, nil, err
	}

	req, err := s.client.NewRequest("GET", u, nil)
	if err != nil {
		return nil, nil, err
	}

	// TODO: remove custom Accept header when this API fully launches.
	req.Header.Set("Accept", mediaTypeIntegrationPreview)

	var r struct {
		Repositories []*Repository `json:"repositories"`
	}
	resp, err := s.client.Do(ctx, req, &r)
	if err != nil {
		return nil, resp, err
	}

	return r.Repositories, resp, nil
}

// AddRepository adds a single repository to an installation.
//
// GitHub API docs: https://developer.github.com/v3/apps/installations/#add-repository-to-installation
func (s *AppsService) AddRepository(ctx context.Context, instID, repoID int64) (*Repository, *Response, error) {
	u := fmt.Sprintf("user/installations/%v/repositories/%v", instID, repoID)
	req, err := s.client.NewRequest("PUT", u, nil)
	if err != nil {
		return nil, nil, err
	}
	req.Header.Set("Accept", mediaTypeIntegrationPreview)

	r := new(Repository)
	resp, err := s.client.Do(ctx, req, r)
	if err != nil {
		return nil, resp, err
	}

	return r, resp, nil
}

// RemoveRepository removes a single repository from an installation.
//
// GitHub docs: https://developer.github.com/v3/apps/installations/#remove-repository-from-installation
func (s *AppsService) RemoveRepository(ctx context.Context, instID, repoID int64) (*Response, error) {
	u := fmt.Sprintf("user/installations/%v/repositories/%v", instID, repoID)
	req, err := s.client.NewRequest("DELETE", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", mediaTypeIntegrationPreview)

	return s.client.Do(ctx, req, nil)
}
