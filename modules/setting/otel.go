// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

type OtelExporterStruct struct {
	OtlpEnabled     bool
	OtlpEndpoint    string
	OtlpCompression string
	OtlpTLSInsecure bool

	OtlpHeaders map[string]string `ini:"-"`
	OtlpTimeout time.Duration     `ini:"-"`
}

var OtelExporter OtelExporterStruct

func loadOtelExporterFrom(cfg ConfigProvider) error {
	OtelExporter = OtelExporterStruct{
		OtlpEndpoint:    "http://localhost:4318",
		OtlpCompression: "gzip",
		OtlpTimeout:     time.Second * 10,
	}
	sec := cfg.Section("otel_exporter")
	if err := sec.MapTo(&OtelExporter); err != nil {
		return err
	}
	if !OtelExporter.OtlpEnabled {
		return nil
	}

	OtelExporter.OtlpTimeout = sec.Key("OTLP_TIMEOUT").MustDuration(OtelExporter.OtlpTimeout)

	otlpHeadersString := sec.Key("OTLP_HEADERS").String()
	if otlpHeadersString != "" {
		OtelExporter.OtlpHeaders = make(map[string]string)
		for _, header := range strings.Split(otlpHeadersString, ",") {
			header = strings.TrimSpace(header)
			if key, valRaw, ok := strings.Cut(header, "="); ok {
				val, err := url.QueryUnescape(valRaw)
				if err != nil {
					return fmt.Errorf("invalid OTLP_HEADER %q, err: %w", header, err)
				}
				OtelExporter.OtlpHeaders[key] = val
			}
		}
	}

	return nil
}
