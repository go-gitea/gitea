// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode/utf8"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/dbfs"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
)

const (
	codespaceLogDBFSPrefix          = "codespace_log/"
	codespaceLogTruncationMessage   = "Codespace log output was truncated because the log size limit was reached."
	codespaceLogMaxTimestampPadding = int64(len("[] \n") + len(time.RFC3339Nano))
	codespaceLogMaxLineSize         = int64(64 * 1024)
	codespaceLogFinalSummaryReserve = int64(64 * 1024)
)

// LogReadMaxBytes is the maximum bytes returned by one Codespace log page.
const LogReadMaxBytes = int64(512 * 1024)

var (
	// ErrUpdateLogNotFound is returned when the Codespace no longer exists.
	ErrUpdateLogNotFound = errors.New("codespace not found")
	// ErrUpdateLogStaleOperation is returned when the request no longer matches the active operation.
	ErrUpdateLogStaleOperation = errors.New("codespace log operation is stale")
	// ErrUpdateLogOffsetConflict is returned when the request offset overlaps different existing bytes.
	ErrUpdateLogOffsetConflict = errors.New("codespace log offset conflict")
	// ErrUpdateLogOffsetGap is returned when the request offset is beyond current file end.
	ErrUpdateLogOffsetGap = errors.New("codespace log offset gap")
	// ErrUpdateLogSizeExceeded is returned when ordinary log bytes have reached their reserved limit.
	ErrUpdateLogSizeExceeded = errors.New("codespace log size exceeded")
	// ErrReadLogNotFound is returned when the Codespace no longer exists.
	ErrReadLogNotFound = errors.New("codespace log codespace not found")
	// ErrReadLogPermissionDenied is returned when the user cannot read this Codespace log.
	ErrReadLogPermissionDenied = errors.New("codespace log permission denied")
	// ErrReadLogInvalidArgument is returned when read options are outside the supported range.
	ErrReadLogInvalidArgument = errors.New("codespace log invalid argument")
	// ErrReadLogOffsetConflict is returned when the requested offset is not a physical line boundary.
	ErrReadLogOffsetConflict = errors.New("codespace log read offset conflict")
)

// LogLine contains one structured Manager log line.
type LogLine struct {
	TimestampUnixNano int64
	Message           string
}

// UpdateLogOptions identifies one log append request.
type UpdateLogOptions struct {
	CodespaceUUID     string
	OperationRVersion int64
	Offset            int64
	Lines             []LogLine
}

// UpdateLogResult contains the server-authoritative next byte offset.
type UpdateLogResult struct {
	NextOffset int64
}

// ReadLogOptions identifies one user-facing log page request.
type ReadLogOptions struct {
	UserID        int64
	CodespaceUUID string
	Offset        int64
	Limit         int64
}

// ReadLogResult contains one byte-offset based log page.
type ReadLogResult struct {
	Offset     int64    `json:"offset"`
	NextOffset int64    `json:"next_offset"`
	EOF        bool     `json:"eof"`
	Lines      []string `json:"lines"`
	Truncated  bool     `json:"truncated"`
}

type internalStateSummary struct {
	CodespaceUUID string
	Message       string
}

// LogOffsetError carries the current server-authoritative log offset.
type LogOffsetError struct {
	Err           error
	CurrentOffset int64
}

func (e *LogOffsetError) Error() string {
	return e.Err.Error()
}

func (e *LogOffsetError) Unwrap() error {
	return e.Err
}

