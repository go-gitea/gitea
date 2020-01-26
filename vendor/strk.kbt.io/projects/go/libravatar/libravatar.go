// Copyright 2016 by Sandro Santilli <strk@kbt.io>
// Use of this source code is governed by a MIT
// license that can be found in the LICENSE file.

// Implements support for federated avatars lookup.
// See https://wiki.libravatar.org/api/

package libravatar // import "strk.kbt.io/projects/go/libravatar"

import (
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"math/rand"
	"net"
	"net/mail"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Default images (to be used as defaultURL)
const (
	// Do not load any image if none is associated with the email
	// hash, instead return an HTTP 404 (File Not Found) response
	HTTP404 = "404"
	// (mystery-man) a simple, cartoon-style silhouetted outline of
	// a person (does not vary by email hash)
	MysteryMan = "mm"
	// a geometric pattern based on an email hash
	IdentIcon = "identicon"
	// a generated 'monster' with different colors, faces, etc
	MonsterID = "monsterid"
	// generated faces with differing features and backgrounds
	Wavatar = "wavatar"
	// awesome generated, 8-bit arcade-style pixelated faces
	Retro = "retro"
)

var (
	// DefaultLibravatar is a default Libravatar object,
	// enabling object-less function calls
	DefaultLibravatar = New()
)

/* This should be moved in its own file */
type cacheKey struct {
	service string
	domain  string
}

type cacheValue struct {
	target    string
	checkedAt time.Time
}

// Libravatar is an opaque structure holding service configuration
type Libravatar struct {
	defURL             string // default url
	picSize            int    // picture size
	fallbackHost       string // default fallback URL
	secureFallbackHost string // default fallback URL for secure connections
	useHTTPS           bool
	nameCache          map[cacheKey]cacheValue
	nameCacheDuration  time.Duration
	nameCacheMutex     *sync.Mutex
	minSize            uint   // smallest image dimension allowed
	maxSize            uint   // largest image dimension allowed
	size               uint   // what dimension should be used
	serviceBase        string // SRV record to be queried for federation
	secureServiceBase  string // SRV record to be queried for federation with secure servers
}

// New instanciates a new Libravatar object (handle)
func New() *Libravatar {
	// According to https://wiki.libravatar.org/running_your_own/
	// the time-to-live (cache expiry) should be set to at least 1 day.
	return &Libravatar{
		fallbackHost:       `cdn.libravatar.org`,
		secureFallbackHost: `seccdn.libravatar.org`,
		minSize:            1,
		maxSize:            512,
		size:               0, // unset, defaults to 80
		serviceBase:        `avatars`,
		secureServiceBase:  `avatars-sec`,
		nameCache:          make(map[cacheKey]cacheValue),
		nameCacheDuration:  24 * time.Hour,
		nameCacheMutex:     &sync.Mutex{},
	}
}

// SetFallbackHost sets the hostname for fallbacks in case no avatar
// service is defined for a domain
func (v *Libravatar) SetFallbackHost(host string) {
	v.fallbackHost = host
}

// SetSecureFallbackHost sets the hostname for fallbacks in case no
// avatar service is defined for a domain, when requiring secure domains
func (v *Libravatar) SetSecureFallbackHost(host string) {
	v.secureFallbackHost = host
}

// SetUseHTTPS sets flag requesting use of https for fetching avatars
func (v *Libravatar) SetUseHTTPS(use bool) {
	v.useHTTPS = use
}

// SetAvatarSize sets avatars image dimension (0 for default)
func (v *Libravatar) SetAvatarSize(size uint) {
	v.size = size
}

// generate hash, either with email address or OpenID
func (v *Libravatar) genHash(email *mail.Address, openid *url.URL) string {
	if email != nil {
		email.Address = strings.ToLower(strings.TrimSpace(email.Address))
		sum := md5.Sum([]byte(email.Address))
		return fmt.Sprintf("%x", sum)
	} else if openid != nil {
		openid.Scheme = strings.ToLower(openid.Scheme)
		openid.Host = strings.ToLower(openid.Host)
		sum := sha256.Sum256([]byte(openid.String()))
		return fmt.Sprintf("%x", sum)
	}
	// panic, because this should not be reachable
	panic("Neither Email or OpenID set")
}

// Gets domain out of email or openid (for openid to be parsed, email has to be nil)
func (v *Libravatar) getDomain(email *mail.Address, openid *url.URL) string {
	if email != nil {
		u, err := url.Parse("//" + email.Address)
		if err != nil {
			if v.useHTTPS && v.secureFallbackHost != "" {
				return v.secureFallbackHost
			}
			return v.fallbackHost
		}
		return u.Host
	} else if openid != nil {
		return openid.Host
	}
	// panic, because this should not be reachable
	panic("Neither Email or OpenID set")
}

// Processes email or openid (for openid to be processed, email has to be nil)
func (v *Libravatar) process(email *mail.Address, openid *url.URL) (string, error) {
	URL, err := v.baseURL(email, openid)
	if err != nil {
		return "", err
	}
	res := fmt.Sprintf("%s/avatar/%s", URL, v.genHash(email, openid))

	values := make(url.Values)
	if v.defURL != "" {
		values.Add("d", v.defURL)
	}
	if v.size > 0 {
		values.Add("s", fmt.Sprintf("%d", v.size))
	}

	if len(values) > 0 {
		return fmt.Sprintf("%s?%s", res, values.Encode()), nil
	}
	return res, nil
}

// Finds or defaults a URL for Federation (for openid to be used, email has to be nil)
func (v *Libravatar) baseURL(email *mail.Address, openid *url.URL) (string, error) {
	var service, protocol, domain string

	if v.useHTTPS {
		protocol = "https://"
		service = v.secureServiceBase
		domain = v.secureFallbackHost

	} else {
		protocol = "http://"
		service = v.serviceBase
		domain = v.fallbackHost
	}

	host := v.getDomain(email, openid)
	key := cacheKey{service, host}
	now := time.Now()
	v.nameCacheMutex.Lock()
	val, found := v.nameCache[key]
	v.nameCacheMutex.Unlock()
	if found && now.Sub(val.checkedAt) <= v.nameCacheDuration {
		return protocol + val.target, nil
	}

	_, addrs, err := net.LookupSRV(service, "tcp", host)
	if err != nil && err.(*net.DNSError).IsTimeout {
		return "", err
	}

	if len(addrs) == 1 {
		// select only record, if only one is available
		domain = strings.TrimSuffix(addrs[0].Target, ".")
	} else if len(addrs) > 1 {
		// Select first record according to RFC2782 weight
		// ordering algorithm (page 3)

		type record struct {
			srv    *net.SRV
			weight uint16
		}

		var (
			totalWeight uint16
			records     []record
			topPriority = addrs[0].Priority
			topRecord   *net.SRV
		)

		for _, rr := range addrs {
			if rr.Priority > topPriority {
				continue
			} else if rr.Priority < topPriority {
				// won't happen, because net sorts
				// by priority, but just in case
				totalWeight = 0
				records = nil
				topPriority = rr.Priority
			}

			totalWeight += rr.Weight

			if rr.Weight > 0 {
				records = append(records, record{rr, totalWeight})
			} else if rr.Weight == 0 {
				records = append([]record{record{srv: rr, weight: totalWeight}}, records...)
			}
		}

		if len(records) == 1 {
			topRecord = records[0].srv
		} else {
			randnum := uint16(rand.Intn(int(totalWeight)))

			for _, rr := range records {
				if rr.weight >= randnum {
					topRecord = rr.srv
					break
				}
			}
		}

		domain = fmt.Sprintf("%s:%d", topRecord.Target, topRecord.Port)
	}

	v.nameCacheMutex.Lock()
	v.nameCache[key] = cacheValue{checkedAt: now, target: domain}
	v.nameCacheMutex.Unlock()
	return protocol + domain, nil
}

// FromEmail returns the url of the avatar for the given email
func (v *Libravatar) FromEmail(email string) (string, error) {
	addr, err := mail.ParseAddress(email)
	if err != nil {
		return "", err
	}

	link, err := v.process(addr, nil)
	if err != nil {
		return "", err
	}

	return link, nil
}

// FromEmail is the object-less call to DefaultLibravatar for an email adders
func FromEmail(email string) (string, error) {
	return DefaultLibravatar.FromEmail(email)
}

// FromURL returns the url of the avatar for the given url (typically
// for OpenID)
func (v *Libravatar) FromURL(openid string) (string, error) {
	ourl, err := url.Parse(openid)
	if err != nil {
		return "", err
	}

	if !ourl.IsAbs() {
		return "", fmt.Errorf("Is not an absolute URL")
	} else if ourl.Scheme != "http" && ourl.Scheme != "https" {
		return "", fmt.Errorf("Invalid protocol: %s", ourl.Scheme)
	}

	link, err := v.process(nil, ourl)
	if err != nil {
		return "", err
	}

	return link, nil
}

// FromURL is the object-less call to DefaultLibravatar for a URL
func FromURL(openid string) (string, error) {
	return DefaultLibravatar.FromURL(openid)
}
