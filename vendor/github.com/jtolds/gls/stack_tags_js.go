// +build js

package gls

// This file is used for GopherJS builds, which don't have normal runtime support

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/gopherjs/gopherjs/js"
)

var stackRE = regexp.MustCompile("\\s+at (\\S*) \\([^:]+:(\\d+):(\\d+)")

func findPtr() uintptr {
	jsStack := js.Global.Get("Error").New().Get("stack").Call("split", "\n")
	for i := 1; i < jsStack.Get("length").Int(); i++ {
		item := jsStack.Index(i).String()
		matches := stackRE.FindAllStringSubmatch(item, -1)
		if matches == nil {
			return 0
		}
		pkgPath := matches[0][1]
		if strings.HasPrefix(pkgPath, "$packages.github.com/jtolds/gls.mark") {
			line, _ := strconv.Atoi(matches[0][2])
			char, _ := strconv.Atoi(matches[0][3])
			x := (uintptr(line) << 16) | uintptr(char)
			return x
		}
	}

	return 0
}

func init() {
	setEntries := func(f func(uint, func()), v int8) {
		var ptr uintptr
		f(0, func() {
			ptr = findPtr()
		})
		pc_lookup[ptr] = v
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

func currentStack(skip int) (stack []uintptr) {
	jsStack := js.Global.Get("Error").New().Get("stack").Call("split", "\n")
	for i := skip + 2; i < jsStack.Get("length").Int(); i++ {
		item := jsStack.Index(i).String()
		matches := stackRE.FindAllStringSubmatch(item, -1)
		if matches == nil {
			return stack
		}
		line, _ := strconv.Atoi(matches[0][2])
		char, _ := strconv.Atoi(matches[0][3])
		x := (uintptr(line) << 16) | uintptr(char)&0xffff
		stack = append(stack, x)
	}

	return stack
}

func readStackTags(skip int) (tags []uint) {
	stack := currentStack(skip)
	var current_tag uint
	for _, pc := range stack {
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
