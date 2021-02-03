// Copyright 2021 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package externalaccount

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"path"
	"sort"
	"strings"
	"time"
)

// RequestSigner is a utility class to sign http requests using a AWS V4 signature.
type awsRequestSigner struct {
	RegionName             string
	AwsSecurityCredentials map[string]string
}

const (
// AWS Signature Version 4 signing algorithm identifier.
	awsAlgorithm = "AWS4-HMAC-SHA256"

// The termination string for the AWS credential scope value as defined in
// https://docs.aws.amazon.com/general/latest/gr/sigv4-create-string-to-sign.html
	awsRequestType = "aws4_request"

// The AWS authorization header name for the security session token if available.
	awsSecurityTokenHeader = "x-amz-security-token"

// The AWS authorization header name for the auto-generated date.
	awsDateHeader = "x-amz-date"

	awsTimeFormatLong = "20060102T150405Z"
	awsTimeFormatShort = "20060102"
)

func getSha256(input []byte) (string, error) {
	hash := sha256.New()
	if _, err := hash.Write(input); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

func getHmacSha256(key, input []byte) ([]byte, error) {
	hash := hmac.New(sha256.New, key)
	if _, err := hash.Write(input); err != nil {
		return nil, err
	}
	return hash.Sum(nil), nil
}

func cloneRequest(r *http.Request) *http.Request {
	r2 := new(http.Request)
	*r2 = *r
	if r.Header != nil {
		r2.Header = make(http.Header, len(r.Header))

		// Find total number of values.
		headerCount := 0
		for _, headerValues := range r.Header {
			headerCount += len(headerValues)
		}
		copiedHeaders := make([]string, headerCount) // shared backing array for headers' values

		for headerKey, headerValues := range r.Header {
			headerCount = copy(copiedHeaders, headerValues)
			r2.Header[headerKey] = copiedHeaders[:headerCount:headerCount]
			copiedHeaders = copiedHeaders[headerCount:]
		}
	}
	return r2
}

func canonicalPath(req *http.Request) string {
	result := req.URL.EscapedPath()
	if result == "" {
		return "/"
	}
	return path.Clean(result)
}

func canonicalQuery(req *http.Request) string {
	queryValues := req.URL.Query()
	for queryKey := range queryValues {
		sort.Strings(queryValues[queryKey])
	}
	return queryValues.Encode()
}

func canonicalHeaders(req *http.Request) (string, string) {
	// Header keys need to be sorted alphabetically.
	var headers []string
	lowerCaseHeaders := make(http.Header)
	for k, v := range req.Header {
		k := strings.ToLower(k)
		if _, ok := lowerCaseHeaders[k]; ok {
			// include additional values
			lowerCaseHeaders[k] = append(lowerCaseHeaders[k], v...)
		} else {
			headers = append(headers, k)
			lowerCaseHeaders[k] = v
		}
	}
	sort.Strings(headers)

	var fullHeaders strings.Builder
	for _, header := range headers {
		headerValue := strings.Join(lowerCaseHeaders[header], ",")
		fullHeaders.WriteString(header)
		fullHeaders.WriteRune(':')
		fullHeaders.WriteString(headerValue)
		fullHeaders.WriteRune('\n')
	}

	return strings.Join(headers, ";"), fullHeaders.String()
}

func requestDataHash(req *http.Request) (string, error) {
	var requestData []byte
	if req.Body != nil {
		requestBody, err := req.GetBody()
		if err != nil {
			return "", err
		}
		defer requestBody.Close()

		requestData, err = ioutil.ReadAll(io.LimitReader(requestBody, 1<<20))
		if err != nil {
			return "", err
		}
	}

	return getSha256(requestData)
}

func requestHost(req *http.Request) string {
	if req.Host != "" {
		return req.Host
	}
	return req.URL.Host
}

func canonicalRequest(req *http.Request, canonicalHeaderColumns, canonicalHeaderData string) (string, error) {
	dataHash, err := requestDataHash(req)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s\n%s\n%s\n%s\n%s\n%s", req.Method, canonicalPath(req), canonicalQuery(req), canonicalHeaderData, canonicalHeaderColumns, dataHash), nil
}

// SignRequest adds the appropriate headers to an http.Request
// or returns an error if something prevented this.
func (rs *awsRequestSigner) SignRequest(req *http.Request) error {
	signedRequest := cloneRequest(req)
	timestamp := now()

	signedRequest.Header.Add("host", requestHost(req))

	if securityToken, ok := rs.AwsSecurityCredentials["security_token"]; ok {
		signedRequest.Header.Add(awsSecurityTokenHeader, securityToken)
	}

	if signedRequest.Header.Get("date") == "" {
		signedRequest.Header.Add(awsDateHeader, timestamp.Format(awsTimeFormatLong))
	}

	authorizationCode, err := rs.generateAuthentication(signedRequest, timestamp)
	if err != nil {
		return err
	}
	signedRequest.Header.Set("Authorization", authorizationCode)

	req.Header = signedRequest.Header
	return nil
}

func (rs *awsRequestSigner) generateAuthentication(req *http.Request, timestamp time.Time) (string, error) {
	secretAccessKey, ok := rs.AwsSecurityCredentials["secret_access_key"]
	if !ok {
		return "", errors.New("oauth2/google: missing secret_access_key header")
	}
	accessKeyId, ok := rs.AwsSecurityCredentials["access_key_id"]
	if !ok {
		return "", errors.New("oauth2/google: missing access_key_id header")
	}

	canonicalHeaderColumns, canonicalHeaderData := canonicalHeaders(req)

	dateStamp := timestamp.Format(awsTimeFormatShort)
	serviceName := ""
	if splitHost := strings.Split(requestHost(req), "."); len(splitHost) > 0 {
		serviceName = splitHost[0]
	}

	credentialScope := fmt.Sprintf("%s/%s/%s/%s",dateStamp, rs.RegionName, serviceName, awsRequestType)

	requestString, err := canonicalRequest(req, canonicalHeaderColumns, canonicalHeaderData)
	if err != nil {
		return "", err
	}
	requestHash, err := getSha256([]byte(requestString))
	if err != nil{
		return "", err
	}

	stringToSign := fmt.Sprintf("%s\n%s\n%s\n%s", awsAlgorithm, timestamp.Format(awsTimeFormatLong), credentialScope, requestHash)

	signingKey := []byte("AWS4" + secretAccessKey)
	for _, signingInput := range []string{
		dateStamp, rs.RegionName, serviceName, awsRequestType, stringToSign,
	} {
		signingKey, err = getHmacSha256(signingKey, []byte(signingInput))
		if err != nil{
			return "", err
		}
	}

	return fmt.Sprintf("%s Credential=%s/%s, SignedHeaders=%s, Signature=%s", awsAlgorithm, accessKeyId, credentialScope, canonicalHeaderColumns, hex.EncodeToString(signingKey)), nil
}