// UpdateLog appends Manager operation logs to the Codespace DBFS log file.
func UpdateLog(ctx context.Context, manager *codespace_model.Manager, opts UpdateLogOptions) (*UpdateLogResult, error) {
	if manager == nil || manager.ID <= 0 {
		return nil, errors.New("manager is required")
	}
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return nil, err
	}
	if opts.OperationRVersion <= 0 {
		return nil, errors.New("operation_rversion must be positive")
	}
	if opts.Offset < 0 {
		return nil, errors.New("offset must not be negative")
	}
	encoded, err := encodeLogLines(opts.Lines)
	if err != nil {
		return nil, err
	}

	var nextOffset int64
	var sizeExceeded bool
	err = globallock.LockAndDo(ctx, updateLogLockKey(opts.CodespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			codespace := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).ID(opts.CodespaceUUID).Get(codespace)
			if err != nil {
				return err
			}
			if !has {
				return ErrUpdateLogNotFound
			}
			if !isCurrentRunningOperation(codespace, manager.ID, opts.OperationRVersion) {
				return ErrUpdateLogStaleOperation
			}
			if codespace.LogFilename == "" {
				codespace.LogFilename = opts.CodespaceUUID + ".log"
			}
			if opts.Offset > codespace.LogSize {
				return &LogOffsetError{Err: ErrUpdateLogOffsetGap, CurrentOffset: codespace.LogSize}
			}
			if opts.Offset < codespace.LogSize {
				ok, err := logReplayMatches(ctx, codespace.LogFilename, opts.Offset, encoded)
				if err != nil {
					return err
				}
				if !ok {
					return &LogOffsetError{Err: ErrUpdateLogOffsetConflict, CurrentOffset: codespace.LogSize}
				}
				nextOffset = codespace.LogSize
				return nil
			}
			if len(encoded) == 0 {
				nextOffset = codespace.LogSize
				return nil
			}
			hasTruncationSummary, err := codespaceLogHasTruncationSummary(ctx, codespace.LogFilename, codespace.LogSize)
			if err != nil {
				return err
			}
			if hasTruncationSummary {
				nextOffset = codespace.LogSize
				sizeExceeded = true
				return nil
			}
			if codespace.LogSize >= codespaceLogOrdinaryLimit() || codespace.LogSize+int64(len(encoded)) > codespaceLogOrdinaryLimit() {
				if err := appendLogTruncationSummary(ctx, codespace); err != nil {
					return err
				}
				nextOffset = codespace.LogSize
				sizeExceeded = true
				return nil
			}
			if err := appendEncodedLogLines(ctx, codespace, encoded, int64(len(opts.Lines))); err != nil {
				return err
			}
			nextOffset = codespace.LogSize
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	if sizeExceeded {
		return nil, ErrUpdateLogSizeExceeded
	}
	return &UpdateLogResult{NextOffset: nextOffset}, nil
}

// ReadLog reads one complete-line page from the Codespace DBFS log file.
func ReadLog(ctx context.Context, opts ReadLogOptions) (*ReadLogResult, error) {
	if opts.UserID <= 0 {
		return nil, fmt.Errorf("%w: user_id must be positive", ErrReadLogInvalidArgument)
	}
	if err := codespace_model.ValidateUUID(opts.CodespaceUUID); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrReadLogInvalidArgument, err)
	}
	if opts.Offset < 0 {
		return nil, &LogOffsetError{Err: ErrReadLogInvalidArgument, CurrentOffset: 0}
	}
	if opts.Limit <= 0 || opts.Limit > LogReadMaxBytes {
		return nil, fmt.Errorf("%w: limit must be between 1 and %d", ErrReadLogInvalidArgument, LogReadMaxBytes)
	}

	codespace := new(codespace_model.Codespace)
	has, err := db.GetEngine(ctx).ID(opts.CodespaceUUID).Get(codespace)
	if err != nil {
		return nil, err
	}
	if !has {
		return nil, ErrReadLogNotFound
	}
	if codespace.UserID != opts.UserID {
		return nil, ErrReadLogPermissionDenied
	}
	if codespace.LogSize == 0 {
		if opts.Offset > 0 {
			return nil, &LogOffsetError{Err: ErrReadLogOffsetConflict, CurrentOffset: 0}
		}
		return &ReadLogResult{Offset: opts.Offset, NextOffset: 0, EOF: true, Lines: []string{}}, nil
	}
	if opts.Offset > codespace.LogSize {
		return nil, &LogOffsetError{Err: ErrReadLogOffsetConflict, CurrentOffset: codespace.LogSize}
	}
	if opts.Offset == codespace.LogSize {
		return &ReadLogResult{Offset: opts.Offset, NextOffset: opts.Offset, EOF: true, Lines: []string{}}, nil
	}
	if codespace.LogFilename == "" {
		return nil, errors.New("codespace log filename is empty")
	}

	lines, nextOffset, eof, truncated, err := readLogLines(ctx, codespace.LogFilename, opts.Offset, codespace.LogSize, opts.Limit)
	if err != nil {
		return nil, err
	}
	return &ReadLogResult{
		Offset:     opts.Offset,
		NextOffset: nextOffset,
		EOF:        eof,
		Lines:      lines,
		Truncated:  truncated,
	}, nil
}

