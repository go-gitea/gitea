// Copyright 2012 Google Inc. All Rights Reserved.
// Copyright 2014 The Macaron Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package context

import (
	"encoding/base64"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

const (
	KEY       = "quay"
	USER_ID   = "12345678"
	ACTION_ID = "POST /form"
)

var (
	now              = time.Now()
	oneMinuteFromNow = now.Add(1 * time.Minute)
)

func Test_ValidToken(t *testing.T) {
	Convey("Validate token", t, func() {
		tok := generateTokenAtTime(KEY, USER_ID, ACTION_ID, now)
		So(validTokenAtTime(tok, KEY, USER_ID, ACTION_ID, oneMinuteFromNow), ShouldBeTrue)
		So(validTokenAtTime(tok, KEY, USER_ID, ACTION_ID, now.Add(TIMEOUT-1*time.Nanosecond)), ShouldBeTrue)
		So(validTokenAtTime(tok, KEY, USER_ID, ACTION_ID, now.Add(-1*time.Minute)), ShouldBeTrue)
	})
}

// Test_SeparatorReplacement tests that separators are being correctly substituted
func Test_SeparatorReplacement(t *testing.T) {
	Convey("Test two separator replacements", t, func() {
		So(generateTokenAtTime("foo:bar", "baz", "wah", now), ShouldNotEqual,
			generateTokenAtTime("foo", "bar:baz", "wah", now))
	})
}

func Test_InvalidToken(t *testing.T) {
	Convey("Test invalid tokens", t, func() {
		invalidTokenTests := []struct {
			name, key, userID, actionID string
			t                           time.Time
		}{
			{"Bad key", "foobar", USER_ID, ACTION_ID, oneMinuteFromNow},
			{"Bad userID", KEY, "foobar", ACTION_ID, oneMinuteFromNow},
			{"Bad actionID", KEY, USER_ID, "foobar", oneMinuteFromNow},
			{"Expired", KEY, USER_ID, ACTION_ID, now.Add(TIMEOUT)},
			{"More than 1 minute from the future", KEY, USER_ID, ACTION_ID, now.Add(-1*time.Nanosecond - 1*time.Minute)},
		}

		tok := generateTokenAtTime(KEY, USER_ID, ACTION_ID, now)
		for _, itt := range invalidTokenTests {
			So(validTokenAtTime(tok, itt.key, itt.userID, itt.actionID, itt.t), ShouldBeFalse)
		}
	})
}

// Test_ValidateBadData primarily tests that no unexpected panics are triggered during parsing
func Test_ValidateBadData(t *testing.T) {
	Convey("Validate bad data", t, func() {
		badDataTests := []struct {
			name, tok string
		}{
			{"Invalid Base64", "ASDab24(@)$*=="},
			{"No delimiter", base64.URLEncoding.EncodeToString([]byte("foobar12345678"))},
			{"Invalid time", base64.URLEncoding.EncodeToString([]byte("foobar:foobar"))},
		}

		for _, bdt := range badDataTests {
			So(validTokenAtTime(bdt.tok, KEY, USER_ID, ACTION_ID, oneMinuteFromNow), ShouldBeFalse)
		}
	})
}
