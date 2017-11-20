// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea // import "code.gitea.io/sdk/gitea"
import "time"

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
