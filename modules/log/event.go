// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package log

import (
	"fmt"
	"sync"
	"time"
)

// Event represents a logging event
type Event struct {
	level      Level
	msg        string
	caller     string
	filename   string
	line       int
	time       time.Time
	stacktrace string
}

// EventLogger represents the behaviours of a logger
type EventLogger interface {
	LogEvent(event *Event) error
	Close()
	Flush()
	GetLevel() Level
	GetStacktraceLevel() Level
	GetName() string
}

// ChannelledLog represents a cached channel to a LoggerProvider
type ChannelledLog struct {
	name           string
	provider       string
	queue          chan *Event
	loggerProvider LoggerProvider
	flush          chan bool
	close          chan bool
	closed         chan bool
}

// NewChannelledLog a new logger instance with given logger provider and config.
func NewChannelledLog(name, provider, config string, bufferLength int64) (*ChannelledLog, error) {
	if log, ok := providers[provider]; ok {
		l := &ChannelledLog{
			queue:  make(chan *Event, bufferLength),
			flush:  make(chan bool),
			close:  make(chan bool),
			closed: make(chan bool),
		}
		l.loggerProvider = log()
		if err := l.loggerProvider.Init(config); err != nil {
			return nil, err
		}
		l.name = name
		l.provider = provider
		go l.Start()
		return l, nil
	}
	return nil, ErrUnknownProvider{provider}
}

// Start processing the ChannelledLog
func (l *ChannelledLog) Start() {
	for {
		select {
		case event, ok := <-l.queue:
			if !ok {
				l.closeLogger()
				return
			}
			l.loggerProvider.LogEvent(event)
		case _, ok := <-l.flush:
			if !ok {
				l.closeLogger()
				return
			}
			l.loggerProvider.Flush()
		case <-l.close:
			l.closeLogger()
			return
		}
	}
}

// LogEvent logs an event to this ChannelledLog
func (l *ChannelledLog) LogEvent(event *Event) error {
	select {
	case l.queue <- event:
		return nil
	case <-time.After(60 * time.Second):
		// We're blocked!
		return ErrTimeout{
			Name:     l.name,
			Provider: l.provider,
		}
	}
}

func (l *ChannelledLog) closeLogger() {
	l.loggerProvider.Flush()
	l.loggerProvider.Close()
	l.closed <- true
}

// Close this ChannelledLog
func (l *ChannelledLog) Close() {
	l.close <- true
	<-l.closed
}

// Flush this ChannelledLog
func (l *ChannelledLog) Flush() {
	l.flush <- true
}

// GetLevel gets the level of this ChannelledLog
func (l *ChannelledLog) GetLevel() Level {
	return l.loggerProvider.GetLevel()
}

// GetStacktraceLevel gets the level of this ChannelledLog
func (l *ChannelledLog) GetStacktraceLevel() Level {
	return l.loggerProvider.GetStacktraceLevel()
}

// GetName returns the name of this ChannelledLog
func (l *ChannelledLog) GetName() string {
	return l.name
}

// MultiChannelledLog represents a cached channel to a LoggerProvider
type MultiChannelledLog struct {
	name            string
	bufferLength    int64
	queue           chan *Event
	mutex           sync.Mutex
	loggers         map[string]EventLogger
	flush           chan bool
	close           chan bool
	started         bool
	level           Level
	stacktraceLevel Level
	closed          chan bool
}

// NewMultiChannelledLog a new logger instance with given logger provider and config.
func NewMultiChannelledLog(name string, bufferLength int64) *MultiChannelledLog {
	m := &MultiChannelledLog{
		name:            name,
		queue:           make(chan *Event, bufferLength),
		flush:           make(chan bool),
		bufferLength:    bufferLength,
		loggers:         make(map[string]EventLogger),
		level:           NONE,
		stacktraceLevel: NONE,
		close:           make(chan bool),
		closed:          make(chan bool),
	}
	return m
}

