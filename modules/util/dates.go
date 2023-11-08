// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package util

import (
	"time"
)

const layout = time.DateOnly

func ListSundaysBetween(startStr, endStr string) ([]int64, error) {
	startDate, err := time.Parse(layout, startStr)
	if err != nil {
		return nil, err
	}

	endDate, err := time.Parse(layout, endStr)
	if err != nil {
		return nil, err
	}

	// Ensure the start date is a Sunday
	for startDate.Weekday() != time.Sunday {
		startDate = startDate.AddDate(0, 0, 1)
	}

	var sundays []int64

	// Iterate from start date to end date and find all Sundays
	for currentDate := startDate; currentDate.Before(endDate); currentDate = currentDate.AddDate(0, 0, 7) {
		sundays = append(sundays, currentDate.UnixMilli())
	}

	return sundays, nil
}

func FindLastSundayBeforeDate(dateStr string) (string, error) {
	date, err := time.Parse(layout, dateStr)
	if err != nil {
		return "", err
	}

	weekday := date.Weekday()
	daysToSubtract := int(weekday) - int(time.Sunday)
	if daysToSubtract < 0 {
		daysToSubtract += 7
	}

	lastSunday := date.AddDate(0, 0, -daysToSubtract)
	return lastSunday.Format(layout), nil
}

func FindFirstSundayAfterDate(dateStr string) (string, error) {
	date, err := time.Parse(layout, dateStr)
	if err != nil {
		return "", err
	}

	weekday := date.Weekday()
	daysToAdd := int(time.Sunday) - int(weekday)
	if daysToAdd <= 0 {
		daysToAdd += 7
	}

	firstSunday := date.AddDate(0, 0, daysToAdd)
	return firstSunday.Format(layout), nil
}
