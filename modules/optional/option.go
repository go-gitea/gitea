// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package optional

import "strconv"

// Option is a generic type that can hold a value of type T or be empty (None).
//
// It must use the slice type to work with "chi" form values binding:
// * non-existing value are represented as an empty slice (None)
// * existing value is represented as a slice with one element (Some)
// * multiple values are represented as a slice with multiple elements (Some), the Value is the first element (not well-defined in this case)
type Option[T any] []T

func None[T any]() Option[T] {
	return nil
}

func Some[T any](v T) Option[T] {
	return Option[T]{v}
}

func FromPtr[T any](v *T) Option[T] {
	if v == nil {
		return None[T]()
	}
	return Some(*v)
}

func FromNonDefault[T comparable](v T) Option[T] {
	var zero T
	if v == zero {
		return None[T]()
	}
	return Some(v)
}

func (o Option[T]) Has() bool {
	return o != nil
}

func (o Option[T]) Value() T {
	var zero T
	return o.ValueOrDefault(zero)
}

func (o Option[T]) ValueOrDefault(v T) T {
	if o.Has() {
		return o[0]
	}
	return v
}

// ParseBool get the corresponding optional.Option[bool] of a string using strconv.ParseBool
func ParseBool(s string) Option[bool] {
	v, e := strconv.ParseBool(s)
	if e != nil {
		return None[bool]()
	}
	return Some(v)
}
