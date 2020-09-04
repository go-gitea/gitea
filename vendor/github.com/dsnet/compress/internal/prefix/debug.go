// Copyright 2015, Joe Tsai. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE.md file.

// +build debug

package prefix

import (
	"fmt"
	"math"
	"strings"
)

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func lenBase2(n uint) int {
	return int(math.Ceil(math.Log2(float64(n + 1))))
}
func padBase2(v, n uint, m int) string {
	s := fmt.Sprintf("%b", 1<<n|v)[1:]
	if pad := m - len(s); pad > 0 {
		return strings.Repeat(" ", pad) + s
	}
	return s
}

func lenBase10(n int) int {
	return int(math.Ceil(math.Log10(float64(n + 1))))
}
func padBase10(n, m int) string {
	s := fmt.Sprintf("%d", n)
	if pad := m - len(s); pad > 0 {
		return strings.Repeat(" ", pad) + s
	}
	return s
}

func (rc RangeCodes) String() string {
	var maxLen, maxBase int
	for _, c := range rc {
		maxLen = max(maxLen, int(c.Len))
		maxBase = max(maxBase, int(c.Base))
	}

	var ss []string
	ss = append(ss, "{")
	for i, c := range rc {
		base := padBase10(int(c.Base), lenBase10(maxBase))
		if c.Len > 0 {
			base += fmt.Sprintf("-%d", c.End()-1)
		}
		ss = append(ss, fmt.Sprintf("\t%s:  {len: %s, range: %s},",
			padBase10(int(i), lenBase10(len(rc)-1)),
			padBase10(int(c.Len), lenBase10(maxLen)),
			base,
		))
	}
	ss = append(ss, "}")
	return strings.Join(ss, "\n")
}

func (pc PrefixCodes) String() string {
	var maxSym, maxLen, maxCnt int
	for _, c := range pc {
		maxSym = max(maxSym, int(c.Sym))
		maxLen = max(maxLen, int(c.Len))
		maxCnt = max(maxCnt, int(c.Cnt))
	}

	var ss []string
	ss = append(ss, "{")
	for _, c := range pc {
		var cntStr string
		if maxCnt > 0 {
			cnt := int(32*float32(c.Cnt)/float32(maxCnt) + 0.5)
			cntStr = fmt.Sprintf("%s |%s",
				padBase10(int(c.Cnt), lenBase10(maxCnt)),
				strings.Repeat("#", cnt),
			)
		}
		ss = append(ss, fmt.Sprintf("\t%s:  %s,  %s",
			padBase10(int(c.Sym), lenBase10(maxSym)),
			padBase2(uint(c.Val), uint(c.Len), maxLen),
			cntStr,
		))
	}
	ss = append(ss, "}")
	return strings.Join(ss, "\n")
}

func (pd Decoder) String() string {
	var ss []string
	ss = append(ss, "{")
	if len(pd.chunks) > 0 {
		ss = append(ss, "\tchunks: {")
		for i, c := range pd.chunks {
			label := "sym"
			if uint(c&countMask) > uint(pd.chunkBits) {
				label = "idx"
			}
			ss = append(ss, fmt.Sprintf("\t\t%s:  {%s: %s, len: %s}",
				padBase2(uint(i), uint(pd.chunkBits), int(pd.chunkBits)),
				label, padBase10(int(c>>countBits), 3),
				padBase10(int(c&countMask), 2),
			))
		}
		ss = append(ss, "\t},")

		for j, links := range pd.links {
			ss = append(ss, fmt.Sprintf("\tlinks[%d]: {", j))
			linkBits := lenBase2(uint(pd.linkMask))
			for i, c := range links {
				ss = append(ss, fmt.Sprintf("\t\t%s:  {sym: %s, len: %s},",
					padBase2(uint(i), uint(linkBits), int(linkBits)),
					padBase10(int(c>>countBits), 3),
					padBase10(int(c&countMask), 2),
				))
			}
			ss = append(ss, "\t},")
		}
	}
	ss = append(ss, fmt.Sprintf("\tchunkMask: %b,", pd.chunkMask))
	ss = append(ss, fmt.Sprintf("\tlinkMask:  %b,", pd.linkMask))
	ss = append(ss, fmt.Sprintf("\tchunkBits: %d,", pd.chunkBits))
	ss = append(ss, fmt.Sprintf("\tMinBits:   %d,", pd.MinBits))
	ss = append(ss, fmt.Sprintf("\tNumSyms:   %d,", pd.NumSyms))
	ss = append(ss, "}")
	return strings.Join(ss, "\n")
}

func (pe Encoder) String() string {
	var maxLen int
	for _, c := range pe.chunks {
		maxLen = max(maxLen, int(c&countMask))
	}

	var ss []string
	ss = append(ss, "{")
	if len(pe.chunks) > 0 {
		ss = append(ss, "\tchunks: {")
		for i, c := range pe.chunks {
			ss = append(ss, fmt.Sprintf("\t\t%s:  %s,",
				padBase10(i, 3),
				padBase2(uint(c>>countBits), uint(c&countMask), maxLen),
			))
		}
		ss = append(ss, "\t},")
	}
	ss = append(ss, fmt.Sprintf("\tchunkMask: %b,", pe.chunkMask))
	ss = append(ss, fmt.Sprintf("\tNumSyms:   %d,", pe.NumSyms))
	ss = append(ss, "}")
	return strings.Join(ss, "\n")
}
