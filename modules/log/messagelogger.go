// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"strings"
	"sync"

	"code.gitea.io/gitea/modules/json"

	"github.com/zeripath/ansihtml"
)

type MessageLog struct {
	lock      sync.RWMutex
	owners    int
	messages  []string
	start     int
	maxLength int
}

func (log *MessageLog) Len() int {
	log.lock.RLock()
	defer log.lock.RUnlock()
	return len(log.messages)
}

func (log *MessageLog) Get() []string {
	log.lock.RLock()
	defer log.lock.RUnlock()
	messages := make([]string, 0, len(log.messages))
	messages = append(messages, log.messages[log.start:]...)
	if log.start > 0 && len(log.messages)-log.start >= 0 {
		messages = append(messages, log.messages[:log.start]...)
	}
	return messages
}

func (log *MessageLog) GetHTML() []string {
	messages := log.Get()
	for i, msg := range messages {
		messages[i] = string(ansihtml.ConvertToHTML([]byte(msg)))
	}
	return messages
}

func (log *MessageLog) Close() error {
	log.lock.Lock()
	defer log.lock.Unlock()
	return log.close()
}

func (log *MessageLog) close() error {
	log.owners--
	if log.owners < 0 {
		log.owners = 0
	}

	if log.owners == 0 {
		log.messages = []string{}
		log.start = 0
		log.maxLength = 0
	}

	return nil
}

func (log *MessageLog) Empty() {
	log.lock.Lock()
	defer log.lock.Unlock()
	log.messages = []string{}
	log.start = 0
}

func (log *MessageLog) Resize(maxLength int) {
	log.lock.Lock()
	defer log.lock.Unlock()
	log.resize(maxLength)
}

func (log *MessageLog) resize(maxLength int) {
	if maxLength <= log.maxLength {
		return
	}
	if len(log.messages) < log.maxLength || log.start == 0 {
		log.maxLength = maxLength
		return
	}

	messages := make([]string, 0, len(log.messages))
	messages = append(messages, log.messages[log.start:]...)
	if log.start > 0 && len(log.messages)-log.start >= 0 {
		messages = append(messages, log.messages[:log.start]...)
	}
	log.messages = messages
	log.start = 0
	log.maxLength = maxLength
}

func (log *MessageLog) Write(p []byte) (int, error) {
	log.lock.Lock()
	defer log.lock.Unlock()
	if len(log.messages) < log.maxLength {
		log.messages = append(log.messages, string(p))
		return len(p), nil
	}
	log.messages[log.start] = string(p)
	log.start = (log.start + 1) % len(log.messages)

	return len(p), nil
}

var (
	messageLogLock     = sync.RWMutex{}
	messageLogRegistry = map[string]*MessageLog{}
)

func GetMessageLogs() map[string]*MessageLog {
	messageLogLock.RLock()
	defer messageLogLock.RUnlock()
	logs := make(map[string]*MessageLog)
	for k, v := range messageLogRegistry {
		logs[k] = v
	}
	return logs
}

type MessageLogger struct {
	WriterLogger
	messageLog *MessageLog
	LogName    string `json:"log_name"`
	MaxLength  int    `json:"max_length"`
}

func NewMessageLogger() LoggerProvider {
	log := &MessageLogger{
		messageLog: &MessageLog{},
		LogName:    "common",
	}
	log.NewWriterLogger(&nopWriteCloser{
		w: log.messageLog,
	})
	return log
}

// Flush when log should be flushed
func (log *MessageLogger) Flush() {
}

// Close when log should be closed
func (log *MessageLogger) Close() {
	_ = log.messageLog.Close()
}

// ReleaseReopen causes the logger to reconnect to os.Stdout
func (log *MessageLogger) ReleaseReopen() error {
	if log.messageLog == nil {
		return nil
	}
	log.messageLog.lock.Lock()
	log.messageLog.owners++
	log.messageLog.resize(log.MaxLength)
	log.messageLog.lock.Unlock()

	return nil
}

// Init inits connection writer with json config.
// json config only need key "level".
func (log *MessageLogger) Init(config string) error {
	log.LogName = "common"
	log.Colorize = true
	log.MaxLength = 1000
	err := json.Unmarshal([]byte(config), log)
	if err != nil {
		return fmt.Errorf("unable to parse JSON: %w", err)
	}

	log.LogName = strings.ToLower(strings.TrimSpace(log.LogName))
	if log.LogName == "" {
		log.LogName = "common"
	}
	if log.MaxLength <= 0 {
		log.MaxLength = 1000
	}
	messageLogLock.Lock()
	messageLog, has := messageLogRegistry[log.LogName]
	if !has {
		messageLog = &MessageLog{}
		messageLogRegistry[log.LogName] = messageLog
	}
	messageLogLock.Unlock()

	log.messageLog = messageLog
	log.messageLog.lock.Lock()
	log.messageLog.owners++
	log.messageLog.resize(log.MaxLength)
	log.messageLog.lock.Unlock()
	log.NewWriterLogger(log.messageLog)
	return nil
}

// GetName returns the default name for this implementation
func (log *MessageLogger) GetName() string {
	return "messages"
}

func init() {
	Register("messages", NewMessageLogger)
}
