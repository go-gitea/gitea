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

package variable

import (
	"github.com/juju/errors"
)

var statisticsList []Statistics
var globalStatusScopes = make(map[string]ScopeFlag)

// DefaultScopeFlag is the status default scope.
var DefaultScopeFlag = ScopeGlobal | ScopeSession

// StatusVal is the value of the corresponding status variable.
type StatusVal struct {
	Scope ScopeFlag
	Value interface{}
}

// Statistics is the interface of statistics.
type Statistics interface {
	// GetScope gets the status variables scope.
	GetScope(status string) ScopeFlag
	// Stats returns the statistics status variables.
	Stats() (map[string]interface{}, error)
}

// RegisterStatistics registers statistics.
func RegisterStatistics(s Statistics) {
	statisticsList = append(statisticsList, s)
}

// GetStatusVars gets registered statistics status variables.
func GetStatusVars() (map[string]*StatusVal, error) {
	statusVars := make(map[string]*StatusVal)

	for _, statistics := range statisticsList {
		vals, err := statistics.Stats()
		if err != nil {
			return nil, errors.Trace(err)
		}

		for name, val := range vals {
			scope := statistics.GetScope(name)
			statusVars[name] = &StatusVal{Value: val, Scope: scope}
		}
	}

	return statusVars, nil
}
