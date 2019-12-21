// +build !go1.7 safe appengine

// Copyright (c) 2012-2018 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

package codec

import (
	"reflect"
	"sync/atomic"
	"time"
)

const safeMode = true

// stringView returns a view of the []byte as a string.
// In unsafe mode, it doesn't incur allocation and copying caused by conversion.
// In regular safe mode, it is an allocation and copy.
//
// Usage: Always maintain a reference to v while result of this call is in use,
//        and call keepAlive4BytesView(v) at point where done with view.
func stringView(v []byte) string {
	return string(v)
}

// bytesView returns a view of the string as a []byte.
// In unsafe mode, it doesn't incur allocation and copying caused by conversion.
// In regular safe mode, it is an allocation and copy.
//
// Usage: Always maintain a reference to v while result of this call is in use,
//        and call keepAlive4BytesView(v) at point where done with view.
func bytesView(v string) []byte {
	return []byte(v)
}

// isNil says whether the value v is nil.
// This applies to references like map/ptr/unsafepointer/chan/func,
// and non-reference values like interface/slice.
func isNil(v interface{}) (rv reflect.Value, isnil bool) {
	rv = rv4i(v)
	if isnilBitset.isset(byte(rv.Kind())) {
		isnil = rv.IsNil()
	}
	return
}

func rv4i(i interface{}) reflect.Value {
	return reflect.ValueOf(i)
}

func rv2i(rv reflect.Value) interface{} {
	return rv.Interface()
}

func rvIsNil(rv reflect.Value) bool {
	return rv.IsNil()
}

func rvSetSliceLen(rv reflect.Value, length int) {
	rv.SetLen(length)
}

func rvZeroAddrK(t reflect.Type, k reflect.Kind) reflect.Value {
	return reflect.New(t).Elem()
}

func rvConvert(v reflect.Value, t reflect.Type) (rv reflect.Value) {
	return v.Convert(t)
}

func rt2id(rt reflect.Type) uintptr {
	return rv4i(rt).Pointer()
}

func i2rtid(i interface{}) uintptr {
	return rv4i(reflect.TypeOf(i)).Pointer()
}

// --------------------------

func isEmptyValue(v reflect.Value, tinfos *TypeInfos, deref, checkStruct bool) bool {
	switch v.Kind() {
	case reflect.Invalid:
		return true
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		if deref {
			if v.IsNil() {
				return true
			}
			return isEmptyValue(v.Elem(), tinfos, deref, checkStruct)
		}
		return v.IsNil()
	case reflect.Struct:
		return isEmptyStruct(v, tinfos, deref, checkStruct)
	}
	return false
}

// --------------------------
type atomicClsErr struct {
	v atomic.Value
}

func (x *atomicClsErr) load() (e clsErr) {
	if i := x.v.Load(); i != nil {
		e = i.(clsErr)
	}
	return
}

func (x *atomicClsErr) store(p clsErr) {
	x.v.Store(p)
}

// --------------------------
type atomicTypeInfoSlice struct { // expected to be 2 words
	v atomic.Value
}

func (x *atomicTypeInfoSlice) load() (e []rtid2ti) {
	if i := x.v.Load(); i != nil {
		e = i.([]rtid2ti)
	}
	return
}

func (x *atomicTypeInfoSlice) store(p []rtid2ti) {
	x.v.Store(p)
}

// --------------------------
type atomicRtidFnSlice struct { // expected to be 2 words
	v atomic.Value
}

func (x *atomicRtidFnSlice) load() (e []codecRtidFn) {
	if i := x.v.Load(); i != nil {
		e = i.([]codecRtidFn)
	}
	return
}

func (x *atomicRtidFnSlice) store(p []codecRtidFn) {
	x.v.Store(p)
}

// --------------------------
func (n *decNaked) ru() reflect.Value {
	return rv4i(&n.u).Elem()
}
func (n *decNaked) ri() reflect.Value {
	return rv4i(&n.i).Elem()
}
func (n *decNaked) rf() reflect.Value {
	return rv4i(&n.f).Elem()
}
func (n *decNaked) rl() reflect.Value {
	return rv4i(&n.l).Elem()
}
func (n *decNaked) rs() reflect.Value {
	return rv4i(&n.s).Elem()
}
func (n *decNaked) rt() reflect.Value {
	return rv4i(&n.t).Elem()
}
func (n *decNaked) rb() reflect.Value {
	return rv4i(&n.b).Elem()
}

// --------------------------
func rvSetBytes(rv reflect.Value, v []byte) {
	rv.SetBytes(v)
}

func rvSetString(rv reflect.Value, v string) {
	rv.SetString(v)
}

func rvSetBool(rv reflect.Value, v bool) {
	rv.SetBool(v)
}

func rvSetTime(rv reflect.Value, v time.Time) {
	rv.Set(rv4i(v))
}

func rvSetFloat32(rv reflect.Value, v float32) {
	rv.SetFloat(float64(v))
}

func rvSetFloat64(rv reflect.Value, v float64) {
	rv.SetFloat(v)
}

func rvSetInt(rv reflect.Value, v int) {
	rv.SetInt(int64(v))
}

func rvSetInt8(rv reflect.Value, v int8) {
	rv.SetInt(int64(v))
}

