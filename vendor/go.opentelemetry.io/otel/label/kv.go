// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package label // import "go.opentelemetry.io/otel/label"

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// KeyValue holds a key and value pair.
type KeyValue struct {
	Key   Key
	Value Value
}

// Bool creates a new key-value pair with a passed name and a bool
// value.
func Bool(k string, v bool) KeyValue {
	return Key(k).Bool(v)
}

// Int64 creates a new key-value pair with a passed name and an int64
// value.
func Int64(k string, v int64) KeyValue {
	return Key(k).Int64(v)
}

// Uint64 creates a new key-value pair with a passed name and a uint64
// value.
func Uint64(k string, v uint64) KeyValue {
	return Key(k).Uint64(v)
}

// Float64 creates a new key-value pair with a passed name and a float64
// value.
func Float64(k string, v float64) KeyValue {
	return Key(k).Float64(v)
}

// Int32 creates a new key-value pair with a passed name and an int32
// value.
func Int32(k string, v int32) KeyValue {
	return Key(k).Int32(v)
}

// Uint32 creates a new key-value pair with a passed name and a uint32
// value.
func Uint32(k string, v uint32) KeyValue {
	return Key(k).Uint32(v)
}

// Float32 creates a new key-value pair with a passed name and a float32
// value.
func Float32(k string, v float32) KeyValue {
	return Key(k).Float32(v)
}

// String creates a new key-value pair with a passed name and a string
// value.
func String(k, v string) KeyValue {
	return Key(k).String(v)
}

// Stringer creates a new key-value pair with a passed name and a string
// value generated by the passed Stringer interface.
func Stringer(k string, v fmt.Stringer) KeyValue {
	return Key(k).String(v.String())
}

// Int creates a new key-value pair instance with a passed name and
// either an int32 or an int64 value, depending on whether the int
// type is 32 or 64 bits wide.
func Int(k string, v int) KeyValue {
	return Key(k).Int(v)
}

// Uint creates a new key-value pair instance with a passed name and
// either an uint32 or an uint64 value, depending on whether the uint
// type is 32 or 64 bits wide.
func Uint(k string, v uint) KeyValue {
	return Key(k).Uint(v)
}

// Array creates a new key-value pair with a passed name and a array.
// Only arrays of primitive type are supported.
func Array(k string, v interface{}) KeyValue {
	return Key(k).Array(v)
}

// Any creates a new key-value pair instance with a passed name and
// automatic type inference. This is slower, and not type-safe.
func Any(k string, value interface{}) KeyValue {
	if value == nil {
		return String(k, "<nil>")
	}

	if stringer, ok := value.(fmt.Stringer); ok {
		return String(k, stringer.String())
	}

	rv := reflect.ValueOf(value)

	switch rv.Kind() {
	case reflect.Array, reflect.Slice:
		return Array(k, value)
	case reflect.Bool:
		return Bool(k, rv.Bool())
	case reflect.Int, reflect.Int8, reflect.Int16:
		return Int(k, int(rv.Int()))
	case reflect.Int32:
		return Int32(k, int32(rv.Int()))
	case reflect.Int64:
		return Int64(k, int64(rv.Int()))
	case reflect.Uint, reflect.Uint8, reflect.Uint16:
		return Uint(k, uint(rv.Uint()))
	case reflect.Uint32:
		return Uint32(k, uint32(rv.Uint()))
	case reflect.Uint64, reflect.Uintptr:
		return Uint64(k, rv.Uint())
	case reflect.Float32:
		return Float32(k, float32(rv.Float()))
	case reflect.Float64:
		return Float64(k, rv.Float())
	case reflect.String:
		return String(k, rv.String())
	}
	if b, err := json.Marshal(value); b != nil && err == nil {
		return String(k, string(b))
	}
	return String(k, fmt.Sprint(value))
}
