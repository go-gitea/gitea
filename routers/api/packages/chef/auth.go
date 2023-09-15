// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package chef

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"hash"
	"math/big"
	"net/http"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	user_model "code.gitea.io/gitea/models/user"
	chef_module "code.gitea.io/gitea/modules/packages/chef"
	"code.gitea.io/gitea/modules/util"
	"code.gitea.io/gitea/services/auth"

	"github.com/minio/sha256-simd"
)

const (
	maxTimeDifference = 10 * time.Minute
)

var (
	algorithmPattern     = regexp.MustCompile(`algorithm=(\w+)`)
	versionPattern       = regexp.MustCompile(`version=(\d+\.\d+)`)
	authorizationPattern = regexp.MustCompile(`\AX-Ops-Authorization-(\d+)`)

	_ auth.Method = &Auth{}
)

// Documentation:
// https://docs.chef.io/server/api_chef_server/#required-headers
// https://github.com/chef-boneyard/chef-rfc/blob/master/rfc065-sign-v1.3.md
// https://github.com/chef/mixlib-authentication/blob/bc8adbef833d4be23dc78cb23e6fe44b51ebc34f/lib/mixlib/authentication/signedheaderauth.rb

type Auth struct{}

func (a *Auth) Name() string {
	return "chef"
}

// Verify extracts the user from the signed request
// If the request is signed with the user private key the user is verified.
func (a *Auth) Verify(req *http.Request, w http.ResponseWriter, store auth.DataStore, sess auth.SessionStore) (*user_model.User, error) {
	u, err := getUserFromRequest(req)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, nil
	}

	pub, err := getUserPublicKey(req.Context(), u)
	if err != nil {
		return nil, err
	}

	if err := verifyTimestamp(req); err != nil {
		return nil, err
	}

	version, err := getSignVersion(req)
	if err != nil {
		return nil, err
	}

	if err := verifySignedHeaders(req, version, pub.(*rsa.PublicKey)); err != nil {
		return nil, err
	}

	return u, nil
}

func getUserFromRequest(req *http.Request) (*user_model.User, error) {
	username := req.Header.Get("X-Ops-Userid")
	if username == "" {
		return nil, nil
	}

	return user_model.GetUserByName(req.Context(), username)
}

func getUserPublicKey(ctx context.Context, u *user_model.User) (crypto.PublicKey, error) {
	pubKey, err := user_model.GetSetting(ctx, u.ID, chef_module.SettingPublicPem)
	if err != nil {
		return nil, err
	}

	pubPem, _ := pem.Decode([]byte(pubKey))

	return x509.ParsePKIXPublicKey(pubPem.Bytes)
}

func verifyTimestamp(req *http.Request) error {
	hdr := req.Header.Get("X-Ops-Timestamp")
	if hdr == "" {
		return util.NewInvalidArgumentErrorf("X-Ops-Timestamp header missing")
	}

	ts, err := time.Parse(time.RFC3339, hdr)
	if err != nil {
		return err
	}

	diff := time.Now().UTC().Sub(ts)
	if diff < 0 {
		diff = -diff
	}

	if diff > maxTimeDifference {
		return fmt.Errorf("time difference")
	}

	return nil
}

func getSignVersion(req *http.Request) (string, error) {
	hdr := req.Header.Get("X-Ops-Sign")
	if hdr == "" {
		return "", util.NewInvalidArgumentErrorf("X-Ops-Sign header missing")
	}

	m := versionPattern.FindStringSubmatch(hdr)
	if len(m) != 2 {
		return "", util.NewInvalidArgumentErrorf("invalid X-Ops-Sign header")
	}

	switch m[1] {
	case "1.0", "1.1", "1.2", "1.3":
	default:
		return "", util.NewInvalidArgumentErrorf("unsupported version")
	}

	version := m[1]

	m = algorithmPattern.FindStringSubmatch(hdr)
	if len(m) == 2 && m[1] != "sha1" && !(m[1] == "sha256" && version == "1.3") {
		return "", util.NewInvalidArgumentErrorf("unsupported algorithm")
	}

	return version, nil
}

