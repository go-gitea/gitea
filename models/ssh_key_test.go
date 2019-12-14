// Copyright 2016 The Gogs Authors. All rights reserved.
// Copyright 2019 The Gitea Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package models

import (
	"strings"
	"testing"

	"code.gitea.io/gitea/modules/setting"

	"github.com/stretchr/testify/assert"
)

func init() {
	setting.SetCustomPathAndConf("", "", "")
	setting.NewContext()
}

func Test_SSHParsePublicKey(t *testing.T) {
	testCases := []struct {
		name    string
		keyType string
		length  int
		content string
	}{
		{"dsa-1024", "dsa", 1024, "ssh-dss AAAAB3NzaC1kc3MAAACBAOChCC7lf6Uo9n7BmZ6M8St19PZf4Tn59NriyboW2x/DZuYAz3ibZ2OkQ3S0SqDIa0HXSEJ1zaExQdmbO+Ux/wsytWZmCczWOVsaszBZSl90q8UnWlSH6P+/YA+RWJm5SFtuV9PtGIhyZgoNuz5kBQ7K139wuQsecdKktISwTakzAAAAFQCzKsO2JhNKlL+wwwLGOcLffoAmkwAAAIBpK7/3xvduajLBD/9vASqBQIHrgK2J+wiQnIb/Wzy0UsVmvfn8A+udRbBo+csM8xrSnlnlJnjkJS3qiM5g+eTwsLIV1IdKPEwmwB+VcP53Cw6lSyWyJcvhFb0N6s08NZysLzvj0N+ZC/FnhKTLzIyMtkHf/IrPCwlM+pV/M/96YgAAAIEAqQcGn9CKgzgPaguIZooTAOQdvBLMI5y0bQjOW6734XOpqQGf/Kra90wpoasLKZjSYKNPjE+FRUOrStLrxcNs4BeVKhy2PYTRnybfYVk1/dmKgH6P1YSRONsGKvTsH6c5IyCRG0ncCgYeF8tXppyd642982daopE7zQ/NPAnJfag= nocomment"},
		{"rsa-1024", "rsa", 1024, "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDAu7tvIvX6ZHrRXuZNfkR3XLHSsuCK9Zn3X58lxBcQzuo5xZgB6vRwwm/QtJuF+zZPtY5hsQILBLmF+BZ5WpKZp1jBeSjH2G7lxet9kbcH+kIVj0tPFEoyKI9wvWqIwC4prx/WVk2wLTJjzBAhyNxfEq7C9CeiX9pQEbEqJfkKCQ== nocomment\n"},
		{"rsa-2048", "rsa", 2048, "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDMZXh+1OBUwSH9D45wTaxErQIN9IoC9xl7MKJkqvTvv6O5RR9YW/IK9FbfjXgXsppYGhsCZo1hFOOsXHMnfOORqu/xMDx4yPuyvKpw4LePEcg4TDipaDFuxbWOqc/BUZRZcXu41QAWfDLrInwsltWZHSeG7hjhpacl4FrVv9V1pS6Oc5Q1NxxEzTzuNLS/8diZrTm/YAQQ/+B+mzWI3zEtF4miZjjAljWd1LTBPvU23d29DcBmmFahcZ441XZsTeAwGxG/Q6j8NgNXj9WxMeWwxXV2jeAX/EBSpZrCVlCQ1yJswT6xCp8TuBnTiGWYMBNTbOZvPC4e0WI2/yZW/s5F nocomment"},
		{"ecdsa-256", "ecdsa", 256, "ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBFQacN3PrOll7PXmN5B/ZNVahiUIqI05nbBlZk1KXsO3d06ktAWqbNflv2vEmA38bTFTfJ2sbn2B5ksT52cDDbA= nocomment"},
		{"ecdsa-384", "ecdsa", 384, "ecdsa-sha2-nistp384 AAAAE2VjZHNhLXNoYTItbmlzdHAzODQAAAAIbmlzdHAzODQAAABhBINmioV+XRX1Fm9Qk2ehHXJ2tfVxW30ypUWZw670Zyq5GQfBAH6xjygRsJ5wWsHXBsGYgFUXIHvMKVAG1tpw7s6ax9oA+dJOJ7tj+vhn8joFqT+sg3LYHgZkHrfqryRasQ== nocomment"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("Native", func(t *testing.T) {
				keyTypeN, lengthN, err := SSHNativeParsePublicKey(tc.content)
				assert.NoError(t, err)
				assert.Equal(t, tc.keyType, keyTypeN)
				assert.EqualValues(t, tc.length, lengthN)
			})
			t.Run("SSHKeygen", func(t *testing.T) {
				keyTypeK, lengthK, err := SSHKeyGenParsePublicKey(tc.content)
				if err != nil {
					// Some servers do not support ecdsa format.
					if !strings.Contains(err.Error(), "line 1 too long:") {
						assert.Fail(t, "%v", err)
					}
				}
				assert.Equal(t, tc.keyType, keyTypeK)
				assert.EqualValues(t, tc.length, lengthK)
			})
		})
	}
}