func rvSetInt16(rv reflect.Value, v int16) {
	rv.SetInt(int64(v))
}

func rvSetInt32(rv reflect.Value, v int32) {
	rv.SetInt(int64(v))
}

func rvSetInt64(rv reflect.Value, v int64) {
	rv.SetInt(v)
}

func rvSetUint(rv reflect.Value, v uint) {
	rv.SetUint(uint64(v))
}

func rvSetUintptr(rv reflect.Value, v uintptr) {
	rv.SetUint(uint64(v))
}

func rvSetUint8(rv reflect.Value, v uint8) {
	rv.SetUint(uint64(v))
}

func rvSetUint16(rv reflect.Value, v uint16) {
	rv.SetUint(uint64(v))
}

func rvSetUint32(rv reflect.Value, v uint32) {
	rv.SetUint(uint64(v))
}

func rvSetUint64(rv reflect.Value, v uint64) {
	rv.SetUint(v)
}

// ----------------

// rvSetDirect is rv.Set for all kinds except reflect.Interface
func rvSetDirect(rv reflect.Value, v reflect.Value) {
	rv.Set(v)
}

// rvSlice returns a slice of the slice of lenth
func rvSlice(rv reflect.Value, length int) reflect.Value {
	return rv.Slice(0, length)
}

// ----------------

func rvSliceIndex(rv reflect.Value, i int, ti *typeInfo) reflect.Value {
	return rv.Index(i)
}

func rvGetSliceLen(rv reflect.Value) int {
	return rv.Len()
}

func rvGetSliceCap(rv reflect.Value) int {
	return rv.Cap()
}

func rvGetArrayBytesRO(rv reflect.Value, scratch []byte) (bs []byte) {
	l := rv.Len()
	if rv.CanAddr() {
		return rvGetBytes(rv.Slice(0, l))
	}

	if l <= cap(scratch) {
		bs = scratch[:l]
	} else {
		bs = make([]byte, l)
	}
	reflect.Copy(rv4i(bs), rv)
	return
}

func rvGetArray4Slice(rv reflect.Value) (v reflect.Value) {
	v = rvZeroAddrK(reflectArrayOf(rvGetSliceLen(rv), rv.Type().Elem()), reflect.Array)
	reflect.Copy(v, rv)
	return
}

func rvGetSlice4Array(rv reflect.Value, tslice reflect.Type) (v reflect.Value) {
	return rv.Slice(0, rv.Len())
}

func rvCopySlice(dest, src reflect.Value) {
	reflect.Copy(dest, src)
}

// ------------

func rvGetBool(rv reflect.Value) bool {
	return rv.Bool()
}

func rvGetBytes(rv reflect.Value) []byte {
	return rv.Bytes()
}

func rvGetTime(rv reflect.Value) time.Time {
	return rv2i(rv).(time.Time)
}

func rvGetString(rv reflect.Value) string {
	return rv.String()
}

func rvGetFloat64(rv reflect.Value) float64 {
	return rv.Float()
}

func rvGetFloat32(rv reflect.Value) float32 {
	return float32(rv.Float())
}

func rvGetInt(rv reflect.Value) int {
	return int(rv.Int())
}

func rvGetInt8(rv reflect.Value) int8 {
	return int8(rv.Int())
}

func rvGetInt16(rv reflect.Value) int16 {
	return int16(rv.Int())
}

func rvGetInt32(rv reflect.Value) int32 {
	return int32(rv.Int())
}

func rvGetInt64(rv reflect.Value) int64 {
	return rv.Int()
}

func rvGetUint(rv reflect.Value) uint {
	return uint(rv.Uint())
}

func rvGetUint8(rv reflect.Value) uint8 {
	return uint8(rv.Uint())
}

func rvGetUint16(rv reflect.Value) uint16 {
	return uint16(rv.Uint())
}

func rvGetUint32(rv reflect.Value) uint32 {
	return uint32(rv.Uint())
}

func rvGetUint64(rv reflect.Value) uint64 {
	return rv.Uint()
}

func rvGetUintptr(rv reflect.Value) uintptr {
	return uintptr(rv.Uint())
}

// ------------ map range and map indexing ----------

func mapGet(m, k, v reflect.Value) (vv reflect.Value) {
	return m.MapIndex(k)
}

func mapSet(m, k, v reflect.Value) {
	m.SetMapIndex(k, v)
}

func mapDelete(m, k reflect.Value) {
	m.SetMapIndex(k, reflect.Value{})
}

// return an addressable reflect value that can be used in mapRange and mapGet operations.
//
// all calls to mapGet or mapRange will call here to get an addressable reflect.Value.
func mapAddressableRV(t reflect.Type, k reflect.Kind) (r reflect.Value) {
	return // reflect.New(t).Elem()
}

// ---------- ENCODER optimized ---------------

func (e *Encoder) jsondriver() *jsonEncDriver {
	return e.e.(*jsonEncDriver)
}

// ---------- DECODER optimized ---------------

func (d *Decoder) checkBreak() bool {
	return d.d.CheckBreak()
}

func (d *Decoder) jsondriver() *jsonDecDriver {
	return d.d.(*jsonDecDriver)
}
