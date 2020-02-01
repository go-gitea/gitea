// Copyright 2020 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package private

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"code.gitea.io/gitea/modules/setting"
)

// Shutdown calls the internal shutdown function
func Shutdown() (int, string) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/manager/shutdown")

	req := newInternalRequest(reqURL, "POST")
	resp, err := req.Response()
	if err != nil {
		return http.StatusInternalServerError, fmt.Sprintf("Unable to contact gitea: %v", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, decodeJSONError(resp).Err
	}

	return http.StatusOK, "Shutting down"
}

// Restart calls the internal restart function
func Restart() (int, string) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/manager/restart")

	req := newInternalRequest(reqURL, "POST")
	resp, err := req.Response()
	if err != nil {
		return http.StatusInternalServerError, fmt.Sprintf("Unable to contact gitea: %v", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, decodeJSONError(resp).Err
	}

	return http.StatusOK, "Restarting"
}

// FlushOptions represents the options for the flush call
type FlushOptions struct {
	Timeout     time.Duration
	NonBlocking bool
}

// FlushQueues calls the internal flush-queues function
func FlushQueues(timeout time.Duration, nonBlocking bool) (int, string) {
	reqURL := setting.LocalURL + fmt.Sprintf("api/internal/manager/flush-queues")

	req := newInternalRequest(reqURL, "POST")
	if timeout > 0 {
		req.SetTimeout(timeout+10*time.Second, timeout+10*time.Second)
	}
	req = req.Header("Content-Type", "application/json")
	jsonBytes, _ := json.Marshal(FlushOptions{
		Timeout:     timeout,
		NonBlocking: nonBlocking,
	})
	req.Body(jsonBytes)
	resp, err := req.Response()
	if err != nil {
		return http.StatusInternalServerError, fmt.Sprintf("Unable to contact gitea: %v", err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, decodeJSONError(resp).Err
	}

	return http.StatusOK, "Flushed"
}
