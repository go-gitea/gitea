// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package values

type NullableValue[T comparable] struct {
	value *T
	isNil bool
}

func (n NullableValue[T]) IsNone() bool {
	return n.isNil
}

func (n NullableValue[T]) Value() T {
	// check if the value IsNone first, otherwise panic
	return *n.value
}

func (n NullableValue[T]) Equal(v T) bool {
	return n.value != nil && *n.value == v
}

func None[T comparable]() NullableValue[T] {
	return NullableValue[T]{nil, true}
}

func Nullable[T comparable](value T) NullableValue[T] {
	return NullableValue[T]{&value, false}
}
