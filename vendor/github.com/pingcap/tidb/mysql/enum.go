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

package mysql

import (
	"strconv"
	"strings"

	"github.com/juju/errors"
)

// Enum is for MySQL enum type.
type Enum struct {
	Name  string
	Value uint64
}

// String implements fmt.Stringer interface.
func (e Enum) String() string {
	return e.Name
}

// ToNumber changes enum index to float64 for numeric operation.
func (e Enum) ToNumber() float64 {
	return float64(e.Value)
}

// ParseEnumName creates a Enum with item name.
func ParseEnumName(elems []string, name string) (Enum, error) {
	for i, n := range elems {
		if strings.EqualFold(n, name) {
			return Enum{Name: n, Value: uint64(i) + 1}, nil
		}
	}

	// name doesn't exist, maybe an integer?
	if num, err := strconv.ParseUint(name, 0, 64); err == nil {
		return ParseEnumValue(elems, num)
	}

	return Enum{}, errors.Errorf("item %s is not in enum %v", name, elems)
}

// ParseEnumValue creates a Enum with special number.
func ParseEnumValue(elems []string, number uint64) (Enum, error) {
	if number == 0 || number > uint64(len(elems)) {
		return Enum{}, errors.Errorf("number %d overflow enum boundary [1, %d]", number, len(elems))
	}

	return Enum{Name: elems[number-1], Value: number}, nil
}
