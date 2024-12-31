// Copyright 2024 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

type timeStrGlobalVarsType struct {
	units []struct {
		name string
		num  int64
	}
	re *regexp.Regexp
}

// When tracking working time, only hour/minute/second units are accurate and could be used.
// For other units like "day", it depends on "how many working hours in a day": 6 or 7 or 8?
// So at the moment, we only support hour/minute/second units.
// In the future, it could be some configurable options to help users
// to convert the working time to different units.

var timeStrGlobalVars = sync.OnceValue(func() *timeStrGlobalVarsType {
	v := &timeStrGlobalVarsType{}
	v.re = regexp.MustCompile(`(?i)(\d+)\s*([hms])`)
	v.units = []struct {
		name string
		num  int64
	}{
		{"h", 60 * 60},
		{"m", 60},
		{"s", 1},
	}
	return v
})

func TimeEstimateParse(timeStr string) (int64, error) {
	if timeStr == "" {
		return 0, nil
	}
	var total int64
	matches := timeStrGlobalVars().re.FindAllStringSubmatchIndex(timeStr, -1)
	if len(matches) == 0 {
		return 0, fmt.Errorf("invalid time string: %s", timeStr)
	}
	if matches[0][0] != 0 || matches[len(matches)-1][1] != len(timeStr) {
		return 0, fmt.Errorf("invalid time string: %s", timeStr)
	}
	for _, match := range matches {
		amount, err := strconv.ParseInt(timeStr[match[2]:match[3]], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid time string: %v", err)
		}
		unit := timeStr[match[4]:match[5]]
		found := false
		for _, u := range timeStrGlobalVars().units {
			if strings.ToLower(unit) == u.name {
				total += amount * u.num
				found = true
				break
			}
		}
		if !found {
			return 0, fmt.Errorf("invalid time unit: %s", unit)
		}
	}
	return total, nil
}

func TimeEstimateString(amount int64) string {
	var timeParts []string
	for _, u := range timeStrGlobalVars().units {
		if amount >= u.num {
			num := amount / u.num
			amount %= u.num
			timeParts = append(timeParts, fmt.Sprintf("%d%s", num, u.name))
		}
	}
	return strings.Join(timeParts, " ")
}
