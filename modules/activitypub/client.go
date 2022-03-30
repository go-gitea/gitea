// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"bytes"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"time"

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/proxy"
	"code.gitea.io/gitea/modules/setting"

	"github.com/go-fed/activity/pub"
	"github.com/go-fed/httpsig"
)

const (
	// ActivityStreamsContentType const
	ActivityStreamsContentType = `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`
)

func containsRequiredHTTPHeaders(method string, headers []string) error {
	var hasRequestTarget, hasDate, hasDigest bool
	for _, header := range headers {
		hasRequestTarget = hasRequestTarget || header == httpsig.RequestTarget
		hasDate = hasDate || header == "Date"
		hasDigest = hasDigest || header == "Digest"
	}
	if !hasRequestTarget {
		return fmt.Errorf("missing http header for %s: %s", method, httpsig.RequestTarget)
	} else if !hasDate {
		return fmt.Errorf("missing http header for %s: Date", method)
	} else if !hasDigest && method != http.MethodGet {
		return fmt.Errorf("missing http header for %s: Digest", method)
	}
	return nil
}

// Client struct
type Client struct {
	clock       pub.Clock
	client      *http.Client
	algs        []httpsig.Algorithm
	digestAlg   httpsig.DigestAlgorithm
	getHeaders  []string
	postHeaders []string
	priv        *rsa.PrivateKey
	pubID       string
}

// NewClient function
func NewClient(user *user_model.User, pubID string) (c *Client, err error) {
	if err = containsRequiredHTTPHeaders(http.MethodGet, setting.Federation.GetHeaders); err != nil {
		return
	} else if err = containsRequiredHTTPHeaders(http.MethodPost, setting.Federation.PostHeaders); err != nil {
		return
	}
	algos := make([]httpsig.Algorithm, len(setting.Federation.Algorithms))
	for i, algo := range setting.Federation.Algorithms {
		algos[i] = httpsig.Algorithm(algo)
	}
	clock, err := NewClock()
	if err != nil {
		return
	}

	priv, err := GetPrivateKey(user)
	if err != nil {
		return
	}
	privPem, _ := pem.Decode([]byte(priv))
	privParsed, err := x509.ParsePKCS1PrivateKey(privPem.Bytes)
	if err != nil {
		return
	}

	c = &Client{
		clock: clock,
		client: &http.Client{
			Transport: &http.Transport{
				Proxy: proxy.Proxy(),
			},
		},
		algs:        algos,
		digestAlg:   httpsig.DigestAlgorithm(setting.Federation.DigestAlgorithm),
		getHeaders:  setting.Federation.GetHeaders,
		postHeaders: setting.Federation.PostHeaders,
		priv:        privParsed,
		pubID:       pubID,
	}
	return
}

// NewRequest function
func (c *Client) NewRequest(b []byte, to string) (req *http.Request, err error) {
	byteCopy := make([]byte, len(b))
	copy(byteCopy, b)
	buf := bytes.NewBuffer(byteCopy)
	req, err = http.NewRequest(http.MethodPost, to, buf)
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", ActivityStreamsContentType)
	req.Header.Add("Accept-Charset", "utf-8")
	req.Header.Add("Date", fmt.Sprintf("%s GMT", c.clock.Now().UTC().Format(time.RFC1123)))

	signer, _, err := httpsig.NewSigner(c.algs, c.digestAlg, c.postHeaders, httpsig.Signature, 60)
	if err != nil {
		return
	}
	err = signer.SignRequest(c.priv, c.pubID, req, b)
	return
}

// Post function
func (c *Client) Post(b []byte, to string) (resp *http.Response, err error) {
	var req *http.Request
	if req, err = c.NewRequest(b, to); err != nil {
		return
	}
	resp, err = c.client.Do(req)
	return
}
