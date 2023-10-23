// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package timezone

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/modules/options"
)

var (
	zoneListCache   TimeZoneList
	lastCacheUpdate int64
)

const (
	unixDay = 60 * 60 * 24
)

type TimeZone struct {
	Name   string
	Offset int64
}

// Returns the current time in this timezone
func (timeZone *TimeZone) CurrentTime() time.Time {
	return time.Now().UTC().Add(time.Second * time.Duration(timeZone.Offset))
}

// returns the value of current time as RFC3339 string but without the timezone
func (timeZone *TimeZone) CurrentTimeString() string {
	return strings.TrimSuffix(timeZone.CurrentTime().Format(time.RFC3339), "Z")
}

// Returns the offset as printable string e.g. +01:00
func (timeZone *TimeZone) OffsetString() string {
	offsetCopy := timeZone.Offset

	// We don't want it to be negative
	if offsetCopy < 0 {
		offsetCopy = offsetCopy * -1
	}

	offsetString := time.Unix(offsetCopy, 0).UTC().Format("15:04")

	if timeZone.Offset > 0 {
		return fmt.Sprintf("+%s", offsetString)
	} else if timeZone.Offset < 0 {
		return fmt.Sprintf("-%s", offsetString)
	}

	return offsetString
}

// Returns if the timezone has any data
func (timeZone *TimeZone) IsEmpty() bool {
	return timeZone.Name == ""
}

type TimeZoneList []*TimeZone

// Returns the timezone with the given name. Retruns nil, if the timezone was not found
func (zoneList TimeZoneList) GetTimeZoneByName(name string) *TimeZone {
	for _, zone := range zoneList {
		if zone.Name == name {
			return zone
		}
	}

	return nil
}

// Same as GetTimeZoneByName but returns the value of GetDefaultTimeZone instead of nil if the timezone is not found
func (zoneList TimeZoneList) GetTimeZoneByNameDefault(name string) *TimeZone {
	zone := zoneList.GetTimeZoneByName(name)

	if zone == nil {
		return GetDefaultTimeZone()
	}

	return zone
}

// Returns a list of all known timezones
func GetTimeZoneList() (TimeZoneList, error) {
	// If we don't have a cache or the cache is older than 24 hours, we need to renew the cache
	if zoneListCache == nil || (lastCacheUpdate+unixDay) <= time.Now().UTC().Unix() {
		err := UpdateTimeZoneListCache()
		if err != nil {
			return nil, err
		}
	}

	return zoneListCache, nil
}

// Update the timezone cache
func UpdateTimeZoneListCache() error {
	content, err := options.AssetFS().ReadFile("timezones.csv")
	if err != nil {
		return fmt.Errorf("ReadFile: %v", err)
	}

	csvReader := csv.NewReader(bytes.NewBuffer(content))
	data, err := csvReader.ReadAll()
	if err != nil {
		return err
	}

	const nameColumn = 0
	const timeStartColumn = 3
	const offsetColumn = 4

	currentTime := time.Now().UTC().Unix()
	currentZone := new(TimeZone)
	lastLoopName := ""
	currentName := ""

	zoneList := make(TimeZoneList, 0)

	for _, row := range data {
		if currentName == row[nameColumn] {
			continue
		}

		// If the last timezone was not added, we add it here
		if lastLoopName != "" && lastLoopName != row[nameColumn] && !currentZone.IsEmpty() {
			zoneList = append(zoneList, currentZone)
			currentZone = new(TimeZone)
		}

		lastLoopName = row[nameColumn]

		timeStart, err := strconv.ParseInt(row[timeStartColumn], 10, 64)
		if err != nil {
			return fmt.Errorf("Convert %s to int64: %v", row[timeStartColumn], err)
		}

		offset, err := strconv.ParseInt(row[offsetColumn], 10, 64)
		if err != nil {
			return fmt.Errorf("Convert %s to int64: %v", row[offsetColumn], err)
		}

		if currentTime < timeStart {
			// If the start time of the current row is higher than the last, we use the timezone from the last run
			currentName = row[nameColumn]
			zoneList = append(zoneList, currentZone)
			currentZone = new(TimeZone)
		} else {
			currentZone = &TimeZone{Name: row[nameColumn], Offset: offset}
		}
	}

	lastCacheUpdate = time.Now().UTC().Unix()
	zoneListCache = zoneList

	return nil
}

// Returns the timezone for Europe/London
func GetDefaultTimeZone() *TimeZone {
	return &TimeZone{
		Name:   "Europe/London",
		Offset: 0,
	}
}
