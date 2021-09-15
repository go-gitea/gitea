// Copyright 2020 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package utils

import (
	"fmt"
	"strings"
)

// IndexName returns index name
func IndexName(tableName, idxName string) string {
	return fmt.Sprintf("IDX_%v_%v", tableName, idxName)
}

// SeqName returns sequence name for some table
func SeqName(tableName string) string {
	return "SEQ_" + strings.ToUpper(tableName)
}
