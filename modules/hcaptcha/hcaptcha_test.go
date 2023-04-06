// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package hcaptcha

import (
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

const (
	dummySiteKey = "10000000-ffff-ffff-ffff-000000000001"
	dummySecret  = "0x0000000000000000000000000000000000000000"
	dummyToken   = "10000000-aaaa-bbbb-cccc-000000000001"
)

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}

func TestCaptcha(t *testing.T) {
	tt := []struct {
		Name   string
		Secret string
		Token  string
		Error  ErrorCode
	}{
		{
			Name:   "Success",
			Secret: dummySecret,
			Token:  dummyToken,
		},
		{
			Name:  "Missing Secret",
			Token: dummyToken,
			Error: ErrMissingInputSecret,
		},
		{
			Name:   "Missing Token",
			Secret: dummySecret,
			Error:  ErrMissingInputResponse,
		},
		{
			Name:   "Invalid Token",
			Secret: dummySecret,
			Token:  "test",
			Error:  ErrInvalidInputResponse,
		},
	}

	for _, tc := range tt {
		t.Run(tc.Name, func(t *testing.T) {
			client, err := New(tc.Secret, WithHTTP(&http.Client{
				Timeout: time.Second * 5,
			}))
			if err != nil {
				// The only error that can be returned from creating a client
				if tc.Error == ErrMissingInputSecret && err == ErrMissingInputSecret {
					return
				}
				t.Log(err)
				t.FailNow()
			}

			resp, err := client.Verify(tc.Token, PostOptions{
				Sitekey: dummySiteKey,
			})
			if err != nil {
				// The only error that can be returned prior to the request
				if tc.Error == ErrMissingInputResponse && err == ErrMissingInputResponse {
					return
				}
				t.Log(err)
				t.FailNow()
			}

			if tc.Error.String() != "" {
				if resp.Success {
					t.Log("Verification should fail.")
					t.Fail()
				}
				if len(resp.ErrorCodes) == 0 {
					t.Log("hCaptcha should have returned an error.")
					t.Fail()
				}
				var hasErr bool
				for _, err := range resp.ErrorCodes {
					if strings.EqualFold(err.String(), tc.Error.String()) {
						hasErr = true
						break
					}
				}
				if !hasErr {
					t.Log("hCaptcha did not return the error being tested")
					t.Fail()
				}
			} else if !resp.Success {
				t.Log("Verification should succeed.")
				t.Fail()
			}
		})
	}
}
