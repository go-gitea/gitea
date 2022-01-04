// Copyright 2021 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package convert

import "database/sql"

var (
	_ sql.Scanner = &EmptyScanner{}
)

// EmptyScanner represents an empty scanner which will ignore the scan
type EmptyScanner struct{}

// Scan implements sql.Scanner
func (EmptyScanner) Scan(value interface{}) error {
	return nil
}
