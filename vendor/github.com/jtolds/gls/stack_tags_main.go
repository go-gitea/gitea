// +build !js

package gls

// This file is used for standard Go builds, which have the expected runtime support

import (
	"reflect"
	"runtime"
)

func init() {
	setEntries := func(f func(uint, func()), v int8) {
		pc_lookup[reflect.ValueOf(f).Pointer()] = v
		if v >= 0 {
			mark_lookup[v] = f
		}
	}
	setEntries(markS, -0x1)
	setEntries(mark0, 0x0)
	setEntries(mark1, 0x1)
	setEntries(mark2, 0x2)
	setEntries(mark3, 0x3)
	setEntries(mark4, 0x4)
	setEntries(mark5, 0x5)
	setEntries(mark6, 0x6)
	setEntries(mark7, 0x7)
	setEntries(mark8, 0x8)
	setEntries(mark9, 0x9)
	setEntries(markA, 0xa)
	setEntries(markB, 0xb)
	setEntries(markC, 0xc)
	setEntries(markD, 0xd)
	setEntries(markE, 0xe)
	setEntries(markF, 0xf)
}

func currentStack(skip int) []uintptr {
	stack := make([]uintptr, maxCallers)
	return stack[:runtime.Callers(3+skip, stack)]
}

func readStackTags(skip int) (tags []uint) {
	stack := currentStack(skip)
	var current_tag uint
	for _, pc := range stack {
		pc = runtime.FuncForPC(pc).Entry()
		val, ok := pc_lookup[pc]
		if !ok {
			continue
		}
		if val < 0 {
			tags = append(tags, current_tag)
			current_tag = 0
			continue
		}
		current_tag <<= bitWidth
		current_tag += uint(val)
	}
	return
}
