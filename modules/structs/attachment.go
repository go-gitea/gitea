// Copyright 2017 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package structs // import "code.gitea.io/gitea/modules/structs"

import (
	"time"
)

// Attachment a generic attachment
// swagger:model
type Attachment struct {
	// ID is the unique identifier for the attachment
	ID int64 `json:"id"`
	// Name is the filename of the attachment
	Name string `json:"name"`
	// Size is the file size in bytes
	Size int64 `json:"size"`
	// DownloadCount is the number of times the attachment has been downloaded
	DownloadCount int64 `json:"download_count"`
	// swagger:strfmt date-time
	// Created is the time when the attachment was uploaded
	Created time.Time `json:"created_at"`
	// UUID is the unique identifier for the attachment file
	UUID string `json:"uuid"`
	// DownloadURL is the URL to download the attachment
	DownloadURL string `json:"browser_download_url"`
}

// EditAttachmentOptions options for editing attachments
// swagger:model
type EditAttachmentOptions struct {
	// Name is the new filename for the attachment
	Name string `json:"name"`
}
