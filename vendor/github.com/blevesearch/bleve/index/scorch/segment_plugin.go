//  Copyright (c) 2019 Couchbase, Inc.
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

package scorch

import (
	"fmt"

	"github.com/blevesearch/bleve/index/scorch/segment"

	zapv11 "github.com/blevesearch/zap/v11"
	zapv12 "github.com/blevesearch/zap/v12"
	zapv13 "github.com/blevesearch/zap/v13"
	zapv14 "github.com/blevesearch/zap/v14"
	zapv15 "github.com/blevesearch/zap/v15"
)

var supportedSegmentPlugins map[string]map[uint32]segment.Plugin
var defaultSegmentPlugin segment.Plugin

func init() {
	ResetPlugins()
	RegisterPlugin(zapv15.Plugin(), false)
	RegisterPlugin(zapv14.Plugin(), false)
	RegisterPlugin(zapv13.Plugin(), false)
	RegisterPlugin(zapv12.Plugin(), false)
	RegisterPlugin(zapv11.Plugin(), true)
}

func ResetPlugins() {
	supportedSegmentPlugins = map[string]map[uint32]segment.Plugin{}
}

func RegisterPlugin(plugin segment.Plugin, makeDefault bool) {
	if _, ok := supportedSegmentPlugins[plugin.Type()]; !ok {
		supportedSegmentPlugins[plugin.Type()] = map[uint32]segment.Plugin{}
	}
	supportedSegmentPlugins[plugin.Type()][plugin.Version()] = plugin
	if makeDefault {
		defaultSegmentPlugin = plugin
	}
}

func SupportedSegmentTypes() (rv []string) {
	for k := range supportedSegmentPlugins {
		rv = append(rv, k)
	}
	return
}

func SupportedSegmentTypeVersions(typ string) (rv []uint32) {
	for k := range supportedSegmentPlugins[typ] {
		rv = append(rv, k)
	}
	return rv
}

func chooseSegmentPlugin(forcedSegmentType string,
	forcedSegmentVersion uint32) (segment.Plugin, error) {
	if versions, ok := supportedSegmentPlugins[forcedSegmentType]; ok {
		if segPlugin, ok := versions[uint32(forcedSegmentVersion)]; ok {
			return segPlugin, nil
		}
		return nil, fmt.Errorf(
			"unsupported version %d for segment type: %s, supported: %v",
			forcedSegmentVersion, forcedSegmentType,
			SupportedSegmentTypeVersions(forcedSegmentType))
	}
	return nil, fmt.Errorf("unsupported segment type: %s, supported: %v",
		forcedSegmentType, SupportedSegmentTypes())
}

func (s *Scorch) loadSegmentPlugin(forcedSegmentType string,
	forcedSegmentVersion uint32) error {
	segPlugin, err := chooseSegmentPlugin(forcedSegmentType,
		forcedSegmentVersion)
	if err != nil {
		return err
	}
	s.segPlugin = segPlugin
	return nil
}
