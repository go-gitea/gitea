// Copyright 2019 Lunny Xiao. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package levelqueue

import "errors"

var (
	// ErrNotFound means no element in queue
	ErrNotFound = errors.New("no key found")
)
