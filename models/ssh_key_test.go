// Copyright 2016 The Gogs Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"fmt"
	"strings"
	"testing"

	. "github.com/smartystreets/goconvey/convey"

	"code.gitea.io/gitea/modules/setting"
)

func init() {
	setting.NewContext()
}

func Test_SSHParsePublicKey(t *testing.T) {
	testKeys := map[string]struct {
		typeName string
		length   int
		content  string
	}{
		"dsa-1024":  {"dsa", 1024, "ssh-dss AAAAB3NzaC1kc3MAAACBAOChCC7lf6Uo9n7BmZ6M8St19PZf4Tn59NriyboW2x/DZuYAz3ibZ2OkQ3S0SqDIa0HXSEJ1zaExQdmbO+Ux/wsytWZmCczWOVsaszBZSl90q8UnWlSH6P+/YA+RWJm5SFtuV9PtGIhyZgoNuz5kBQ7K139wuQsecdKktISwTakzAAAAFQCzKsO2JhNKlL+wwwLGOcLffoAmkwAAAIBpK7/3xvduajLBD/9vASqBQIHrgK2J+wiQnIb/Wzy0UsVmvfn8A+udRbBo+csM8xrSnlnlJnjkJS3qiM5g+eTwsLIV1IdKPEwmwB+VcP53Cw6lSyWyJcvhFb0N6s08NZysLzvj0N+ZC/FnhKTLzIyMtkHf/IrPCwlM+pV/M/96YgAAAIEAqQcGn9CKgzgPaguIZooTAOQdvBLMI5y0bQjOW6734XOpqQGf/Kra90wpoasLKZjSYKNPjE+FRUOrStLrxcNs4BeVKhy2PYTRnybfYVk1/dmKgH6P1YSRONsGKvTsH6c5IyCRG0ncCgYeF8tXppyd642982daopE7zQ/NPAnJfag= nocomment"},
		"rsa-1024":  {"rsa", 1024, "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDAu7tvIvX6ZHrRXuZNfkR3XLHSsuCK9Zn3X58lxBcQzuo5xZgB6vRwwm/QtJuF+zZPtY5hsQILBLmF+BZ5WpKZp1jBeSjH2G7lxet9kbcH+kIVj0tPFEoyKI9wvWqIwC4prx/WVk2wLTJjzBAhyNxfEq7C9CeiX9pQEbEqJfkKCQ== nocomment\n"},
		"rsa-2048":  {"rsa", 2048, "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDMZXh+1OBUwSH9D45wTaxErQIN9IoC9xl7MKJkqvTvv6O5RR9YW/IK9FbfjXgXsppYGhsCZo1hFOOsXHMnfOORqu/xMDx4yPuyvKpw4LePEcg4TDipaDFuxbWOqc/BUZRZcXu41QAWfDLrInwsltWZHSeG7hjhpacl4FrVv9V1pS6Oc5Q1NxxEzTzuNLS/8diZrTm/YAQQ/+B+mzWI3zEtF4miZjjAljWd1LTBPvU23d29DcBmmFahcZ441XZsTeAwGxG/Q6j8NgNXj9WxMeWwxXV2jeAX/EBSpZrCVlCQ1yJswT6xCp8TuBnTiGWYMBNTbOZvPC4e0WI2/yZW/s5F nocomment"},
		"ecdsa-256": {"ecdsa", 256, "ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBFQacN3PrOll7PXmN5B/ZNVahiUIqI05nbBlZk1KXsO3d06ktAWqbNflv2vEmA38bTFTfJ2sbn2B5ksT52cDDbA= nocomment"},
		"ecdsa-384": {"ecdsa", 384, "ecdsa-sha2-nistp384 AAAAE2VjZHNhLXNoYTItbmlzdHAzODQAAAAIbmlzdHAzODQAAABhBINmioV+XRX1Fm9Qk2ehHXJ2tfVxW30ypUWZw670Zyq5GQfBAH6xjygRsJ5wWsHXBsGYgFUXIHvMKVAG1tpw7s6ax9oA+dJOJ7tj+vhn8joFqT+sg3LYHgZkHrfqryRasQ== nocomment"},
		// "ecdsa-521": {"ecdsa", 521, "ecdsa-sha2-nistp521 AAAAE2VjZHNhLXNoYTItbmlzdHA1MjEAAAAIbmlzdHA1MjEAAACFBACGt3UG3EzRwNOI17QR84l6PgiAcvCE7v6aXPj/SC6UWKg4EL8vW9ZBcdYL9wzs4FZXh4MOV8jAzu3KRWNTwb4k2wFNUpGOt7l28MztFFEtH5BDDrtAJSPENPy8pvPLMfnPg5NhvWycqIBzNcHipem5wSJFN5PdpNOC2xMrPWKNqj+ZjQ== nocomment"},
	}

	Convey("Parse public keys in both native and ssh-keygen", t, func() {
		for name, key := range testKeys {
			fmt.Println("\nTesting key:", name)

			keyTypeN, lengthN, errN := SSHNativeParsePublicKey(key.content)
			So(errN, ShouldBeNil)
			So(keyTypeN, ShouldEqual, key.typeName)
			So(lengthN, ShouldEqual, key.length)

			keyTypeK, lengthK, errK := SSHKeyGenParsePublicKey(key.content)
			if errK != nil {
				// Some server just does not support ecdsa format.
				if strings.Contains(errK.Error(), "line 1 too long:") {
					continue
				}
				So(errK, ShouldBeNil)
			}
			So(keyTypeK, ShouldEqual, key.typeName)
			So(lengthK, ShouldEqual, key.length)
		}
	})
}
