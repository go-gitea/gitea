// Copyright 2022 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package activitypub

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"code.gitea.io/gitea/modules/activitypub"
	gitea_context "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/json"
	"code.gitea.io/gitea/modules/setting"

	"github.com/go-fed/activity/pub"
	"github.com/go-fed/activity/streams"
	"github.com/go-fed/activity/streams/vocab"
	"github.com/go-fed/httpsig"
)

type publicKeyer interface {
	GetW3IDSecurityV1PublicKey() vocab.W3IDSecurityV1PublicKeyProperty
}

func getPublicKeyFromResponse(ctx context.Context, b []byte, keyID *url.URL) (p crypto.PublicKey, err error) {
	m := make(map[string]interface{})
	err = json.Unmarshal(b, &m)
	if err != nil {
		return
	}
	var t vocab.Type
	t, err = streams.ToType(ctx, m)
	if err != nil {
		return
	}
	pker, ok := t.(publicKeyer)
	if !ok {
		err = fmt.Errorf("ActivityStreams type cannot be converted to one known to have publicKey property: %T", t)
		return
	}
	pkp := pker.GetW3IDSecurityV1PublicKey()
	if pkp == nil {
		err = fmt.Errorf("publicKey property is not provided")
		return
	}
	var pkpFound vocab.W3IDSecurityV1PublicKey
	for pkpIter := pkp.Begin(); pkpIter != pkp.End(); pkpIter = pkpIter.Next() {
		if !pkpIter.IsW3IDSecurityV1PublicKey() {
			continue
		}
		pkValue := pkpIter.Get()
		var pkID *url.URL
		pkID, err = pub.GetId(pkValue)
		if err != nil {
			return
		}
		if pkID.String() != keyID.String() {
			continue
		}
		pkpFound = pkValue
		break
	}
	if pkpFound == nil {
		err = fmt.Errorf("cannot find publicKey with id: %s in %s", keyID, b)
		return
	}
	pkPemProp := pkpFound.GetW3IDSecurityV1PublicKeyPem()
	if pkPemProp == nil || !pkPemProp.IsXMLSchemaString() {
		err = fmt.Errorf("publicKeyPem property is not provided or it is not embedded as a value")
		return
	}
	pubKeyPem := pkPemProp.Get()
	var block *pem.Block
	block, _ = pem.Decode([]byte(pubKeyPem))
	if block == nil || block.Type != "PUBLIC KEY" {
		err = fmt.Errorf("could not decode publicKeyPem to PUBLIC KEY pem block type")
		return
	}
	p, err = x509.ParsePKIXPublicKey(block.Bytes)
	return
}

func fetch(iri *url.URL) (b []byte, err error) {
	var req *http.Request
	req, err = http.NewRequest(http.MethodGet, iri.String(), nil)
	if err != nil {
		return
	}
	req.Header.Add("Accept", activitypub.ActivityStreamsContentType)
	req.Header.Add("Accept-Charset", "utf-8")
	clock, err := activitypub.NewClock()
	if err != nil {
		return
	}
	req.Header.Add("Date", fmt.Sprintf("%s GMT", clock.Now().UTC().Format("Mon, 02 Jan 2006 15:04:05")))
	var resp *http.Response
	client := &http.Client{}
	resp, err = client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("url IRI fetch [%s] failed with status (%d): %s", iri, resp.StatusCode, resp.Status)
		return
	}
	b, err = io.ReadAll(resp.Body)
	return
}

func verifyHTTPSignatures(ctx *gitea_context.APIContext) (authenticated bool, err error) {
	r := ctx.Req

	// 1. Figure out what key we need to verify
	var v httpsig.Verifier
	v, err = httpsig.NewVerifier(r)
	if err != nil {
		return
	}
	ID := v.KeyId()
	var idIRI *url.URL
	idIRI, err = url.Parse(ID)
	if err != nil {
		return
	}
	// 2. Fetch the public key of the other actor
	var b []byte
	b, err = fetch(idIRI)
	if err != nil {
		return
	}
	pKey, err := getPublicKeyFromResponse(*ctx, b, idIRI)
	if err != nil {
		return
	}
	// 3. Verify the other actor's key
	algo := httpsig.Algorithm(setting.Federation.Algorithms[0])
	authenticated = nil == v.Verify(pKey, algo)
	return
}

// ReqSignature function
func ReqSignature() func(ctx *gitea_context.APIContext) {
	return func(ctx *gitea_context.APIContext) {
		if authenticated, err := verifyHTTPSignatures(ctx); err != nil {
			ctx.Error(http.StatusInternalServerError, "verifyHttpSignatures", err)
		} else if !authenticated {
			ctx.Error(http.StatusForbidden, "reqSignature", "request signature verification failed")
		}
	}
}
