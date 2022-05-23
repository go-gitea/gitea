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
	"time"

	"code.gitea.io/gitea/modules/activitypub"
	gitea_context "code.gitea.io/gitea/modules/context"
	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/setting"

	ap "github.com/go-ap/activitypub"
	"github.com/go-fed/httpsig"
)

func getPublicKeyFromResponse(ctx context.Context, b []byte, keyID *url.URL) (p crypto.PublicKey, err error) {
	person := ap.PersonNew(ap.IRI(keyID.String()))
	err = person.UnmarshalJSON(b)
	if err != nil {
		err = fmt.Errorf("ActivityStreams type cannot be converted to one known to have publicKey property: %T", b)
		return
	}
	pkey := person.PublicKey
	if pkey.ID.String() != keyID.String() {
		err = fmt.Errorf("cannot find publicKey with id: %s in %s", keyID, b)
		return
	}
	pubKeyPem := pkey.PublicKeyPem
	block, _ := pem.Decode([]byte(pubKeyPem))
	if block == nil || block.Type != "PUBLIC KEY" {
		err = fmt.Errorf("could not decode publicKeyPem to PUBLIC KEY pem block type")
		return
	}
	p, err = x509.ParsePKIXPublicKey(block.Bytes)
	return
}

func fetch(iri *url.URL) (b []byte, err error) {
	req := httplib.NewRequest(iri.String(), http.MethodGet)
	req.Header("Accept", activitypub.ActivityStreamsContentType)
	req.Header("Accept-Charset", "utf-8")
	req.Header("Date", fmt.Sprintf("%s GMT", time.Now().UTC().Format(time.RFC1123)))
	resp, err := req.Response()
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
	v, err := httpsig.NewVerifier(r)
	if err != nil {
		return
	}
	ID := v.KeyId()
	idIRI, err := url.Parse(ID)
	if err != nil {
		return
	}
	// 2. Fetch the public key of the other actor
	b, err := fetch(idIRI)
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
