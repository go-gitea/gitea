// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea // import "code.gitea.io/sdk/gitea"
import (
	"fmt"
	"time"
)

// Attachment a generic attachment
// swagger:model
type Attachment struct {
	ID            int64  `json:"id"`
	Name          string `json:"name"`
	Size          int64  `json:"size"`
	DownloadCount int64  `json:"download_count"`
	// swagger:strfmt date-time
	Created     time.Time `json:"created_at"`
	UUID        string    `json:"uuid"`
	DownloadURL string    `json:"browser_download_url"`
}

// ListReleaseAttachments list release's attachments
func (c *Client) ListReleaseAttachments(user, repo string, release int64) ([]*Attachment, error) {
	attachments := make([]*Attachment, 0, 10)
	err := c.getParsedResponse("GET",
		fmt.Sprintf("/repos/%s/%s/releases/%d/attachments", user, repo, release),
		nil, nil, &attachments)
	return attachments, err
}

// ListReleaseAttachments list release's attachments
func (c *Client) GetReleaseAttachment(user, repo string, release int64, id int64) (*Attachment, error) {
	a := new(Attachment)
	err := c.getParsedResponse("GET",
		fmt.Sprintf("/repos/%s/%s/releases/%d/attachments/%d", user, repo, release, id),
		nil, nil, &a)
	return a, err
}
