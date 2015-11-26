// Copyright 2015 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package git

import (
	"time"
)

// Signature represents the Author or Committer information.
type Signature struct {
	Email string
	Name  string
	When  time.Time
}
