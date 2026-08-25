// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

var bytesSizeTable = map[string]uint64{
	"": 1, "b": 1,
	"k": 1e3, "kb": 1e3, "ki": 1 << 10, "kib": 1 << 10,
	"m": 1e6, "mb": 1e6, "mi": 1 << 20, "mib": 1 << 20,
	"g": 1e9, "gb": 1e9, "gi": 1 << 30, "gib": 1 << 30,
	"t": 1e12, "tb": 1e12, "ti": 1 << 40, "tib": 1 << 40,
	"p": 1e15, "pb": 1e15, "pi": 1 << 50, "pib": 1 << 50,
	"e": 1e18, "eb": 1e18, "ei": 1 << 60, "eib": 1 << 60,
}

func FormatBytes(size int64) string {
	if size < 10 {
		return fmt.Sprintf("%d B", size)
	}
	exponent := math.Floor(math.Log(float64(size)) / math.Log(1024))
	value := math.Floor(float64(size)/math.Pow(1024, exponent)*10+0.5) / 10
	unit := [...]string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}[int(exponent)]
	return fmt.Sprintf(Iif(value < 10, "%.1f %s", "%.0f %s"), value, unit)
}

func ParseBytes(value string) (uint64, error) {
	lastDigit := strings.IndexFunc(value, func(r rune) bool {
		return !unicode.IsDigit(r) && r != '.' && r != ','
	})
	if lastDigit < 0 {
		lastDigit = len(value)
	}

	parsed, err := strconv.ParseFloat(strings.ReplaceAll(value[:lastDigit], ",", ""), 64)
	if err != nil {
		return 0, err
	}

	suffix := strings.ToLower(strings.TrimSpace(value[lastDigit:]))
	multiplier, ok := bytesSizeTable[suffix]
	if !ok {
		return 0, fmt.Errorf("unhandled size name: %v", suffix)
	}
	parsed *= float64(multiplier)
	if parsed >= math.MaxUint64 {
		return 0, fmt.Errorf("too large: %v", value)
	}
	return uint64(parsed), nil
}
