// Copyright 2022 Gitea. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

var (
	// Time estimate match regex
	rTimeEstimateOnlyHours = regexp.MustCompile(`^([\d]+)$`)
	rTimeEstimateWeeks     = regexp.MustCompile(`([\d]+)w`)
	rTimeEstimateDays      = regexp.MustCompile(`([\d]+)d`)
	rTimeEstimateHours     = regexp.MustCompile(`([\d]+)h`)
	rTimeEstimateMinutes   = regexp.MustCompile(`([\d]+)m`)
)

// TimeEstimateFromStr returns time estimate in seconds from formatted string
func TimeEstimateFromStr(timeStr string) int64 {
	timeTotal := 0

	// If single number entered, assume hours
	timeStrMatches := rTimeEstimateOnlyHours.FindStringSubmatch(timeStr)
	if len(timeStrMatches) > 0 {
		raw, _ := strconv.Atoi(timeStrMatches[1])
		timeTotal += raw * (60 * 60)
	} else {
		// Find time weeks
		timeStrMatches = rTimeEstimateWeeks.FindStringSubmatch(timeStr)
		if len(timeStrMatches) > 0 {
			raw, _ := strconv.Atoi(timeStrMatches[1])
			timeTotal += raw * (60 * 60 * 24 * 7)
		}

		// Find time days
		timeStrMatches = rTimeEstimateDays.FindStringSubmatch(timeStr)
		if len(timeStrMatches) > 0 {
			raw, _ := strconv.Atoi(timeStrMatches[1])
			timeTotal += raw * (60 * 60 * 24)
		}

		// Find time hours
		timeStrMatches = rTimeEstimateHours.FindStringSubmatch(timeStr)
		if len(timeStrMatches) > 0 {
			raw, _ := strconv.Atoi(timeStrMatches[1])
			timeTotal += raw * (60 * 60)
		}

		// Find time minutes
		timeStrMatches = rTimeEstimateMinutes.FindStringSubmatch(timeStr)
		if len(timeStrMatches) > 0 {
			raw, _ := strconv.Atoi(timeStrMatches[1])
			timeTotal += raw * (60)
		}
	}

	return int64(timeTotal)
}

// TimeEstimateStr returns formatted time estimate string from seconds (e.g. "2w 4d 12h 5m")
func TimeEstimateToStr(amount int64) string {
	var timeParts []string

	timeSeconds := float64(amount)

	// Format weeks
	weeks := math.Floor(timeSeconds / (60 * 60 * 24 * 7))
	if weeks > 0 {
		timeParts = append(timeParts, fmt.Sprintf("%dw", int64(weeks)))
	}
	timeSeconds -= weeks * (60 * 60 * 24 * 7)

	// Format days
	days := math.Floor(timeSeconds / (60 * 60 * 24))
	if days > 0 {
		timeParts = append(timeParts, fmt.Sprintf("%dd", int64(days)))
	}
	timeSeconds -= days * (60 * 60 * 24)

	// Format hours
	hours := math.Floor(timeSeconds / (60 * 60))
	if hours > 0 {
		timeParts = append(timeParts, fmt.Sprintf("%dh", int64(hours)))
	}
	timeSeconds -= hours * (60 * 60)

	// Format minutes
	minutes := math.Floor(timeSeconds / (60))
	if minutes > 0 {
		timeParts = append(timeParts, fmt.Sprintf("%dm", int64(minutes)))
	}

	return strings.Join(timeParts, " ")
}
