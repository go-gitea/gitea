//  Copyright (c) 2020 Couchbase, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 		http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package zap

import (
	"github.com/blevesearch/bleve/index/scorch/segment"
)

// ZapPlugin implements the Plugin interface of
// the blevesearch/bleve/index/scorch/segment pkg
type ZapPlugin struct{}

func (*ZapPlugin) Type() string {
	return Type
}

func (*ZapPlugin) Version() uint32 {
	return Version
}

// Plugin returns an instance segment.Plugin for use
// by the Scorch indexing scheme
func Plugin() segment.Plugin {
	return &ZapPlugin{}
}
