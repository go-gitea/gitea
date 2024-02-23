// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package values

type NullableValue[T any] struct {
	value *T
	isNil bool
}

func (n NullableValue[T]) IsNone() bool {
	return n.isNil
}

func (n NullableValue[T]) IsSome() bool {
	return !n.isNil
}

func (n NullableValue[T]) Value() T {
	// check if the value is nil first, otherwise panic
	return *n.value
}

func None[T any]() NullableValue[T] {
	return NullableValue[T]{nil, true}
}

func Nullable[T any](value T) NullableValue[T] {
	return NullableValue[T]{&value, false}
}
