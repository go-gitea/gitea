// Copyright 2017 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gitea // import "code.gitea.io/sdk/gitea"
import "time"

// a generic attachment
type Attachment struct {
	ID            int64     `json:"id"`
	Created       time.Time `json:"created_at"`
	Name          string    `json:"name"`
	UUID          string    `json:"uuid"`
	DownloadURL   string    `json:"download_url"`
	DownloadCount int64     `json:"download_count"`
}
