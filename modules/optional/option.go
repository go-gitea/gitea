// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package optional

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