func Test_CheckPublicKeyString(t *testing.T) {
	for _, test := range []struct {
		content string
	}{
		{"ssh-dss AAAAB3NzaC1kc3MAAACBAOChCC7lf6Uo9n7BmZ6M8St19PZf4Tn59NriyboW2x/DZuYAz3ibZ2OkQ3S0SqDIa0HXSEJ1zaExQdmbO+Ux/wsytWZmCczWOVsaszBZSl90q8UnWlSH6P+/YA+RWJm5SFtuV9PtGIhyZgoNuz5kBQ7K139wuQsecdKktISwTakzAAAAFQCzKsO2JhNKlL+wwwLGOcLffoAmkwAAAIBpK7/3xvduajLBD/9vASqBQIHrgK2J+wiQnIb/Wzy0UsVmvfn8A+udRbBo+csM8xrSnlnlJnjkJS3qiM5g+eTwsLIV1IdKPEwmwB+VcP53Cw6lSyWyJcvhFb0N6s08NZysLzvj0N+ZC/FnhKTLzIyMtkHf/IrPCwlM+pV/M/96YgAAAIEAqQcGn9CKgzgPaguIZooTAOQdvBLMI5y0bQjOW6734XOpqQGf/Kra90wpoasLKZjSYKNPjE+FRUOrStLrxcNs4BeVKhy2PYTRnybfYVk1/dmKgH6P1YSRONsGKvTsH6c5IyCRG0ncCgYeF8tXppyd642982daopE7zQ/NPAnJfag= nocomment"},
		{"ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDAu7tvIvX6ZHrRXuZNfkR3XLHSsuCK9Zn3X58lxBcQzuo5xZgB6vRwwm/QtJuF+zZPtY5hsQILBLmF+BZ5WpKZp1jBeSjH2G7lxet9kbcH+kIVj0tPFEoyKI9wvWqIwC4prx/WVk2wLTJjzBAhyNxfEq7C9CeiX9pQEbEqJfkKCQ== nocomment\n"},
		{"ssh-rsa AAAAB3NzaC1yc2EA\r\nAAADAQABAAAAgQDAu7tvIvX6ZHrRXuZNfkR3XLHSsuCK9Zn3X58lxBcQzuo5xZgB6vRwwm/QtJuF+zZPtY5hsQILBLmF+\r\nBZ5WpKZp1jBeSjH2G7lxet9kbcH+kIVj0tPFEoyKI9wvWqIwC4prx/WVk2wLTJjzBAhyNx\r\nfEq7C9CeiX9pQEbEqJfkKCQ== nocomment\r\n\r\n"},
		{"ssh-rsa AAAAB3NzaC1yc2EA\r\nAAADAQABAAAAgQDAu7tvI\nvX6ZHrRXuZNfkR3XLHSsuCK9Zn3X58lxBcQzuo5xZgB6vRwwm/QtJuF+zZPtY5hsQILBLmF+\r\nBZ5WpKZp1jBeSjH2G7lxet9kbcH+kIVj0tPFEoyKI9wvW\nqIwC4prx/WVk2wLTJjzBAhyNx\r\nfEq7C9CeiX9pQEbEqJfkKCQ== nocomment\r\n\r\n"},
		{"ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAICV0MGX/W9IvLA4FXpIuUcdDcbj5KX4syHgsTy7soVgf"},
		{"\r\nssh-ed25519 \r\nAAAAC3NzaC1lZDI1NTE5AAAAICV0MGX/W9IvLA4FXpIuUcdDcbj5KX4syHgsTy7soVgf\r\n\r\n"},
		{`---- BEGIN SSH2 PUBLIC KEY ----
Comment: "1024-bit DSA, converted by andrew@phaedra from OpenSSH"
AAAAB3NzaC1kc3MAAACBAOChCC7lf6Uo9n7BmZ6M8St19PZf4Tn59NriyboW2x/DZuYAz3
ibZ2OkQ3S0SqDIa0HXSEJ1zaExQdmbO+Ux/wsytWZmCczWOVsaszBZSl90q8UnWlSH6P+/
YA+RWJm5SFtuV9PtGIhyZgoNuz5kBQ7K139wuQsecdKktISwTakzAAAAFQCzKsO2JhNKlL
+wwwLGOcLffoAmkwAAAIBpK7/3xvduajLBD/9vASqBQIHrgK2J+wiQnIb/Wzy0UsVmvfn8
A+udRbBo+csM8xrSnlnlJnjkJS3qiM5g+eTwsLIV1IdKPEwmwB+VcP53Cw6lSyWyJcvhFb
0N6s08NZysLzvj0N+ZC/FnhKTLzIyMtkHf/IrPCwlM+pV/M/96YgAAAIEAqQcGn9CKgzgP
aguIZooTAOQdvBLMI5y0bQjOW6734XOpqQGf/Kra90wpoasLKZjSYKNPjE+FRUOrStLrxc
Ns4BeVKhy2PYTRnybfYVk1/dmKgH6P1YSRONsGKvTsH6c5IyCRG0ncCgYeF8tXppyd6429
82daopE7zQ/NPAnJfag=
---- END SSH2 PUBLIC KEY ----
`},
		{`---- BEGIN SSH2 PUBLIC KEY ----
Comment: "1024-bit RSA, converted by andrew@phaedra from OpenSSH"
AAAAB3NzaC1yc2EAAAADAQABAAAAgQDAu7tvIvX6ZHrRXuZNfkR3XLHSsuCK9Zn3X58lxB
cQzuo5xZgB6vRwwm/QtJuF+zZPtY5hsQILBLmF+BZ5WpKZp1jBeSjH2G7lxet9kbcH+kIV
j0tPFEoyKI9wvWqIwC4prx/WVk2wLTJjzBAhyNxfEq7C9CeiX9pQEbEqJfkKCQ==
---- END SSH2 PUBLIC KEY ----
`},
		{`-----BEGIN RSA PUBLIC KEY-----
MIGJAoGBAMC7u28i9fpketFe5k1+RHdcsdKy4Ir1mfdfnyXEFxDO6jnFmAHq9HDC
b9C0m4X7Nk+1jmGxAgsEuYX4FnlakpmnWMF5KMfYbuXF632Rtwf6QhWPS08USjIo
j3C9aojALimvH9ZWTbAtMmPMECHI3F8SrsL0J6Jf2lARsSol+QoJAgMBAAE=
-----END RSA PUBLIC KEY-----
`},
		{`-----BEGIN PUBLIC KEY-----
MIIBtzCCASsGByqGSM44BAEwggEeAoGBAOChCC7lf6Uo9n7BmZ6M8St19PZf4Tn5
9NriyboW2x/DZuYAz3ibZ2OkQ3S0SqDIa0HXSEJ1zaExQdmbO+Ux/wsytWZmCczW
OVsaszBZSl90q8UnWlSH6P+/YA+RWJm5SFtuV9PtGIhyZgoNuz5kBQ7K139wuQse
cdKktISwTakzAhUAsyrDtiYTSpS/sMMCxjnC336AJpMCgYBpK7/3xvduajLBD/9v
ASqBQIHrgK2J+wiQnIb/Wzy0UsVmvfn8A+udRbBo+csM8xrSnlnlJnjkJS3qiM5g
+eTwsLIV1IdKPEwmwB+VcP53Cw6lSyWyJcvhFb0N6s08NZysLzvj0N+ZC/FnhKTL
zIyMtkHf/IrPCwlM+pV/M/96YgOBhQACgYEAqQcGn9CKgzgPaguIZooTAOQdvBLM
I5y0bQjOW6734XOpqQGf/Kra90wpoasLKZjSYKNPjE+FRUOrStLrxcNs4BeVKhy2
PYTRnybfYVk1/dmKgH6P1YSRONsGKvTsH6c5IyCRG0ncCgYeF8tXppyd642982da
opE7zQ/NPAnJfag=
-----END PUBLIC KEY-----
`},
		{`-----BEGIN PUBLIC KEY-----
MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDAu7tvIvX6ZHrRXuZNfkR3XLHS
suCK9Zn3X58lxBcQzuo5xZgB6vRwwm/QtJuF+zZPtY5hsQILBLmF+BZ5WpKZp1jB
eSjH2G7lxet9kbcH+kIVj0tPFEoyKI9wvWqIwC4prx/WVk2wLTJjzBAhyNxfEq7C
9CeiX9pQEbEqJfkKCQIDAQAB
-----END PUBLIC KEY-----
`},
		{`-----BEGIN PUBLIC KEY-----
MIIBIjANBgkqhkiG9w0BAQEFAAOCAQ8AMIIBCgKCAQEAzGV4ftTgVMEh/Q+OcE2s
RK0CDfSKAvcZezCiZKr077+juUUfWFvyCvRW3414F7KaWBobAmaNYRTjrFxzJ3zj
karv8TA8eMj7sryqcOC3jxHIOEw4qWgxbsW1jqnPwVGUWXF7uNUAFnwy6yJ8LJbV
mR0nhu4Y4aWnJeBa1b/VdaUujnOUNTccRM087jS0v/HYma05v2AEEP/gfps1iN8x
LReJomY4wJY1ndS0wT71Nt3dvQ3AZphWoXGeONV2bE3gMBsRv0Oo/DYDV4/VsTHl
sMV1do3gF/xAUqWawlZQkNcibME+sQqfE7gZ04hlmDATU2zmbzwuHtFiNv8mVv7O
RQIDAQAB
-----END PUBLIC KEY-----
`},
		{`---- BEGIN SSH2 PUBLIC KEY ----
Comment: "256-bit ED25519, converted by andrew@phaedra from OpenSSH"
AAAAC3NzaC1lZDI1NTE5AAAAICV0MGX/W9IvLA4FXpIuUcdDcbj5KX4syHgsTy7soVgf
---- END SSH2 PUBLIC KEY ----
`},
	} {
		_, err := CheckPublicKeyString(test.content)
		assert.NoError(t, err)
	}

	for _, invalidKeys := range []struct {
		content string
	}{
		{"test"},
		{"---- NOT A REAL KEY ----"},
		{"bad\nkey"},
		{"\t\t:)\t\r\n"},
		{"\r\ntest \r\ngitea\r\n\r\n"},
	} {
		_, err := CheckPublicKeyString(invalidKeys.content)
		assert.Error(t, err)
	}
}

