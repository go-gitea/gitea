// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package gtprof

type EventConfig struct {
	attributes []*TraceAttribute
}

type EventOption interface {
	applyEvent(*EventConfig)
}

type applyEventFunc func(*EventConfig)

func (f applyEventFunc) applyEvent(cfg *EventConfig) {
	f(cfg)
}

func WithAttributes(attrs ...*TraceAttribute) EventOption {
	return applyEventFunc(func(cfg *EventConfig) {
		cfg.attributes = append(cfg.attributes, attrs...)
	})
}

func eventConfigFromOptions(options ...EventOption) *EventConfig {
	cfg := &EventConfig{}
	for _, opt := range options {
		opt.applyEvent(cfg)
	}
	return cfg
}
