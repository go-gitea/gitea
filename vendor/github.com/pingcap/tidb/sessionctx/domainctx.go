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

package sessionctx

import (
	"github.com/pingcap/tidb/context"
	"github.com/pingcap/tidb/domain"
)

// A dummy type to avoid naming collision in context.
type domainKeyType int

// String defines a Stringer function for debugging and pretty printing.
func (k domainKeyType) String() string {
	return "domain"
}

const domainKey domainKeyType = 0

// BindDomain binds domain to context.
func BindDomain(ctx context.Context, domain *domain.Domain) {
	ctx.SetValue(domainKey, domain)
}

// GetDomain gets domain from context.
func GetDomain(ctx context.Context) *domain.Domain {
	v, ok := ctx.Value(domainKey).(*domain.Domain)
	if !ok {
		return nil
	}
	return v
}
