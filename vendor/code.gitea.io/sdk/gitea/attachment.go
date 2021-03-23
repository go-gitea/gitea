// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea // import "code.gitea.io/sdk/gitea"
import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"time"
)

// Attachment a generic attachment
type Attachment struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	Size          int64     `json:"size"`
	DownloadCount int64     `json:"download_count"`
	Created       time.Time `json:"created_at"`
	UUID          string    `json:"uuid"`
	DownloadURL   string    `json:"browser_download_url"`
}

// ListReleaseAttachmentsOptions options for listing release's attachments
type ListReleaseAttachmentsOptions struct {
	ListOptions
}

// ListReleaseAttachments list release's attachments
func (c *Client) ListReleaseAttachments(user, repo string, release int64, opt ListReleaseAttachmentsOptions) ([]*Attachment, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	opt.setDefaults()
	attachments := make([]*Attachment, 0, opt.PageSize)
	resp, err := c.getParsedResponse("GET",
		fmt.Sprintf("/repos/%s/%s/releases/%d/assets?%s", user, repo, release, opt.getURLQuery().Encode()),
		nil, nil, &attachments)
	return attachments, resp, err
}

// GetReleaseAttachment returns the requested attachment
func (c *Client) GetReleaseAttachment(user, repo string, release int64, id int64) (*Attachment, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	a := new(Attachment)
	resp, err := c.getParsedResponse("GET",
		fmt.Sprintf("/repos/%s/%s/releases/%d/assets/%d", user, repo, release, id),
		nil, nil, &a)
	return a, resp, err
}

// CreateReleaseAttachment creates an attachment for the given release
func (c *Client) CreateReleaseAttachment(user, repo string, release int64, file io.Reader, filename string) (*Attachment, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	// Write file to body
	body := new(bytes.Buffer)
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("attachment", filename)
	if err != nil {
		return nil, nil, err
	}

	if _, err = io.Copy(part, file); err != nil {
		return nil, nil, err
	}
	if err = writer.Close(); err != nil {
		return nil, nil, err
	}

	// Send request
	attachment := new(Attachment)
	resp, err := c.getParsedResponse("POST",
		fmt.Sprintf("/repos/%s/%s/releases/%d/assets", user, repo, release),
		http.Header{"Content-Type": {writer.FormDataContentType()}}, body, &attachment)
	return attachment, resp, err
}

// EditAttachmentOptions options for editing attachments
type EditAttachmentOptions struct {
	Name string `json:"name"`
}

// EditReleaseAttachment updates the given attachment with the given options
func (c *Client) EditReleaseAttachment(user, repo string, release int64, attachment int64, form EditAttachmentOptions) (*Attachment, *Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, nil, err
	}
	body, err := json.Marshal(&form)
	if err != nil {
		return nil, nil, err
	}
	attach := new(Attachment)
	resp, err := c.getParsedResponse("PATCH", fmt.Sprintf("/repos/%s/%s/releases/%d/assets/%d", user, repo, release, attachment), jsonHeader, bytes.NewReader(body), attach)
	return attach, resp, err
}

// DeleteReleaseAttachment deletes the given attachment including the uploaded file
func (c *Client) DeleteReleaseAttachment(user, repo string, release int64, id int64) (*Response, error) {
	if err := escapeValidatePathSegments(&user, &repo); err != nil {
		return nil, err
	}
	_, resp, err := c.getResponse("DELETE", fmt.Sprintf("/repos/%s/%s/releases/%d/assets/%d", user, repo, release, id), nil, nil)
	return resp, err
}
