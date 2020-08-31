// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/http"
)

// GetIssueSubscribers get list of users who subscribed on an issue
func (c *Client) GetIssueSubscribers(owner, repo string, index int64) ([]*User, error) {
	if err := c.CheckServerVersionConstraint(">=1.11.0"); err != nil {
		return nil, err
	}
	subscribers := make([]*User, 0, 10)
	return subscribers, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/issues/%d/subscriptions", owner, repo, index), nil, nil, &subscribers)
}

// AddIssueSubscription Subscribe user to issue
func (c *Client) AddIssueSubscription(owner, repo string, index int64, user string) error {
	if err := c.CheckServerVersionConstraint(">=1.11.0"); err != nil {
		return err
	}
	status, err := c.getStatusCode("PUT", fmt.Sprintf("/repos/%s/%s/issues/%d/subscriptions/%s", owner, repo, index, user), nil, nil)
	if err != nil {
		return err
	}
	if status == http.StatusCreated {
		return nil
	}
	if status == http.StatusOK {
		return fmt.Errorf("already subscribed")
	}
	return fmt.Errorf("unexpected Status: %d", status)
}

// DeleteIssueSubscription unsubscribe user from issue
func (c *Client) DeleteIssueSubscription(owner, repo string, index int64, user string) error {
	if err := c.CheckServerVersionConstraint(">=1.11.0"); err != nil {
		return err
	}
	status, err := c.getStatusCode("DELETE", fmt.Sprintf("/repos/%s/%s/issues/%d/subscriptions/%s", owner, repo, index, user), nil, nil)
	if err != nil {
		return err
	}
	if status == http.StatusCreated {
		return nil
	}
	if status == http.StatusOK {
		return fmt.Errorf("already unsubscribed")
	}
	return fmt.Errorf("unexpected Status: %d", status)
}

// CheckIssueSubscription check if current user is subscribed to an issue
func (c *Client) CheckIssueSubscription(owner, repo string, index int64) (*WatchInfo, error) {
	if err := c.CheckServerVersionConstraint(">=1.12.0"); err != nil {
		return nil, err
	}
	wi := new(WatchInfo)
	return wi, c.getParsedResponse("GET", fmt.Sprintf("/repos/%s/%s/issues/%d/subscriptions/check", owner, repo, index), nil, nil, wi)
}

// IssueSubscribe subscribe current user to an issue
func (c *Client) IssueSubscribe(owner, repo string, index int64) error {
	u, err := c.GetMyUserInfo()
	if err != nil {
		return err
	}
	return c.AddIssueSubscription(owner, repo, index, u.UserName)
}

// IssueUnSubscribe unsubscribe current user from an issue
func (c *Client) IssueUnSubscribe(owner, repo string, index int64) error {
	u, err := c.GetMyUserInfo()
	if err != nil {
		return err
	}
	return c.DeleteIssueSubscription(owner, repo, index, u.UserName)
}
