// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package bots

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"code.gitea.io/gitea/models/dbfs"
	runnerv1 "gitea.com/gitea/proto-go/runner/v1"

	"google.golang.org/protobuf/types/known/timestamppb"
)

const defaultBufSize = 64 * 1024

const (
	StorageSchemaDBFS = "dbfs"
	StorageSchemaS2   = "s2"
	// ...
)

func WriteLogs(ctx context.Context, rawURL string, offset int64, rows []*runnerv1.LogRow) ([]int, error) {
	name, err := parseDBFSName(rawURL)
	if err != nil {
		return nil, err
	}
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

func ReadLogs(ctx context.Context, rawURL string, offset int64, limit int64) ([]*runnerv1.LogRow, error) {
	name, err := parseDBFSName(rawURL)
	if err != nil {
		return nil, err
	}

	f, err := dbfs.Open(ctx, name)
	if err != nil {
		return nil, fmt.Errorf("dbfs Open %q: %w", name, err)
	}
	defer f.Close()
	if _, err := f.Seek(offset, io.SeekStart); err != nil {
		return nil, fmt.Errorf("dbfs Seek %q: %w", name, err)
	}

	var rows []*runnerv1.LogRow
	scanner := bufio.NewScanner(f)
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

func parseDBFSName(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid url: %w", err)
	}
	if u.Scheme != StorageSchemaDBFS {
		return "", fmt.Errorf("%s supported only yet", StorageSchemaDBFS)
	}

	if u.Path == "" {
		return "", fmt.Errorf("empty path")
	}

	return u.Path, nil
}

func FormatLog(timestamp time.Time, content string) string {
	return fmt.Sprintf("%s %s", timestamp.UTC().Format(time.RFC3339Nano), strconv.Quote(content))
}

func ParseLog(in string) (timestamp time.Time, content string, err error) {
	index := strings.IndexRune(in, ' ')
	if index < 0 {
		err = fmt.Errorf("invalid log: %q", in)
		return
	}
	timestamp, err = time.Parse(time.RFC3339Nano, in[:index])
	if err != nil {
		return
	}
	content, err = strconv.Unquote(in[index+1:])
	return
}
