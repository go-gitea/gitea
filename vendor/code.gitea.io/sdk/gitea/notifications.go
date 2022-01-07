// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea

import (
	"fmt"
	"net/url"
	"time"

	"github.com/hashicorp/go-version"
)

var (
	version1_12_3, _ = version.NewVersion("1.12.3")
)

// NotificationThread expose Notification on API
type NotificationThread struct {
	ID         int64                `json:"id"`
	Repository *Repository          `json:"repository"`
	Subject    *NotificationSubject `json:"subject"`
	Unread     bool                 `json:"unread"`
	Pinned     bool                 `json:"pinned"`
	UpdatedAt  time.Time            `json:"updated_at"`
	URL        string               `json:"url"`
}

// NotificationSubject contains the notification subject (Issue/Pull/Commit)
type NotificationSubject struct {
	Title            string             `json:"title"`
	URL              string             `json:"url"`
	LatestCommentURL string             `json:"latest_comment_url"`
	Type             NotifySubjectType  `json:"type"`
	State            NotifySubjectState `json:"state"`
}

// NotifyStatus notification status type
type NotifyStatus string

const (
	// NotifyStatusUnread was not read
	NotifyStatusUnread NotifyStatus = "unread"
	// NotifyStatusRead was already read by user
	NotifyStatusRead NotifyStatus = "read"
	// NotifyStatusPinned notification is pinned by user
	NotifyStatusPinned NotifyStatus = "pinned"
)

// NotifySubjectType represent type of notification subject
type NotifySubjectType string

const (
	// NotifySubjectIssue an issue is subject of an notification
	NotifySubjectIssue NotifySubjectType = "Issue"
	// NotifySubjectPull an pull is subject of an notification
	NotifySubjectPull NotifySubjectType = "Pull"
	// NotifySubjectCommit an commit is subject of an notification
	NotifySubjectCommit NotifySubjectType = "Commit"
	// NotifySubjectRepository an repository is subject of an notification
	NotifySubjectRepository NotifySubjectType = "Repository"
)

// NotifySubjectState reflect state of notification subject
type NotifySubjectState string

const (
	// NotifySubjectOpen if subject is a pull/issue and is open at the moment
	NotifySubjectOpen NotifySubjectState = "open"
	// NotifySubjectClosed if subject is a pull/issue and is closed at the moment
	NotifySubjectClosed NotifySubjectState = "closed"
	// NotifySubjectMerged if subject is a pull and got merged
	NotifySubjectMerged NotifySubjectState = "merged"
)

// ListNotificationOptions represents the filter options
type ListNotificationOptions struct {
	ListOptions
	Since        time.Time
	Before       time.Time
	Status       []NotifyStatus
	SubjectTypes []NotifySubjectType
}

// MarkNotificationOptions represents the filter & modify options
type MarkNotificationOptions struct {
	LastReadAt time.Time
	Status     []NotifyStatus
	ToStatus   NotifyStatus
}

// QueryEncode encode options to url query
func (opt *ListNotificationOptions) QueryEncode() string {
	query := opt.getURLQuery()
	if !opt.Since.IsZero() {
		query.Add("since", opt.Since.Format(time.RFC3339))
	}
	if !opt.Before.IsZero() {
		query.Add("before", opt.Before.Format(time.RFC3339))
	}
	for _, s := range opt.Status {
		query.Add("status-types", string(s))
	}
	for _, s := range opt.SubjectTypes {
		query.Add("subject-type", string(s))
	}
	return query.Encode()
}

// Validate the CreateUserOption struct
func (opt ListNotificationOptions) Validate(c *Client) error {
	if len(opt.Status) != 0 {
		return c.checkServerVersionGreaterThanOrEqual(version1_12_3)
	}
	return nil
}