func Test_calcFingerprint(t *testing.T) {
	testCases := []struct {
		name    string
		fp      string
		content string
	}{
		{"dsa-1024", "SHA256:fSIHQlpKMDsGPVAXI8BPYfRp+e2sfvSt1sMrPsFiXrc", "ssh-dss AAAAB3NzaC1kc3MAAACBAOChCC7lf6Uo9n7BmZ6M8St19PZf4Tn59NriyboW2x/DZuYAz3ibZ2OkQ3S0SqDIa0HXSEJ1zaExQdmbO+Ux/wsytWZmCczWOVsaszBZSl90q8UnWlSH6P+/YA+RWJm5SFtuV9PtGIhyZgoNuz5kBQ7K139wuQsecdKktISwTakzAAAAFQCzKsO2JhNKlL+wwwLGOcLffoAmkwAAAIBpK7/3xvduajLBD/9vASqBQIHrgK2J+wiQnIb/Wzy0UsVmvfn8A+udRbBo+csM8xrSnlnlJnjkJS3qiM5g+eTwsLIV1IdKPEwmwB+VcP53Cw6lSyWyJcvhFb0N6s08NZysLzvj0N+ZC/FnhKTLzIyMtkHf/IrPCwlM+pV/M/96YgAAAIEAqQcGn9CKgzgPaguIZooTAOQdvBLMI5y0bQjOW6734XOpqQGf/Kra90wpoasLKZjSYKNPjE+FRUOrStLrxcNs4BeVKhy2PYTRnybfYVk1/dmKgH6P1YSRONsGKvTsH6c5IyCRG0ncCgYeF8tXppyd642982daopE7zQ/NPAnJfag= nocomment"},
		{"rsa-1024", "SHA256:vSnDkvRh/xM6kMxPidLgrUhq3mCN7CDaronCEm2joyQ", "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAAAgQDAu7tvIvX6ZHrRXuZNfkR3XLHSsuCK9Zn3X58lxBcQzuo5xZgB6vRwwm/QtJuF+zZPtY5hsQILBLmF+BZ5WpKZp1jBeSjH2G7lxet9kbcH+kIVj0tPFEoyKI9wvWqIwC4prx/WVk2wLTJjzBAhyNxfEq7C9CeiX9pQEbEqJfkKCQ== nocomment\n"},
		{"rsa-2048", "SHA256:ZHD//a1b9VuTq9XSunAeYjKeU1xDa2tBFZYrFr2Okkg", "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQDMZXh+1OBUwSH9D45wTaxErQIN9IoC9xl7MKJkqvTvv6O5RR9YW/IK9FbfjXgXsppYGhsCZo1hFOOsXHMnfOORqu/xMDx4yPuyvKpw4LePEcg4TDipaDFuxbWOqc/BUZRZcXu41QAWfDLrInwsltWZHSeG7hjhpacl4FrVv9V1pS6Oc5Q1NxxEzTzuNLS/8diZrTm/YAQQ/+B+mzWI3zEtF4miZjjAljWd1LTBPvU23d29DcBmmFahcZ441XZsTeAwGxG/Q6j8NgNXj9WxMeWwxXV2jeAX/EBSpZrCVlCQ1yJswT6xCp8TuBnTiGWYMBNTbOZvPC4e0WI2/yZW/s5F nocomment"},
		{"ecdsa-256", "SHA256:Bqx/xgWqRKLtkZ0Lr4iZpgb+5lYsFpSwXwVZbPwuTRw", "ecdsa-sha2-nistp256 AAAAE2VjZHNhLXNoYTItbmlzdHAyNTYAAAAIbmlzdHAyNTYAAABBBFQacN3PrOll7PXmN5B/ZNVahiUIqI05nbBlZk1KXsO3d06ktAWqbNflv2vEmA38bTFTfJ2sbn2B5ksT52cDDbA= nocomment"},
		{"ecdsa-384", "SHA256:4qfJOgJDtUd8BrEjyVNdI8IgjiZKouztVde43aDhe1E", "ecdsa-sha2-nistp384 AAAAE2VjZHNhLXNoYTItbmlzdHAzODQAAAAIbmlzdHAzODQAAABhBINmioV+XRX1Fm9Qk2ehHXJ2tfVxW30ypUWZw670Zyq5GQfBAH6xjygRsJ5wWsHXBsGYgFUXIHvMKVAG1tpw7s6ax9oA+dJOJ7tj+vhn8joFqT+sg3LYHgZkHrfqryRasQ== nocomment"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("Native", func(t *testing.T) {
				fpN, err := calcFingerprintNative(tc.content)
				assert.NoError(t, err)
				assert.Equal(t, tc.fp, fpN)
			})
			t.Run("SSHKeygen", func(t *testing.T) {
				fpK, err := calcFingerprintSSHKeygen(tc.content)
				assert.NoError(t, err)
				assert.Equal(t, tc.fp, fpK)
			})
		})
	}
}
