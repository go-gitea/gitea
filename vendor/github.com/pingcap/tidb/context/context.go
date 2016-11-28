// Copyright 2015 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package context

import (
	"fmt"

	"github.com/pingcap/tidb/kv"
)

// Context is an interface for transaction and executive args environment.
type Context interface {
	// GetTxn gets a transaction for futher execution.
	GetTxn(forceNew bool) (kv.Transaction, error)

	// FinishTxn commits or rolls back the current transaction.
	FinishTxn(rollback bool) error

	// SetValue saves a value associated with this context for key.
	SetValue(key fmt.Stringer, value interface{})

	// Value returns the value associated with this context for key.
	Value(key fmt.Stringer) interface{}

	// ClearValue clears the value associated with this context for key.
	ClearValue(key fmt.Stringer)
}
