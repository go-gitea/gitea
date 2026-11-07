// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package setting

import (
	"errors"
	"fmt"
	"math"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"gitea.dev/modules/util"
)

const (
	defaultPagesMaxTarGzSize int64 = 100 * 1024 * 1024
	defaultPagesMaxSiteSize  int64 = 512 * 1024 * 1024
	defaultPagesMaxFileSize  int64 = 100 * 1024 * 1024
	defaultPagesMaxEntries         = 10_000
)

// Pages contains the Gitea Pages settings.
var Pages = struct {
	Storage                     *Storage
	MaxTarGzSize                int64
	MaxSiteSize                 int64
	MaxFileSize                 int64
	MaxEntries                  int
	BranchSourceBatchSize       int
	DomainVerificationBatchSize int
	DomainVerificationInterval  time.Duration
	RequireGiteaTXTRecord       bool
	DNSResolver                 string
}{
	MaxTarGzSize:                defaultPagesMaxTarGzSize,
	MaxSiteSize:                 defaultPagesMaxSiteSize,
	MaxFileSize:                 defaultPagesMaxFileSize,
	MaxEntries:                  defaultPagesMaxEntries,
	BranchSourceBatchSize:       50,
	DomainVerificationBatchSize: 100,
	DomainVerificationInterval:  15 * time.Minute,
	RequireGiteaTXTRecord:       true,
}

func loadPagesFrom(rootCfg ConfigProvider) error {
	sec := rootCfg.Section("pages")
	maxTarGzSize, err := parsePagesByteSize(sec, "MAX_TAR_GZ_SIZE", defaultPagesMaxTarGzSize)
	if err != nil {
		return err
	}
	maxSiteSize, err := parsePagesByteSize(sec, "MAX_SITE_SIZE", defaultPagesMaxSiteSize)
	if err != nil {
		return err
	}
	maxFileSize, err := parsePagesByteSize(sec, "MAX_FILE_SIZE", defaultPagesMaxFileSize)
	if err != nil {
		return err
	}
	maxEntries, err := parsePagesInt(sec, "MAX_ENTRIES", defaultPagesMaxEntries)
	if err != nil {
		return err
	}
	const archiveOverhead = int64(1024 * 1024)
	if maxTarGzSize > math.MaxInt64-archiveOverhead || maxFileSize > maxSiteSize ||
		maxSiteSize > math.MaxInt64-archiveOverhead || maxEntries <= 0 || int64(maxEntries) > (math.MaxInt64-maxSiteSize-archiveOverhead)/4096 {
		return errors.New("loadPagesFrom: invalid Pages archive limits")
	}
	branchBatchSize, err := parsePagesInt(sec, "BRANCH_SOURCE_BATCH_SIZE", 50)
	if err != nil {
		return err
	}
	domainBatchSize, err := parsePagesInt(sec, "DOMAIN_VERIFICATION_BATCH_SIZE", 100)
	if err != nil {
		return err
	}
	if branchBatchSize < 1 || branchBatchSize > 500 {
		return errors.New("loadPagesFrom: BRANCH_SOURCE_BATCH_SIZE must be between 1 and 500")
	}
	if domainBatchSize < 1 || domainBatchSize > 500 {
		return errors.New("loadPagesFrom: DOMAIN_VERIFICATION_BATCH_SIZE must be between 1 and 500")
	}
	domainVerificationInterval, err := time.ParseDuration(ConfigSectionKeyString(sec, "DOMAIN_VERIFICATION_INTERVAL", "15m"))
	if err != nil || domainVerificationInterval < time.Minute || domainVerificationInterval > 24*time.Hour {
		return errors.New("loadPagesFrom: DOMAIN_VERIFICATION_INTERVAL must be between 1m and 24h")
	}
	requireGiteaTXTRecord, err := strconv.ParseBool(ConfigSectionKeyString(sec, "REQUIRE_GITEA_TXT_RECORD", "true"))
	if err != nil {
		return fmt.Errorf("loadPagesFrom: REQUIRE_GITEA_TXT_RECORD must be true or false: %w", err)
	}
	dnsResolver, err := normalizeDNSResolver(ConfigSectionKeyString(sec, "DNS_RESOLVER", ""))
	if err != nil {
		return fmt.Errorf("loadPagesFrom: DNS_RESOLVER: %w", err)
	}
	storage, err := getStorage(rootCfg, "pages", "", nil)
	if err != nil {
		return err
	}
	Pages.Storage = storage
	Pages.MaxTarGzSize = maxTarGzSize
	Pages.MaxSiteSize = maxSiteSize
	Pages.MaxFileSize = maxFileSize
	Pages.MaxEntries = maxEntries
	Pages.BranchSourceBatchSize = branchBatchSize
	Pages.DomainVerificationBatchSize = domainBatchSize
	Pages.DomainVerificationInterval = domainVerificationInterval
	Pages.RequireGiteaTXTRecord = requireGiteaTXTRecord
	Pages.DNSResolver = dnsResolver
	return nil
}

func normalizeDNSResolver(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if address, err := netip.ParseAddr(value); err == nil {
		return net.JoinHostPort(address.String(), "53"), nil
	}
	host, port, err := net.SplitHostPort(value)
	if err != nil {
		return "", errors.New("must be an IP address with an optional numeric port")
	}
	address, err := netip.ParseAddr(host)
	if err != nil {
		return "", errors.New("must use an IP address")
	}
	parsedPort, err := strconv.ParseUint(port, 10, 16)
	if err != nil || parsedPort == 0 {
		return "", errors.New("must use a port between 1 and 65535")
	}
	return net.JoinHostPort(address.String(), strconv.FormatUint(parsedPort, 10)), nil
}

func parsePagesByteSize(sec ConfigSection, name string, defaultValue int64) (int64, error) {
	value := ConfigSectionKeyString(sec, name, strconv.FormatInt(defaultValue, 10))
	size, err := util.ParseByteSize(value)
	if err != nil || size == 0 || size > math.MaxInt64 {
		return 0, fmt.Errorf("loadPagesFrom: %s must be a positive byte size", name)
	}
	return int64(size), nil
}

func parsePagesInt(sec ConfigSection, name string, defaultValue int) (int, error) {
	value := ConfigSectionKeyString(sec, name, strconv.Itoa(defaultValue))
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("loadPagesFrom: %s must be an integer: %w", name, err)
	}
	return parsed, nil
}
