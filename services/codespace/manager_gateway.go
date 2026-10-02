// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package codespace

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	codespace_model "gitea.dev/models/codespace"
	"gitea.dev/models/db"
	"gitea.dev/modules/log"
	"gitea.dev/modules/setting"

	"golang.org/x/net/publicsuffix"
)

var dnsLabelPattern = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

// WarnManagerGatewayAddressConflicts reports stored Gateway base domains that overlap the current site cookie scope.
func WarnManagerGatewayAddressConflicts(ctx context.Context) error {
	var addresses []*codespace_model.ManagerAddress
	if err := db.GetEngine(ctx).Where("kind = ?", codespace_model.ManagerAddressGateway).Find(&addresses); err != nil {
		return err
	}
	for _, address := range addresses {
		normalized, err := normalizeGatewayURL(address.Address)
		if err != nil {
			log.Warn("Codespace Manager Gateway address is invalid in database: manager_id=%d gateway_url=%q error=%v. This stored declaration is ignored for startup validation; fix or delete the Manager from the Codespace settings page.", address.ManagerID, address.Address, err)
			continue
		}
		warnGatewayCookieScopeConflict(address.ManagerID, normalized)
	}
	return nil
}

func normalizeGatewayURL(rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", fmt.Errorf("parse gateway url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", errors.New("gateway url must use http or https")
	}
	if setting.Codespace.GatewayRequireHTTPS && parsed.Scheme != "https" {
		return "", errors.New("gateway url must use https")
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return "", errors.New("gateway url host is required")
	}
	if err := validateDNSHost(host); err != nil {
		return "", fmt.Errorf("invalid gateway url host: %w", err)
	}
	if len(strings.Repeat("a", 30)+"-"+strings.Repeat("0", 32)+"."+host) > 253 {
		return "", errors.New("derived gateway endpoint host is too long")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("gateway url must not contain userinfo, query, or fragment")
	}
	if parsed.Path != "" && parsed.Path != "/" {
		return "", errors.New("gateway url must not contain a business path")
	}
	port := parsed.Port()
	if port != "" {
		portNumber, err := strconv.Atoi(port)
		if err != nil || portNumber < 1 || portNumber > 65535 {
			return "", errors.New("invalid gateway url port")
		}
		if (parsed.Scheme == "http" && portNumber == 80) || (parsed.Scheme == "https" && portNumber == 443) {
			port = ""
		} else {
			port = strconv.Itoa(portNumber)
		}
	}
	normalized := parsed.Scheme + "://" + host
	if port != "" {
		normalized += ":" + port
	}
	if len(normalized) > 512 {
		return "", errors.New("gateway url is too long")
	}
	return normalized, nil
}

func warnGatewayCookieScopeConflict(managerID int64, gatewayURL string) {
	reason := diagnoseGatewayCookieScope(gatewayURL)
	if reason == "" {
		return
	}
	log.Warn("Codespace Manager Gateway cookie scope overlaps Gitea login scope: manager_id=%d gateway_url=%q root_url=%q session_domain=%q reason=%s. This is allowed because deployment scope is an administrator choice; use a separate registrable domain for stricter browser cookie isolation.",
		managerID, gatewayURL, setting.AppURL, setting.SessionConfig.Domain, reason)
}

func diagnoseGatewayCookieScope(gatewayURL string) string {
	parsed, err := url.Parse(gatewayURL)
	if err != nil {
		return "gateway URL cannot be parsed"
	}
	gatewayHost := parsed.Hostname()

	giteaURL, err := url.Parse(setting.AppURL)
	if err != nil {
		return "Gitea ROOT_URL cannot be parsed"
	}
	giteaHost := strings.ToLower(giteaURL.Hostname())
	if giteaHost != "" && net.ParseIP(giteaHost) == nil {
		if err := validateDNSHost(giteaHost); err != nil {
			return "Gitea ROOT_URL host is not a valid DNS host"
		}
		if sameRegistrableDomain(gatewayHost, giteaHost) {
			return "Gateway host and Gitea ROOT_URL host share the same registrable domain"
		}
		if isSameOrSubdomain(giteaHost, gatewayHost) {
			return "Gateway host covers the Gitea ROOT_URL host"
		}
	}

	sessionDomain := normalizeCookieDomain(setting.SessionConfig.Domain)
	if sessionDomain == "" {
		return ""
	}
	if err := validateDNSHost(sessionDomain); err != nil {
		return "Gitea session cookie domain is not a valid DNS host"
	}
	if isSameOrSubdomain(gatewayHost, sessionDomain) {
		return "Gateway host is inside the Gitea session cookie domain"
	}
	return ""
}

func sameRegistrableDomain(a, b string) bool {
	aSite, err := publicsuffix.EffectiveTLDPlusOne(a)
	if err != nil {
		return true
	}
	bSite, err := publicsuffix.EffectiveTLDPlusOne(b)
	if err != nil {
		return true
	}
	return aSite == bSite
}

func normalizeCookieDomain(domain string) string {
	domain = strings.ToLower(strings.TrimSpace(domain))
	domain = strings.TrimPrefix(domain, ".")
	return strings.TrimSuffix(domain, ".")
}

func isSameOrSubdomain(host, parent string) bool {
	return host == parent || strings.HasSuffix(host, "."+parent)
}

func normalizeGatewaySSHAddr(rawAddr string) (string, error) {
	host, port, err := net.SplitHostPort(strings.TrimSpace(rawAddr))
	if err != nil {
		return "", errors.New("gateway ssh address must use host:port")
	}
	host = strings.ToLower(strings.TrimSpace(host))
	if err := validateDNSHost(host); err != nil {
		return "", fmt.Errorf("invalid gateway ssh host: %w", err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return "", errors.New("invalid gateway ssh port")
	}
	normalized := net.JoinHostPort(host, strconv.Itoa(portNumber))
	if len(normalized) > 512 {
		return "", errors.New("gateway ssh address is too long")
	}
	return normalized, nil
}

func validateDNSHost(host string) error {
	if host == "" || strings.HasSuffix(host, ".") {
		return errors.New("host must be a DNS name without trailing dot")
	}
	if net.ParseIP(host) != nil {
		return errors.New("host must not be an IP address")
	}
	if len(host) > 253 {
		return errors.New("host is too long")
	}
	for label := range strings.SplitSeq(host, ".") {
		if !dnsLabelPattern.MatchString(label) {
			return fmt.Errorf("invalid DNS label %q", label)
		}
	}
	return nil
}