// AddLogger adds a logger to this MultiChannelledLog
func (m *MultiChannelledLog) AddLogger(logger EventLogger) error {
	m.mutex.Lock()
	name := logger.GetName()
	if _, has := m.loggers[name]; has {
		m.mutex.Unlock()
		return ErrDuplicateName{name}
	}
	m.loggers[name] = logger
	if logger.GetLevel() < m.level {
		m.level = logger.GetLevel()
	}
	if logger.GetStacktraceLevel() < m.stacktraceLevel {
		m.stacktraceLevel = logger.GetStacktraceLevel()
	}
	m.mutex.Unlock()
	go m.Start()
	return nil
}

// DelLogger removes a sub logger from this MultiChannelledLog
// NB: If you delete the last sublogger this logger will simply drop
// log events
func (m *MultiChannelledLog) DelLogger(name string) bool {
	m.mutex.Lock()
	logger, has := m.loggers[name]
	if !has {
		m.mutex.Unlock()
		return false
	}
	delete(m.loggers, name)
	m.internalResetLevel()
	m.mutex.Unlock()
	logger.Flush()
	logger.Close()
	return true
}

// GetEventLogger returns a sub logger from this MultiChannelledLog
func (m *MultiChannelledLog) GetEventLogger(name string) EventLogger {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.loggers[name]
}

// GetEventLoggerNames returns a list of names
func (m *MultiChannelledLog) GetEventLoggerNames() []string {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	var keys []string
	for k := range m.loggers {
		keys = append(keys, k)
	}
	return keys
}

func (m *MultiChannelledLog) closeLoggers() {
	m.mutex.Lock()
	for _, logger := range m.loggers {
		logger.Flush()
		logger.Close()
	}
	m.mutex.Unlock()
	m.closed <- true
}

// Start processing the MultiChannelledLog
func (m *MultiChannelledLog) Start() {
	m.mutex.Lock()
	if m.started {
		m.mutex.Unlock()
		return
	}
	m.started = true
	m.mutex.Unlock()
	for {
		select {
		case event, ok := <-m.queue:
			if !ok {
				m.closeLoggers()
				return
			}
			m.mutex.Lock()
			for _, logger := range m.loggers {
				err := logger.LogEvent(event)
				if err != nil {
					fmt.Println(err)
				}
			}
			m.mutex.Unlock()
		case _, ok := <-m.flush:
			if !ok {
				m.closeLoggers()
				return
			}
			m.mutex.Lock()
			for _, logger := range m.loggers {
				logger.Flush()
			}
			m.mutex.Unlock()
		case <-m.close:
			m.closeLoggers()
			return
		}
	}
}

// LogEvent logs an event to this MultiChannelledLog
func (m *MultiChannelledLog) LogEvent(event *Event) error {
	select {
	case m.queue <- event:
		return nil
	case <-time.After(60 * time.Second):
		// We're blocked!
		return ErrTimeout{
			Name:     m.name,
			Provider: "MultiChannelledLog",
		}
	}
}

// Close this MultiChannelledLog
func (m *MultiChannelledLog) Close() {
	m.close <- true
	<-m.closed
}

// Flush this ChannelledLog
func (m *MultiChannelledLog) Flush() {
	m.flush <- true
}

// GetLevel gets the level of this MultiChannelledLog
func (m *MultiChannelledLog) GetLevel() Level {
	return m.level
}

// GetStacktraceLevel gets the level of this MultiChannelledLog
func (m *MultiChannelledLog) GetStacktraceLevel() Level {
	return m.stacktraceLevel
}

func (m *MultiChannelledLog) internalResetLevel() Level {
	m.level = NONE
	for _, logger := range m.loggers {
		level := logger.GetLevel()
		if level < m.level {
			m.level = level
		}
		level = logger.GetStacktraceLevel()
		if level < m.stacktraceLevel {
			m.stacktraceLevel = level
		}
	}
	return m.level
}

// ResetLevel will reset the level of this MultiChannelledLog
func (m *MultiChannelledLog) ResetLevel() Level {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.internalResetLevel()
}

// GetName gets the name of this MultiChannelledLog
func (m *MultiChannelledLog) GetName() string {
	return m.name
}
