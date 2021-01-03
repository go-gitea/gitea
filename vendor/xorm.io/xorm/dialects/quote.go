// Copyright 2020 The Xorm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package dialects

// QuotePolicy describes quote handle policy
type QuotePolicy int

// All QuotePolicies
const (
	QuotePolicyAlways QuotePolicy = iota
	QuotePolicyNone
	QuotePolicyReserved
)