func encodeLogLines(lines []LogLine) ([]byte, error) {
	var buf bytes.Buffer
	for _, line := range lines {
		if line.TimestampUnixNano <= 0 {
			return nil, errors.New("log timestamp must be positive")
		}
		if !utf8.ValidString(line.Message) {
			return nil, errors.New("log message must be valid UTF-8")
		}
		if strings.ContainsAny(line.Message, "\r\n") {
			return nil, errors.New("log message must not contain newline")
		}
		if int64(len(line.Message)) > codespaceLogMaxLineSize {
			return nil, errors.New("log message exceeds maximum line size")
		}
		encoded := fmt.Sprintf("[%s] %s\n", time.Unix(0, line.TimestampUnixNano).UTC().Format(time.RFC3339Nano), line.Message)
		if int64(len(encoded)) > codespaceLogMaxLineSize+codespaceLogMaxTimestampPadding {
			return nil, errors.New("encoded log line exceeds maximum line size")
		}
		buf.WriteString(encoded)
	}
	return buf.Bytes(), nil
}

func appendEncodedLogLines(ctx context.Context, codespace *codespace_model.Codespace, encoded []byte, lineCount int64) error {
	if len(encoded) == 0 {
		return nil
	}
	if codespace.LogFilename == "" {
		codespace.LogFilename = codespace.UUID + ".log"
	}
	if err := appendLogBytes(ctx, codespace.LogFilename, codespace.LogSize, encoded); err != nil {
		return err
	}
	codespace.LogSize += int64(len(encoded))
	codespace.LogLineCount += lineCount
	_, err := db.GetEngine(ctx).ID(codespace.UUID).Cols("log_filename", "log_size", "log_line_count").Update(codespace)
	return err
}

func appendLogTruncationSummary(ctx context.Context, codespace *codespace_model.Codespace) error {
	hasTruncationSummary, err := codespaceLogHasTruncationSummary(ctx, codespace.LogFilename, codespace.LogSize)
	if err != nil {
		return err
	}
	if hasTruncationSummary {
		return nil
	}
	encoded, err := encodeLogLines([]LogLine{{
		TimestampUnixNano: time.Now().UnixNano(),
		Message:           codespaceLogTruncationMessage,
	}})
	if err != nil {
		return err
	}
	if codespace.LogSize+int64(len(encoded)) > setting.Codespace.LogMaxSize {
		return ErrUpdateLogSizeExceeded
	}
	return appendEncodedLogLines(ctx, codespace, encoded, 1)
}

func appendInternalStateSummary(ctx context.Context, summary *internalStateSummary) {
	if summary == nil {
		return
	}
	if err := appendInternalLogSummary(ctx, summary); err != nil {
		log.Warn("failed to write codespace internal state summary for %s: %v", summary.CodespaceUUID, err)
	}
}

func appendInternalStateSummaries(ctx context.Context, summaries []*internalStateSummary) {
	for _, summary := range summaries {
		appendInternalStateSummary(ctx, summary)
	}
}

func appendInternalLogSummary(ctx context.Context, summary *internalStateSummary) error {
	if summary == nil || summary.Message == "" {
		return nil
	}
	return globallock.LockAndDo(ctx, updateLogLockKey(summary.CodespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			codespace := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).ID(summary.CodespaceUUID).Get(codespace)
			if err != nil || !has {
				return err
			}
			encoded, err := encodeLogLines([]LogLine{{
				TimestampUnixNano: time.Now().UnixNano(),
				Message:           summary.Message,
			}})
			if err != nil {
				return err
			}
			if codespace.LogSize+int64(len(encoded)) > setting.Codespace.LogMaxSize {
				return ErrUpdateLogSizeExceeded
			}
			return appendEncodedLogLines(ctx, codespace, encoded, 1)
		})
	})
}

func operationFinalSummary(codespace *codespace_model.Codespace, finalStatus, resultStatus string) *internalStateSummary {
	return &internalStateSummary{
		CodespaceUUID: codespace.UUID,
		Message: fmt.Sprintf("Gitea recorded operation %s#%d final %s as %s.",
			codespace.OperationType, codespace.OperationRVersion, finalStatus, resultStatus),
	}
}

func operationTimeoutSummary(codespace *codespace_model.Codespace, resultStatus string) *internalStateSummary {
	return &internalStateSummary{
		CodespaceUUID: codespace.UUID,
		Message: fmt.Sprintf("Gitea recorded operation %s#%d timeout as %s.",
			codespace.OperationType, codespace.OperationRVersion, resultStatus),
	}
}

