// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"code.gitea.io/gitea/models/dbfs"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	MaxLineSize = 64 * 1024
	DBFSPrefix  = "bots_tasks/"

	timeFormat     = time.RFC3339Nano
	defaultBufSize = 64 * 1024
)

func WriteLogs(ctx context.Context, filename string, offset int64, rows []*runnerv1.LogRow) ([]int, error) {
	name := DBFSPrefix + filename
	f, err := dbfs.OpenFile(ctx, name, os.O_WRONLY|os.O_CREATE)
	if err != nil {
		return nil, fmt.Errorf("dbfs OpenFile %q: %w", name, err)
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("dbfs Seek %q: %w", name, err)
	}

	writer := bufio.NewWriterSize(f, defaultBufSize)

	ns := make([]int, 0, len(rows))
	for _, row := range rows {
		n, err := writer.WriteString(FormatLog(row.Time.AsTime(), row.Content) + "\n")
		if err != nil {
			return nil, err
		}
		ns = append(ns, n)
	}

	if err := writer.Flush(); err != nil {
		return nil, err
	}
	return ns, nil
}

func ReadLogs(ctx context.Context, filename string, offset int64, limit int64) ([]*runnerv1.LogRow, error) {
	name := DBFSPrefix + filename
	f, err := dbfs.Open(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("dbfs Open %q: %w", name, err)
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("dbfs Seek %q: %w", name, err)
	}

	scanner := bufio.NewScanner(f)
	maxLineSize := len(timeFormat) + MaxLineSize + 1
	scanner.Buffer(make([]byte, maxLineSize), maxLineSize)

	var rows []*runnerv1.LogRow
	for scanner.Scan() && (int64(len(rows)) < limit || limit < 0) {
		t, c, err := ParseLog(scanner.Text())
		if err != nil {
			return nil, fmt.Errorf("parse log %q: %w", scanner.Text(), err)
		}
		rows = append(rows, &runnerv1.LogRow{
			Time:    timestamppb.New(t),
			Content: c,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan: %w", err)
	}

	return rows, nil
}

func FormatLog(timestamp time.Time, content string) string {
	// Content shouldn't contain new line, it will break log indexes, other control chars are safe.
	content = strings.ReplaceAll(content, "\n", `\n`)
	if len(content) > MaxLineSize {
		content = content[:MaxLineSize]
	}
	return fmt.Sprintf("%s %s", timestamp.UTC().Format(timeFormat), content)
}

func ParseLog(in string) (timestamp time.Time, content string, err error) {
	index := strings.IndexRune(in, ' ')
	if index < 0 {
		err = fmt.Errorf("invalid log: %q", in)
		return
	}
	timestamp, err = time.Parse(timeFormat, in[:index])
	if err != nil {
		return
	}
	content = in[index+1:]
	return
}
