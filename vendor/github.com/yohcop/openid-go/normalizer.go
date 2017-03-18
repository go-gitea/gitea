package openid

import (
	"errors"
	"net/url"
	"strings"
)

func Normalize(id string) (string, error) {
	id = strings.TrimSpace(id)
	if len(id) == 0 {
		return "", errors.New("No id provided")
	}

	// 7.2 from openID 2.0 spec.

	//If the user's input starts with the "xri://" prefix, it MUST be
	//stripped off, so that XRIs are used in the canonical form.
	if strings.HasPrefix(id, "xri://") {
		id = id[6:]
		return id, errors.New("XRI identifiers not supported")
	}

	// If the first character of the resulting string is an XRI
	// Global Context Symbol ("=", "@", "+", "$", "!") or "(", as
	// defined in Section 2.2.1 of [XRI_Syntax_2.0], then the input
	// SHOULD be treated as an XRI.
	if b := id[0]; b == '=' || b == '@' || b == '+' || b == '$' || b == '!' {
		return id, errors.New("XRI identifiers not supported")
	}

	// Otherwise, the input SHOULD be treated as an http URL; if it
	// does not include a "http" or "https" scheme, the Identifier
	// MUST be prefixed with the string "http://". If the URL
	// contains a fragment part, it MUST be stripped off together
	// with the fragment delimiter character "#". See Section 11.5.2 for
	// more information.
	if !strings.HasPrefix(id, "http://") && !strings.HasPrefix(id,
		"https://") {
		id = "http://" + id
	}
	if fragmentIndex := strings.Index(id, "#"); fragmentIndex != -1 {
		id = id[0:fragmentIndex]
	}
	if u, err := url.ParseRequestURI(id); err != nil {
		return "", err
	} else {
		if u.Host == "" {
			return "", errors.New("Invalid address provided as id")
		}
		if u.Path == "" {
			u.Path = "/"
		}
		id = u.String()
	}

	// URL Identifiers MUST then be further normalized by both
	// following redirects when retrieving their content and finally
	// applying the rules in Section 6 of [RFC3986] to the final
	// destination URL. This final URL MUST be noted by the Relying
	// Party as the Claimed Identifier and be used when requesting
	// authentication.
	return id, nil
}