func runtimeMissingSummary(codespace *codespace_model.Codespace) *internalStateSummary {
	return &internalStateSummary{
		CodespaceUUID: codespace.UUID,
		Message:       "Gitea recorded missing runtime as failed.",
	}
}

func runtimeTransitionSummary(codespace *codespace_model.Codespace, runtimeGeneration int64, targetStatus string) *internalStateSummary {
	return &internalStateSummary{
		CodespaceUUID: codespace.UUID,
		Message:       fmt.Sprintf("Gitea recorded runtime generation %d as %s.", runtimeGeneration, targetStatus),
	}
}

func codespaceLogHasTruncationSummary(ctx context.Context, filename string, logSize int64) (bool, error) {
	if filename == "" || logSize <= 0 {
		return false, nil
	}
	suffix := []byte(codespaceLogTruncationMessage + "\n")
	readSize := int64(len(suffix))
	if logSize < readSize {
		return false, nil
	}
	file, err := dbfs.Open(ctx, codespaceLogDBFSPrefix+filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	defer file.Close()
	if _, err := file.Seek(logSize-readSize, io.SeekStart); err != nil {
		return false, err
	}
	actual := make([]byte, readSize)
	if _, err := io.ReadFull(file, actual); err != nil {
		return false, err
	}
	return bytes.Equal(actual, suffix), nil
}

func logReplayMatches(ctx context.Context, filename string, offset int64, expected []byte) (bool, error) {
	if len(expected) == 0 {
		return true, nil
	}
	file, err := dbfs.Open(ctx, codespaceLogDBFSPrefix+filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		return false, err
	}
	defer file.Close()
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return false, err
	}
	actual := make([]byte, len(expected))
	n, err := io.ReadFull(file, actual)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return false, err
	}
	return n == len(expected) && bytes.Equal(actual, expected), nil
}

func appendLogBytes(ctx context.Context, filename string, offset int64, data []byte) error {
	flag := os.O_RDWR
	if offset == 0 {
		flag |= os.O_CREATE
	}
	file, err := dbfs.OpenFile(ctx, codespaceLogDBFSPrefix+filename, flag)
	if err != nil {
		return fmt.Errorf("open codespace log: %w", err)
	}
	defer file.Close()
	stat, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat codespace log: %w", err)
	}
	if stat.Size() != offset {
		return &LogOffsetError{Err: ErrUpdateLogOffsetConflict, CurrentOffset: stat.Size()}
	}
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return err
	}
	n, err := file.Write(data)
	if err != nil {
		return err
	}
	if n != len(data) {
		return io.ErrShortWrite
	}
	return nil
}

func readLogLines(ctx context.Context, filename string, offset, logSize, limit int64) ([]string, int64, bool, bool, error) {
	file, err := dbfs.Open(ctx, codespaceLogDBFSPrefix+filename)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, 0, false, false, ErrReadLogNotFound
		}
		return nil, 0, false, false, err
	}
	defer file.Close()
	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		return nil, 0, false, false, err
	}

	reader := bufio.NewReaderSize(io.LimitReader(file, logSize-offset), int(codespaceLogMaxLineSize))
	lines := make([]string, 0)
	nextOffset := offset
	readBytes := int64(0)
	for nextOffset < logSize {
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return nil, 0, false, false, err
		}
		if line == "" && errors.Is(err, io.EOF) {
			break
		}
		lineLen := int64(len(line))
		if lineLen > LogReadMaxBytes {
			return nil, 0, false, false, errors.New("codespace log line exceeds read limit")
		}
		if len(lines) > 0 && readBytes+lineLen > limit {
			return lines, nextOffset, false, true, nil
		}
		lines = append(lines, line)
		nextOffset += lineLen
		readBytes += lineLen
		if errors.Is(err, io.EOF) {
			break
		}
		if readBytes >= limit {
			return lines, nextOffset, nextOffset >= logSize, nextOffset < logSize, nil
		}
	}
	return lines, nextOffset, nextOffset >= logSize, false, nil
}

func codespaceLogOrdinaryLimit() int64 {
	return setting.Codespace.LogMaxSize - codespaceLogFinalSummaryReserve
}

func updateLogLockKey(codespaceUUID string) string {
	return "codespace_log_" + codespaceUUID
}

func deleteCodespaceLog(ctx context.Context, codespaceUUID string) error {
	if err := dbfs.Remove(ctx, codespaceLogDBFSPrefix+codespaceUUID+".log"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
