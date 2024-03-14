// Copyright 2024 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package optional

import "reflect"

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

// ExcractValue return value or nil and bool if object was an Optional
// it should only be used if you already have to deal with interface{} values
// and expect an Option type within it.
func ExcractValue(obj any) (any, bool) {
	rt := reflect.TypeOf(obj)
	if rt.Kind() != reflect.Slice {
		return nil, false
	}

	type hasHasFunc interface {
		Has() bool
	}
	if hasObj, ok := obj.(hasHasFunc); !ok {
		return nil, false
	} else if !hasObj.Has() {
		return nil, true
	}

	rv := reflect.ValueOf(obj)
	if rv.Len() != 1 {
		// it's still false as optional.Option[T] types would have reported with hasObj.Has() that it is empty
		return nil, false
	}
	return rv.Index(0).Interface(), true
}
