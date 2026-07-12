// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"sync"
	"sync/atomic"
)

type onceValueResult[T any] struct {
	value T
	panic any
}

// OnceValue is similar to Golang's "sync.OnceValue", this one is reset-able
type OnceValue[T any] struct {
	Func func() T
	mu   sync.Mutex
	res  atomic.Pointer[onceValueResult[T]]
}

func (o *OnceValue[T]) Value() T {
	res := o.res.Load()
	if res == nil {
		o.mu.Lock()
		defer o.mu.Unlock()
		res = o.res.Load()
		if res == nil {
			res = &onceValueResult[T]{}
			defer func() {
				res.panic = recover()
				o.res.Store(res)
				if res.panic != nil {
					panic(res.panic)
				}
			}()
			res.value = o.Func()
		}
	}
	if res.panic != nil {
		panic(res.panic)
	}
	return res.value
}

func (o *OnceValue[T]) Reset() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.res.Store(nil)
}
