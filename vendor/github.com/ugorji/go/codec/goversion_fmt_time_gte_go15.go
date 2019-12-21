// Copyright (c) 2012-2018 Ugorji Nwoke. All rights reserved.
// Use of this source code is governed by a MIT license found in the LICENSE file.

// +build go1.5

package codec

import "time"

func fmtTime(t time.Time, b []byte) []byte {
	return t.AppendFormat(b, time.RFC3339Nano)
}