func verifySignedHeaders(req *http.Request, version string, pub *rsa.PublicKey) error {
	authorizationData, err := getAuthorizationData(req)
	if err != nil {
		return err
	}

	checkData := buildCheckData(req, version)

	switch version {
	case "1.3":
		return verifyDataNew(authorizationData, checkData, pub, crypto.SHA256)
	case "1.2":
		return verifyDataNew(authorizationData, checkData, pub, crypto.SHA1)
	default:
		return verifyDataOld(authorizationData, checkData, pub)
	}
}

func getAuthorizationData(req *http.Request) ([]byte, error) {
	valueList := make(map[int]string)
	for k, vs := range req.Header {
		if m := authorizationPattern.FindStringSubmatch(k); m != nil {
			index, _ := strconv.Atoi(m[1])
			var v string
			if len(vs) == 0 {
				v = ""
			} else {
				v = vs[0]
			}
			valueList[index] = v
		}
	}

	tmp := make([]string, len(valueList))
	for k, v := range valueList {
		if k > len(tmp) {
			return nil, fmt.Errorf("invalid X-Ops-Authorization headers")
		}
		tmp[k-1] = v
	}

	return base64.StdEncoding.DecodeString(strings.Join(tmp, ""))
}

func buildCheckData(req *http.Request, version string) []byte {
	username := req.Header.Get("X-Ops-Userid")
	if version != "1.0" && version != "1.3" {
		sum := sha1.Sum([]byte(username))
		username = base64.StdEncoding.EncodeToString(sum[:])
	}

	var data string
	if version == "1.3" {
		data = fmt.Sprintf(
			"Method:%s\nPath:%s\nX-Ops-Content-Hash:%s\nX-Ops-Sign:version=%s\nX-Ops-Timestamp:%s\nX-Ops-UserId:%s\nX-Ops-Server-API-Version:%s",
			req.Method,
			path.Clean(req.URL.Path),
			req.Header.Get("X-Ops-Content-Hash"),
			version,
			req.Header.Get("X-Ops-Timestamp"),
			username,
			req.Header.Get("X-Ops-Server-Api-Version"),
		)
	} else {
		sum := sha1.Sum([]byte(path.Clean(req.URL.Path)))
		data = fmt.Sprintf(
			"Method:%s\nHashed Path:%s\nX-Ops-Content-Hash:%s\nX-Ops-Timestamp:%s\nX-Ops-UserId:%s",
			req.Method,
			base64.StdEncoding.EncodeToString(sum[:]),
			req.Header.Get("X-Ops-Content-Hash"),
			req.Header.Get("X-Ops-Timestamp"),
			username,
		)
	}

	return []byte(data)
}

func verifyDataNew(signature, data []byte, pub *rsa.PublicKey, algo crypto.Hash) error {
	var h hash.Hash
	if algo == crypto.SHA256 {
		h = sha256.New()
	} else {
		h = sha1.New()
	}
	if _, err := h.Write(data); err != nil {
		return err
	}

	return rsa.VerifyPKCS1v15(pub, algo, h.Sum(nil), signature)
}

func verifyDataOld(signature, data []byte, pub *rsa.PublicKey) error {
	c := new(big.Int)
	m := new(big.Int)
	m.SetBytes(signature)
	e := big.NewInt(int64(pub.E))
	c.Exp(m, e, pub.N)

	out := c.Bytes()

	skip := 0
	for i := 2; i < len(out); i++ {
		if i+1 >= len(out) {
			break
		}
		if out[i] == 0xFF && out[i+1] == 0 {
			skip = i + 2
			break
		}
	}

	if !slices.Equal(out[skip:], data) {
		return fmt.Errorf("could not verify signature")
	}

	return nil
}