// QueryEncode encode options to url query
func (opt *MarkNotificationOptions) QueryEncode() string {
	query := make(url.Values)
	if !opt.LastReadAt.IsZero() {
		query.Add("last_read_at", opt.LastReadAt.Format(time.RFC3339))
	}
	for _, s := range opt.Status {
		query.Add("status-types", string(s))
	}
	if len(opt.ToStatus) != 0 {
		query.Add("to-status", string(opt.ToStatus))
	}
	return query.Encode()
}

// Validate the CreateUserOption struct
func (opt MarkNotificationOptions) Validate(c *Client) error {
	if len(opt.Status) != 0 || len(opt.ToStatus) != 0 {
		return c.checkServerVersionGreaterThanOrEqual(version1_12_3)
	}
	return nil
}

// CheckNotifications list users's notification threads
func (c *Client) CheckNotifications() (int64, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
		return 0, nil, err
	}
	new := struct {
		New int64 `json:"new"`
	}{}

	resp, err := c.getParsedResponse("GET", "/notifications/new", jsonHeader, nil, &new)
	return new.New, resp, err
}

// GetNotification get notification thread by ID
func (c *Client) GetNotification(id int64) (*NotificationThread, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
		return nil, nil, err
	}
	thread := new(NotificationThread)
	resp, err := c.getParsedResponse("GET", fmt.Sprintf("/notifications/threads/%d", id), nil, nil, thread)
	return thread, resp, err
}

// ReadNotification mark notification thread as read by ID
// It optionally takes a second argument if status has to be set other than 'read'
func (c *Client) ReadNotification(id int64, status ...NotifyStatus) (*Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
		return nil, err
	}
	link := fmt.Sprintf("/notifications/threads/%d", id)
	if len(status) != 0 {
		link += fmt.Sprintf("?to-status=%s", status[0])
	}
	_, resp, err := c.getResponse("PATCH", link, nil, nil)
	return resp, err
}

// ListNotifications list users's notification threads
func (c *Client) ListNotifications(opt ListNotificationOptions) ([]*NotificationThread, *Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(c); err != nil {
		return nil, nil, err
	}
	link, _ := url.Parse("/notifications")
	link.RawQuery = opt.QueryEncode()
	threads := make([]*NotificationThread, 0, 10)
	resp, err := c.getParsedResponse("GET", link.String(), nil, nil, &threads)
	return threads, resp, err
}

// ReadNotifications mark notification threads as read
func (c *Client) ReadNotifications(opt MarkNotificationOptions) (*Response, error) {
	if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
		return nil, err
	}
	if err := opt.Validate(c); err != nil {
		return nil, err
	}
	link, _ := url.Parse("/notifications")
	link.RawQuery = opt.QueryEncode()
	_, resp, err := c.getResponse("PUT", link.String(), nil, nil)
	return resp, err
}

// ListRepoNotifications list users's notification threads on a specific repo
func (c *Client) ListRepoNotifications(owner, repo string, opt ListNotificationOptions) ([]*NotificationThread, *Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
		return nil, nil, err
	}
	if err := opt.Validate(c); err != nil {
		return nil, nil, err
	}
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/notifications", owner, repo))
	link.RawQuery = opt.QueryEncode()
	threads := make([]*NotificationThread, 0, 10)
	resp, err := c.getParsedResponse("GET", link.String(), nil, nil, &threads)
	return threads, resp, err
}

// ReadRepoNotifications mark notification threads as read on a specific repo
func (c *Client) ReadRepoNotifications(owner, repo string, opt MarkNotificationOptions) (*Response, error) {
	if err := escapeValidatePathSegments(&owner, &repo); err != nil {
		return nil, err
	}
	if err := c.checkServerVersionGreaterThanOrEqual(version1_12_0); err != nil {
		return nil, err
	}
	if err := opt.Validate(c); err != nil {
		return nil, err
	}
	link, _ := url.Parse(fmt.Sprintf("/repos/%s/%s/notifications", owner, repo))
	link.RawQuery = opt.QueryEncode()
	_, resp, err := c.getResponse("PUT", link.String(), nil, nil)
	return resp, err
}
