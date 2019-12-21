// Copyright (c) 2012-2018 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

// +build !go1.12
// +build !go1.7 safe

package codec

import "reflect"

type mapIter struct {
	m      reflect.Value
	keys   []reflect.Value
	j      int
	values bool
}

func (t *mapIter) ValidKV() (r bool) {
	return true
}

func (t *mapIter) Next() (r bool) {
	t.j++
	return t.j < len(t.keys)
}

func (t *mapIter) Key() reflect.Value {
	return t.keys[t.j]
}

func (t *mapIter) Value() (r reflect.Value) {
	if t.values {
		return t.m.MapIndex(t.keys[t.j])
	}
	return
}

func (t *mapIter) Done() {}

func mapRange(t *mapIter, m, k, v reflect.Value, values bool) {
	*t = mapIter{
		m:      m,
		keys:   m.MapKeys(),
		values: values,
		j:      -1,
	}
}
