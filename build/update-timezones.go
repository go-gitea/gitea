// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

//go:build ignore

package main

import (
	"archive/zip"
	"compress/gzip"
	"encoding/csv"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strconv"
	"time"

	"code.gitea.io/gitea/modules/util"
)

type TimezoneInfo struct {
	Name      string
	TimeStart int64
	Offset    int64
}

func (timeZone *TimezoneInfo) Write(csvWriter *csv.Writer) {
	csvWriter.Write([]string{timeZone.Name, strconv.FormatInt(timeZone.TimeStart, 10), strconv.FormatInt(timeZone.Offset, 10)})
}

func writeTimezones(csvWriter *csv.Writer, timezoneList []*TimezoneInfo) {
	// We don't want data for timezones that are more than 5 years old
	startTime := time.Now().UTC().AddDate(-5, 0, 0).Unix()

	// We don't want data for timezones that are more than 30 years in the future
	endTime := time.Now().UTC().AddDate(30, 0, 0).Unix()

	hasWritten := false

	for {
		if len(timezoneList) == 0 {
			return
		}

		if !hasWritten && len(timezoneList) == 1 {
			// If we only have one element left, but nothing written, we need to write it
			timezoneList[0].Write(csvWriter)
			return
		}

		if timezoneList[0].TimeStart >= startTime && timezoneList[0].TimeStart <= endTime {
			timezoneList[0].Write(csvWriter)
			hasWritten = true
		}

		// Remove the first elemnt from the slice
		timezoneList = slices.Delete(timezoneList, 0, 1)
	}
}

func main() {
	const (
		url      = "https://timezonedb.com/files/TimeZoneDB.csv.zip"
		dest     = "options/timezones.csv.gz"
		prefix   = "timezone-archive"
		filename = "time_zone.csv"
	)

	file, err := os.CreateTemp(os.TempDir(), prefix)
	if err != nil {
		log.Fatalf("Failed to create temp file. %s", err)
	}

	defer util.Remove(file.Name())

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		log.Fatalf("Failed to download archive. %s", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Failed to download archive. %s", err)
	}

	defer resp.Body.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		log.Fatalf("Failed to copy archive to file. %s", err)
	}

	file.Close()

	zf, err := zip.OpenReader(file.Name())
	if err != nil {
		log.Fatalf("Failed to open archive. %s", err)
	}
	defer zf.Close()

	fi, err := zf.Open(filename)
	if err != nil {
		log.Fatalf("Failed to open file in archive. %s", err)
	}
	defer fi.Close()

	fo, err := os.Create(dest)
	if err != nil {
		log.Fatalf("Failed to create file. %s", err)
	}
	defer fo.Close()

	zo := gzip.NewWriter(fo)
	defer zo.Close()

	const nameColumn = 0
	const timeStartColumn = 3
	const offsetColumn = 4

	csvReader := csv.NewReader(fi)
	data, err := csvReader.ReadAll()
	if err != nil {
		log.Fatalf("Failed to create CSV Reader. %s", err)
	}

	csvWriter := csv.NewWriter(zo)
	defer csvWriter.Flush()

	timezoneList := make([]*TimezoneInfo, 0)
	currentName := ""

	for _, row := range data {
		if currentName != "" && currentName != row[nameColumn] {
			writeTimezones(csvWriter, timezoneList)
			timezoneList = make([]*TimezoneInfo, 0)
		}

		currentName = row[nameColumn]

		timeStart, err := strconv.ParseInt(row[timeStartColumn], 10, 64)
		if err != nil {
			log.Fatalf("Convert %s to int64: %s", row[timeStartColumn], err)
		}

		offset, err := strconv.ParseInt(row[offsetColumn], 10, 64)
		if err != nil {
			log.Fatalf("Convert %s to int64: %s", row[offsetColumn], err)
		}

		timezoneList = append(timezoneList, &TimezoneInfo{Name: row[nameColumn], TimeStart: timeStart, Offset: offset})
	}
}
