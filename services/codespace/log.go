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
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	codespacev1 "gitea.dev/codespace-proto-go/codespace/v1"
	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/models/dbfs"
	"gitea.dev/modules/globallock"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"
)

const (
	codespaceLogDBFSPrefix             = "codespace_log/"
	codespaceLogTruncationMessage      = "Codespace log output was truncated because the log size limit was reached."
	codespaceLogMaxTimestampPadding    = int64(len("[] \n") + len(time.RFC3339Nano))
	codespaceLogMaxLineSize            = int64(64 * 1024)
	codespaceLogInternalSummaryReserve = int64(64 * 1024)
)

// LogReadMaxBytes is the maximum bytes returned by one Codespace log page.
const LogReadMaxBytes = int64(512 * 1024)

var (
	codespaceLogTokenPattern         = regexp.MustCompile(`\bgcs_[0-9a-f]{64}\b`)
	codespaceLogAuthorizationPattern = regexp.MustCompile(`(?i)(authorization:\s*(?:bearer|basic)\s+)[^\s]+`)
	codespaceLogBearerBasicPattern   = regexp.MustCompile(`(?i)\b((?:bearer|basic)\s+)[A-Za-z0-9._~+/=-]+`)
	codespaceLogURLUserinfoPattern   = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://)[^/@\s]+@`)
	codespaceLogURLTokenPattern      = regexp.MustCompile(`(?i)([?&](?:access_token|auth_token|private_token|token)=)[^&#\s]+`)

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

// UpdateLogOptions identifies one log append request.
type UpdateLogOptions struct {
	CodespaceUUID     string
	OperationRVersion int64
	Offset            int64
	Lines             []*codespacev1.LogLine
}

// ReadLogOptions identifies one user-facing log page request.
type ReadLogOptions struct {
	UserID        int64
	CodespaceUUID string
	Offset        int64
	Limit         int64
}

// ReadLogLine contains one parsed user-facing log line.
type ReadLogLine struct {
	Timestamp         float64 `json:"timestamp"`
	Message           string  `json:"message"`
	TimestampUnixNano int64   `json:"-"`
}

// Encoded returns the canonical DBFS representation of the line.
func (line ReadLogLine) Encoded() string {
	return encodeLogLine(line.TimestampUnixNano, line.Message)
}

// ReadLogResult contains one byte-offset based log page.
type ReadLogResult struct {
	Offset          int64         `json:"offset"`
	NextOffset      int64         `json:"next_offset"`
	EOF             bool          `json:"eof"`
	OperationActive bool          `json:"operation_active"`
	Lines           []ReadLogLine `json:"lines"`
	Truncated       bool          `json:"truncated"`
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
func UpdateLog(ctx context.Context, manager *codespace_model.Manager, opts UpdateLogOptions) (*codespacev1.UpdateLogResponse, error) {
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
			logFilename := codespaceLogFilename(codespace.UUID)
			if opts.Offset > codespace.LogSize {
				return &LogOffsetError{Err: ErrUpdateLogOffsetGap, CurrentOffset: codespace.LogSize}
			}
			if opts.Offset < codespace.LogSize {
				ok, err := logReplayMatches(ctx, logFilename, opts.Offset, encoded)
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
			hasTruncationSummary, err := codespaceLogHasTruncationSummary(ctx, logFilename, codespace.LogSize)
			if err != nil {
				return err
			}
			if hasTruncationSummary {
				nextOffset = codespace.LogSize
				sizeExceeded = true
				return nil
			}
			if codespace.LogSize >= codespaceLogOrdinaryLimit() || codespace.LogSize+int64(len(encoded)) > codespaceLogOrdinaryLimit() {
				truncation, err := encodeLogLines([]*codespacev1.LogLine{{
					TimestampUnixNano: time.Now().UnixNano(),
					Message:           codespaceLogTruncationMessage,
				}})
				if err != nil {
					return err
				}
				if codespace.LogSize+int64(len(truncation)) > setting.Codespace.LogMaxSize {
					return ErrUpdateLogSizeExceeded
				}
				if err := appendEncodedLogLines(ctx, codespace, truncation); err != nil {
					return err
				}
				nextOffset = codespace.LogSize
				sizeExceeded = true
				return nil
			}
			if err := appendEncodedLogLines(ctx, codespace, encoded); err != nil {
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
	return &codespacev1.UpdateLogResponse{NextOffset: nextOffset}, nil
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
	if opts.Offset > codespace.LogSize {
		return nil, &LogOffsetError{Err: ErrReadLogOffsetConflict, CurrentOffset: codespace.LogSize}
	}
	if opts.Offset == codespace.LogSize {
		return &ReadLogResult{Offset: opts.Offset, NextOffset: opts.Offset, EOF: true, OperationActive: hasActiveOperation(codespace), Lines: []ReadLogLine{}}, nil
	}
	lines, nextOffset, eof, truncated, err := readLogLines(ctx, codespaceLogFilename(codespace.UUID), opts.Offset, codespace.LogSize, opts.Limit)
	if err != nil {
		return nil, err
	}
	return &ReadLogResult{
		Offset:          opts.Offset,
		NextOffset:      nextOffset,
		EOF:             eof,
		OperationActive: hasActiveOperation(codespace),
		Lines:           lines,
		Truncated:       truncated,
	}, nil
}

func encodeLogLines(lines []*codespacev1.LogLine) ([]byte, error) {
	var buf bytes.Buffer
	for _, line := range lines {
		if line == nil {
			return nil, errors.New("log line is required")
		}
		if line.GetTimestampUnixNano() <= 0 {
			return nil, errors.New("log timestamp must be positive")
		}
		if !utf8.ValidString(line.GetMessage()) {
			return nil, errors.New("log message must be valid UTF-8")
		}
		if strings.ContainsAny(line.GetMessage(), "\r\n") {
			return nil, errors.New("log message must not contain newline")
		}
		if int64(len(line.GetMessage())) > codespaceLogMaxLineSize {
			return nil, errors.New("log message exceeds maximum line size")
		}
		message := line.GetMessage()
		message = codespaceLogTokenPattern.ReplaceAllString(message, "[redacted]")
		message = codespaceLogAuthorizationPattern.ReplaceAllString(message, "${1}[redacted]")
		message = codespaceLogBearerBasicPattern.ReplaceAllString(message, "${1}[redacted]")
		message = codespaceLogURLUserinfoPattern.ReplaceAllString(message, "${1}[redacted]@")
		message = codespaceLogURLTokenPattern.ReplaceAllString(message, "${1}[redacted]")
		message = strings.Map(func(r rune) rune {
			if r < 0x20 && r != '\t' || r == 0x7f {
				return -1
			}
			return r
		}, message)
		if int64(len(message)) > codespaceLogMaxLineSize {
			return nil, errors.New("log message exceeds maximum line size")
		}
		encoded := encodeLogLine(line.GetTimestampUnixNano(), message)
		if int64(len(encoded)) > codespaceLogMaxLineSize+codespaceLogMaxTimestampPadding {
			return nil, errors.New("encoded log line exceeds maximum line size")
		}
		buf.WriteString(encoded)
	}
	return buf.Bytes(), nil
}

func encodeLogLine(timestampUnixNano int64, message string) string {
	return fmt.Sprintf("[%s] %s\n", time.Unix(0, timestampUnixNano).UTC().Format(time.RFC3339Nano), message)
}

func appendEncodedLogLines(ctx context.Context, codespace *codespace_model.Codespace, encoded []byte) error {
	if len(encoded) == 0 {
		return nil
	}
	if err := appendLogBytes(ctx, codespaceLogFilename(codespace.UUID), codespace.LogSize, encoded); err != nil {
		return err
	}
	codespace.LogSize += int64(len(encoded))
	_, err := db.GetEngine(ctx).ID(codespace.UUID).Cols("log_size").Update(codespace)
	return err
}

func appendInternalStateSummary(ctx context.Context, summary *internalStateSummary) {
	// Diagnostic summaries run after the lifecycle commit so logging failure cannot roll back an accepted state transition.
	if summary == nil || summary.Message == "" {
		return
	}
	err := globallock.LockAndDo(ctx, updateLogLockKey(summary.CodespaceUUID), func(ctx context.Context) error {
		return db.WithTx(ctx, func(ctx context.Context) error {
			codespace := new(codespace_model.Codespace)
			has, err := db.GetEngine(ctx).ID(summary.CodespaceUUID).Get(codespace)
			if err != nil || !has {
				return err
			}
			encoded, err := encodeLogLines([]*codespacev1.LogLine{{
				TimestampUnixNano: time.Now().UnixNano(),
				Message:           summary.Message,
			}})
			if err != nil {
				return err
			}
			if codespace.LogSize+int64(len(encoded)) > setting.Codespace.LogMaxSize {
				return ErrUpdateLogSizeExceeded
			}
			return appendEncodedLogLines(ctx, codespace, encoded)
		})
	})
	if err != nil {
		log.Warn("failed to write codespace internal state summary for %s: %v", summary.CodespaceUUID, err)
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

func readLogLines(ctx context.Context, filename string, offset, logSize, limit int64) ([]ReadLogLine, int64, bool, bool, error) {
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
	lines := make([]ReadLogLine, 0)
	nextOffset := offset
	readBytes := int64(0)
	for nextOffset < logSize {
		line, readErr := reader.ReadString('\n')
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			return nil, 0, false, false, readErr
		}
		if line == "" && errors.Is(readErr, io.EOF) {
			break
		}
		lineLen := int64(len(line))
		if lineLen > LogReadMaxBytes {
			return nil, 0, false, false, errors.New("codespace log line exceeds read limit")
		}
		if len(lines) > 0 && readBytes+lineLen > limit {
			return lines, nextOffset, false, true, nil
		}
		parsed, parseErr := parseEncodedLogLine(line)
		if parseErr != nil {
			return nil, 0, false, false, parseErr
		}
		lines = append(lines, parsed)
		nextOffset += lineLen
		readBytes += lineLen
		if readBytes >= limit {
			return lines, nextOffset, nextOffset >= logSize, nextOffset < logSize, nil
		}
	}
	return lines, nextOffset, nextOffset >= logSize, false, nil
}

func parseEncodedLogLine(line string) (ReadLogLine, error) {
	if !strings.HasPrefix(line, "[") || !strings.HasSuffix(line, "\n") {
		return ReadLogLine{}, errors.New("invalid codespace log line")
	}
	separator := strings.Index(line, "] ")
	if separator < 0 {
		return ReadLogLine{}, errors.New("invalid codespace log timestamp separator")
	}
	timestamp, err := time.Parse(time.RFC3339Nano, line[1:separator])
	if err != nil {
		return ReadLogLine{}, fmt.Errorf("parse codespace log timestamp: %w", err)
	}
	return ReadLogLine{
		Timestamp:         float64(timestamp.UnixNano()) / float64(time.Second),
		Message:           line[separator+2 : len(line)-1],
		TimestampUnixNano: timestamp.UnixNano(),
	}, nil
}

func codespaceLogOrdinaryLimit() int64 {
	return setting.Codespace.LogMaxSize - codespaceLogInternalSummaryReserve
}

func updateLogLockKey(codespaceUUID string) string {
	return "codespace_log_" + codespaceUUID
}

func codespaceLogFilename(codespaceUUID string) string {
	return codespaceUUID + ".log"
}

func deleteCodespaceLog(ctx context.Context, codespaceUUID string) error {
	if err := dbfs.Remove(ctx, codespaceLogDBFSPrefix+codespaceLogFilename(codespaceUUID)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
