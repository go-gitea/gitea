package oauth

import (
	"bytes"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

//
// OAuth1 2-legged provider
// Contributed by https://github.com/jacobpgallagher
//

// Provide an buffer reader which implements the Close() interface
type oauthBufferReader struct {
	*bytes.Buffer
}

// So that it implements the io.ReadCloser interface
func (m oauthBufferReader) Close() error { return nil }

type ConsumerGetter func(key string, header map[string]string) (*Consumer, error)

// Provider provides methods for a 2-legged Oauth1 provider
type Provider struct {
	ConsumerGetter ConsumerGetter

	// For mocking
	clock clock
}

// NewProvider takes a function to get the consumer secret from a datastore.
// Returns a Provider
func NewProvider(secretGetter ConsumerGetter) *Provider {
	provider := &Provider{
		secretGetter,
		&defaultClock{},
	}
	return provider
}

// Combine a URL and Request to make the URL absolute
func makeURLAbs(url *url.URL, request *http.Request) {
	if !url.IsAbs() {
		url.Host = request.Host
		if request.TLS != nil || request.Header.Get("X-Forwarded-Proto") == "https" {
			url.Scheme = "https"
		} else {
			url.Scheme = "http"
		}
	}
}

// IsAuthorized takes an *http.Request and returns a pointer to a string containing the consumer key,
// or nil if not authorized
func (provider *Provider) IsAuthorized(request *http.Request) (*string, error) {
	var err error
	var userParams map[string]string

	// start with the body/query params
	userParams, err = parseBody(request)
	if err != nil {
		return nil, err
	}

	// if the oauth params are in the Authorization header, grab them, and
	// let them override what's in userParams
	authHeader := request.Header.Get(HTTP_AUTH_HEADER)
	if len(authHeader) > 6 && strings.EqualFold(OAUTH_HEADER, authHeader[0:6]) {
		authHeader = authHeader[6:]
		params := strings.Split(authHeader, ",")
		for _, param := range params {
			vals := strings.SplitN(param, "=", 2)
			k := strings.Trim(vals[0], " ")
			v := strings.Trim(strings.Trim(vals[1], "\""), " ")
			if strings.HasPrefix(k, "oauth") {
				userParams[k], err = url.QueryUnescape(v)
				if err != nil {
					return nil, err
				}
			}
		}
	}

	// pop the request's signature, it's not included in our signature
	// calculation
	oauthSignature, ok := userParams[SIGNATURE_PARAM]
	if !ok {
		return nil, fmt.Errorf("no oauth signature")
	}
	delete(userParams, SIGNATURE_PARAM)

	// get the oauth consumer key
	consumerKey, ok := userParams[CONSUMER_KEY_PARAM]
	if !ok || consumerKey == "" {
		return nil, fmt.Errorf("no consumer key")
	}

	// use it to create a consumer object
	consumer, err := provider.ConsumerGetter(consumerKey, userParams)
	if err != nil {
		return nil, err
	}

	// Make sure timestamp is no more than 10 digits
	timestamp := userParams[TIMESTAMP_PARAM]
	if len(timestamp) > 10 {
		timestamp = timestamp[0:10]
	}

	// Check the timestamp
	if !consumer.serviceProvider.IgnoreTimestamp {
		oauthTimeNumber, err := strconv.Atoi(timestamp)
		if err != nil {
			return nil, err
		}

		if math.Abs(float64(int64(oauthTimeNumber)-provider.clock.Seconds())) > 5*60 {
			return nil, fmt.Errorf("too much clock skew")
		}
	}

	// Include the query string params in the base string
	if consumer.serviceProvider.SignQueryParams {
		for k, v := range request.URL.Query() {
			userParams[k] = strings.Join(v, "")
		}
	}

	// if our consumer supports bodyhash, check it
	if consumer.serviceProvider.BodyHash {
		bodyHash, err := calculateBodyHash(request, consumer.signer)
		if err != nil {
			return nil, err
		}

		sentHash, ok := userParams[BODY_HASH_PARAM]

		if bodyHash == "" && ok {
			return nil, fmt.Errorf("body_hash must not be set")
		} else if sentHash != bodyHash {
			return nil, fmt.Errorf("body_hash mismatch")
		}
	}

	allParams := NewOrderedParams()
	for key, value := range userParams {
		allParams.Add(key, value)
	}

	makeURLAbs(request.URL, request)
	baseString := consumer.requestString(request.Method, canonicalizeUrl(request.URL), allParams)
	err = consumer.signer.Verify(baseString, oauthSignature)
	if err != nil {
		return nil, err
	}

	return &consumerKey, nil
}
