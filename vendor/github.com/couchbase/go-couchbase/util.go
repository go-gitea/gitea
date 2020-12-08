package couchbase

import (
	"fmt"
	"net/url"
	"strings"
)

// CleanupHost returns the hostname with the given suffix removed.
func CleanupHost(h, commonSuffix string) string {
	if strings.HasSuffix(h, commonSuffix) {
		return h[:len(h)-len(commonSuffix)]
	}
	return h
}

// FindCommonSuffix returns the longest common suffix from the given
// strings.
func FindCommonSuffix(input []string) string {
	rv := ""
	if len(input) < 2 {
		return ""
	}
	from := input
	for i := len(input[0]); i > 0; i-- {
		common := true
		suffix := input[0][i:]
		for _, s := range from {
			if !strings.HasSuffix(s, suffix) {
				common = false
				break
			}
		}
		if common {
			rv = suffix
		}
	}
	return rv
}

// ParseURL is a wrapper around url.Parse with some sanity-checking
func ParseURL(urlStr string) (result *url.URL, err error) {
	result, err = url.Parse(urlStr)
	if result != nil && result.Scheme == "" {
		result = nil
		err = fmt.Errorf("invalid URL <%s>", urlStr)
	}
	return
}
