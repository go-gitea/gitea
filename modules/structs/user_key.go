// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package structs

import (
	"time"
)

// PublicKey publickey is a user key to push code to repository
type PublicKey struct {
	ID          int64  `json:"id"`
	Key         string `json:"key"`
	URL         string `json:"url,omitempty"`
	Title       string `json:"title,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	// swagger:strfmt date-time
	Created  time.Time `json:"created_at,omitempty"`
	Owner    *User     `json:"user,omitempty"`
	ReadOnly bool      `json:"read_only,omitempty"`
	KeyType  string    `json:"key_type,omitempty"`
}
