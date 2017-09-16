// Copyright 2017 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build go1.6

package xorm

import "time"

// SetConnMaxLifetime sets the maximum amount of time a connection may be reused.
func (engine *Engine) SetConnMaxLifetime(d time.Duration) {
	engine.db.SetConnMaxLifetime(d)
}
