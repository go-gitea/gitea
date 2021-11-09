// Copyright 2021 The Gitea Authors. All rights reserved.
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

	user_model "code.gitea.io/gitea/models/user"
	"code.gitea.io/gitea/modules/setting"
	"github.com/go-fed/activity/pub"
	"github.com/go-fed/httpsig"
)

const (
	activityStreamsContentType = "application/ld+json; profile=\"https://www.w3.org/ns/activitystreams\""
)

func containsRequiredHttpHeaders(method string, headers []string) error {
	var hasRequestTarget, hasDate, hasDigest bool
	for _, header := range headers {
		hasRequestTarget = hasRequestTarget || header == httpsig.RequestTarget
		hasDate = hasDate || header == "Date"
		hasDigest = method == "GET" || hasDigest || header == "Digest"
	}
	if !hasRequestTarget {
		return fmt.Errorf("missing http header for %s: %s", method, httpsig.RequestTarget)
	} else if !hasDate {
		return fmt.Errorf("missing http header for %s: Date", method)
	} else if !hasDigest {
		return fmt.Errorf("missing http header for %s: Digest", method)
	}
	return nil
}

type Client struct {
	clock       pub.Clock
	client      *http.Client
	algs        []httpsig.Algorithm
	digestAlg   httpsig.DigestAlgorithm
	getHeaders  []string
	postHeaders []string
	priv        *rsa.PrivateKey
	pubId       string
}

func NewClient(user *user_model.User, pubId string) (c *Client, err error) {
	if err = containsRequiredHttpHeaders(http.MethodGet, setting.Federation.GetHeaders); err != nil {
		return
	} else if err = containsRequiredHttpHeaders(http.MethodPost, setting.Federation.PostHeaders); err != nil {
		return
	} else if !httpsig.IsSupportedDigestAlgorithm(setting.Federation.DigestAlgorithm) {
		err = fmt.Errorf("unsupported digest algorithm: %s", setting.Federation.DigestAlgorithm)
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
		clock:       clock,
		client:      &http.Client{},
		algs:        algos,
		digestAlg:   httpsig.DigestAlgorithm(setting.Federation.DigestAlgorithm),
		getHeaders:  setting.Federation.GetHeaders,
		postHeaders: setting.Federation.PostHeaders,
		priv:        privParsed,
		pubId:       pubId,
	}
	return
}

func (c *Client) Post(b []byte, to string) (resp *http.Response, err error) {
	byteCopy := make([]byte, len(b))
	copy(byteCopy, b)
	buf := bytes.NewBuffer(byteCopy)
	var req *http.Request
	req, err = http.NewRequest(http.MethodPost, to, buf)
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", activityStreamsContentType)
	req.Header.Add("Accept-Charset", "utf-8")
	req.Header.Add("Date", fmt.Sprintf("%s GMT", c.clock.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05")))

	signer, _, err := httpsig.NewSigner(c.algs, c.digestAlg, c.postHeaders, httpsig.Signature, 60)
	if err != nil {
		return
	}
	err = signer.SignRequest(c.priv, c.pubId, req, b)
	if err != nil {
		return
	}
	resp, err = c.client.Do(req)
	return
}
