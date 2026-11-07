// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package avatar

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"gitea.dev/modules/cache"
)

// LookupFederatedHost returns the avatar host from the email domain's SRV record. https://wiki.libravatar.org/api/
func LookupFederatedHost(ctx context.Context, email string, secure bool) string {
	_, domain, found := strings.CutLast(email, "@")
	if !found {
		return ""
	}
	domain = strings.ToLower(domain)

	service, defaultPort := "avatars", uint16(80)
	if secure {
		service, defaultPort = "avatars-sec", 443
	}

	host, _ := cache.GetString(cache.SafeCacheKey("Avatar:SRV:"+service, domain), func() (string, error) {
		lookupCtx, cancel := context.WithTimeout(ctx, 3*time.Second) // a slow resolver must not stall the request
		defer cancel()

		// LookupSRV already sorts by priority and randomizes by weight (RFC 2782)
		_, records, err := net.DefaultResolver.LookupSRV(lookupCtx, service, "tcp", domain)
		var dnsErr *net.DNSError
		if err != nil && !(errors.As(err, &dnsErr) && dnsErr.IsNotFound) {
			return "", err // a timeout or a cancelled request must not cache as "no record"
		}
		if err != nil || len(records) == 0 {
			return "", nil
		}
		return srvHost(records[0], defaultPort), nil
	})
	return host
}

// a target of "." means the service is unavailable (RFC 2782)
func srvHost(record *net.SRV, defaultPort uint16) string {
	target := strings.TrimSuffix(record.Target, ".")
	if target == "" || record.Port == defaultPort {
		return target
	}
	return net.JoinHostPort(target, strconv.Itoa(int(record.Port)))
}
