// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/http"
)

// GetIssueSubscribers get list of users who subscribed on an issue
func (c *Client) GetIssueSubscribers(owner, repo string, index int64) ([]*User, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	subscribers := make([]*User, 0, 10)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/issues/%d/subscriptions", owner, repo, index), nil, nil, &subscribers)
	return subscribers, resp, err
}

// AddIssueSubscription Subscribe user to issue
func (c *Client) AddIssueSubscription(owner, repo string, index int64, user string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &user); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("PUT", fmt.Sprintf("/repos/%s/%s/issues/%d/subscriptions/%s", owner, repo, index, user), nil, nil)
	if err != nil {
		return resp, err
	}
	if status == http.StatusCreated {
		return resp, nil
	}
	if status == http.StatusOK {
		return resp, fmt.Errorf("already subscribed")
	}
	return resp, fmt.Errorf("unexpected Status: %d", status)
}

// DeleteIssueSubscription unsubscribe user from issue
func (c *Client) DeleteIssueSubscription(owner, repo string, index int64, user string) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo, &user); err != nil {
		return nil, err
	}
	status, resp, err := c.getStatusCode("DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d/subscriptions/%s", owner, repo, index, user), nil, nil)
	if err != nil {
		return resp, err
	}
	if status == http.StatusCreated {
		return resp, nil
	}
	if status == http.StatusOK {
		return resp, fmt.Errorf("already unsubscribed")
	}
	return resp, fmt.Errorf("unexpected Status: %d", status)
}

// CheckIssueSubscription check if current user is subscribed to an issue
func (c *Client) CheckIssueSubscription(owner, repo string, index int64) (*WatchInfo, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
		return nil, nil, err
	}
	wi := new(WatchInfo)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/issues/%d/subscriptions/check", owner, repo, index), nil, nil, wi)
	return wi, resp, err
}

// IssueSubscribe subscribe current user to an issue
func (c *Client) IssueSubscribe(owner, repo string, index int64) (*Response, error) {
	u, _, err := c.GetMyUserInfo()
	if err != nil {
		return nil, err
	}
	return c.AddIssueSubscription(owner, repo, index, u.UserName)
}

// IssueUnSubscribe unsubscribe current user from an issue
func (c *Client) IssueUnSubscribe(owner, repo string, index int64) (*Response, error) {
	u, _, err := c.GetMyUserInfo()
	if err != nil {
		return nil, err
	}
	return c.DeleteIssueSubscription(owner, repo, index, u.UserName)
}
