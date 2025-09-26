// Copyright 2025 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"os"
	"strings"
	"time"
)

// OpenTelemetry settings for integration
var OpenTelemetry = struct {
	Enabled     bool
	ServiceName string
	Endpoint    string
	Headers     map[string]string
	Insecure    bool
	Compression string
	Timeout     time.Duration
}{
	Enabled:     false,
	ServiceName: "gitea",
	Endpoint:    "localhost:4318",
	Headers:     make(map[string]string),
	Insecure:    true,
	Compression: "gzip",
	Timeout:     10 * time.Second,
}

func loadOpenTelemetryFrom(rootCfg ConfigProvider) {
	mustMapSetting(rootCfg, "opentelemetry", &OpenTelemetry)

	sec := rootCfg.Section("opentelemetry")
	OpenTelemetry.Enabled = sec.Key("Enabled").MustBool(false)
	OpenTelemetry.ServiceName = sec.Key("ServiceName").MustString("gitea")
	OpenTelemetry.Endpoint = sec.Key("Endpoint").MustString("localhost:4318")
	OpenTelemetry.Insecure = sec.Key("Insecure").MustBool(true)
	OpenTelemetry.Compression = sec.Key("Compression").MustString("gzip")
	OpenTelemetry.Timeout = sec.Key("Timeout").MustDuration(10 * time.Second)

	if headersStr := sec.Key("Headers").MustString(""); headersStr != "" {
		if OpenTelemetry.Headers == nil {
			OpenTelemetry.Headers = make(map[string]string)
		}
		headers := parseOTELHeaders(headersStr)
		for k, v := range headers {
			OpenTelemetry.Headers[k] = v
		}
	}

	// Support standard OpenTelemetry environment variables
	if envServiceName := getEnvValue("OTEL_SERVICE_NAME"); envServiceName != "" {
		OpenTelemetry.ServiceName = envServiceName
	}

	if envEndpoint := getEnvValue("OTEL_EXPORTER_OTLP_ENDPOINT"); envEndpoint != "" {
		OpenTelemetry.Endpoint = envEndpoint
	} else if envTracesEndpoint := getEnvValue("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"); envTracesEndpoint != "" {
		// Convert specific traces endpoint back to base endpoint
		if strings.HasSuffix(envTracesEndpoint, "/v1/traces") {
			OpenTelemetry.Endpoint = strings.TrimSuffix(envTracesEndpoint, "/v1/traces")
		} else {
			OpenTelemetry.Endpoint = envTracesEndpoint
		}
	}

	if envHeaders := getEnvValue("OTEL_EXPORTER_OTLP_HEADERS"); envHeaders != "" {
		headers := parseOTELHeaders(envHeaders)
		if OpenTelemetry.Headers == nil {
			OpenTelemetry.Headers = make(map[string]string)
		}
		for k, v := range headers {
			OpenTelemetry.Headers[k] = v
		}
	}

	if envInsecure := getEnvValue("OTEL_EXPORTER_OTLP_INSECURE"); envInsecure == "true" {
		OpenTelemetry.Insecure = true
	} else if envInsecure == "false" {
		OpenTelemetry.Insecure = false
	}

	if envCompression := getEnvValue("OTEL_EXPORTER_OTLP_COMPRESSION"); envCompression != "" {
		OpenTelemetry.Compression = envCompression
	}
}

// parseOTELHeaders parses OpenTelemetry headers from environment variable format
// Format: "key1=value1,key2=value2"
func parseOTELHeaders(headersStr string) map[string]string {
	headers := make(map[string]string)
	pairs := strings.Split(headersStr, ",")

	for _, pair := range pairs {
		kv := strings.Split(strings.TrimSpace(pair), "=")
		if len(kv) == 2 {
			headers[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}

	return headers
}

func getEnvValue(key string) string {
	return os.Getenv(key)
}
