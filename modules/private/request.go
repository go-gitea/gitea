// Copyright 2023 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package private

import (
	"fmt"
	"io"
	"net/http"

	"code.gitea.io/gitea/modules/httplib"
	"code.gitea.io/gitea/modules/json"
)

// responseText is used to get the response as text, instead of parsing it as JSON.
type responseText struct {
	Text string
}

// ResponseExtra contains extra information about the response, especially for error responses.
type ResponseExtra struct {
	StatusCode int
	UserMsg    string
	Error      error
}

type responseCallback struct {
	Callback func(resp *http.Response, extra *ResponseExtra)
}

func (re *ResponseExtra) HasError() bool {
	return re.Error != nil
}

type responseError struct {
	statusCode  int
	errorString string
}

func (re responseError) Error() string {
	if re.errorString == "" {
		return fmt.Sprintf("internal API error response, status=%d", re.statusCode)
	}
	return fmt.Sprintf("internal API error response, status=%d, err=%s", re.statusCode, re.errorString)
}

// requestJSONResp sends a request to the gitea server and then parses the response.
// If the status code is not 2xx, or any error occurs, the ResponseExtra.Error field is guaranteed to be non-nil,
// and the ResponseExtra.UserMsg field will be set to a message for the end user.
//
// * If the "res" is a struct pointer, the response will be parsed as JSON
// * If the "res" is responseText pointer, the response will be stored as text in it
// * If the "res" is responseCallback pointer, the callback function should set the ResponseExtra fields accordingly
func requestJSONResp[T any](req *httplib.Request, res *T) (ret *T, extra ResponseExtra) {
	resp, err := req.Response()
	if err != nil {
		extra.UserMsg = "Internal Server Connection Error"
		extra.Error = fmt.Errorf("unable to contact gitea %q: %w", req.GoString(), err)
		return nil, extra
	}
	defer resp.Body.Close()

	extra.StatusCode = resp.StatusCode

	// if the status code is not 2xx, try to parse the error response
	if resp.StatusCode/100 != 2 {
		var respErr Response
		if err := json.NewDecoder(resp.Body).Decode(&respErr); err != nil {
			extra.UserMsg = "Internal Server Error Decoding Failed"
			extra.Error = fmt.Errorf("unable to decode error response %q: %w", req.GoString(), err)
			return nil, extra
		}
		extra.UserMsg = respErr.UserMsg
		if extra.UserMsg == "" {
			extra.UserMsg = "Internal Server Error (no message for end users)"
		}
		extra.Error = responseError{statusCode: resp.StatusCode, errorString: respErr.Err}
		return res, extra
	}

	// now, the StatusCode must be 2xx
	var v any = res
	if respText, ok := v.(*responseText); ok {
		// get the whole response as a text string
		bs, err := io.ReadAll(resp.Body)
		if err != nil {
			extra.UserMsg = "Internal Server Response Reading Failed"
			extra.Error = fmt.Errorf("unable to read response %q: %w", req.GoString(), err)
			return nil, extra
		}
		respText.Text = string(bs)
		return res, extra
	} else if cb, ok := v.(*responseCallback); ok {
		// pass the response to callback, and let the callback update the ResponseExtra
		extra.StatusCode = resp.StatusCode
		cb.Callback(resp, &extra)
		return nil, extra
	} else if err := json.NewDecoder(resp.Body).Decode(res); err != nil {
		// decode the response into the given struct
		extra.UserMsg = "Internal Server Response Decoding Failed"
		extra.Error = fmt.Errorf("unable to decode response %q: %w", req.GoString(), err)
		return nil, extra
	}

	if respMsg, ok := v.(*Response); ok {
		// if the "res" is Response structure, try to get the UserMsg from it and update the ResponseExtra
		extra.UserMsg = respMsg.UserMsg
		if respMsg.Err != "" {
			// usually this shouldn't happen, because the StatusCode is 2xx, there should be no error.
			// but we still handle the "err" response, in case some people return error messages by status code 200.
			extra.Error = responseError{statusCode: resp.StatusCode, errorString: respMsg.Err}
		}
	}

	return res, extra
}

// requestJSONClientMsg sends a request to the gitea server, server only responds text message status=200 with "success" body
// If the request succeeds (200), the argument clientSuccessMsg will be used as ResponseExtra.UserMsg.
func requestJSONClientMsg(req *httplib.Request, clientSuccessMsg string) ResponseExtra {
	_, extra := requestJSONResp(req, &responseText{})
	if extra.HasError() {
		return extra
	}
	extra.UserMsg = clientSuccessMsg
	return extra
}
