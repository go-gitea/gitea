// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package imagecaptcha

import (
	"crypto/hmac"
	"crypto/sha256"
	"image/color"
	"image/png"
	"math/rand/v2"
	"net/http"
	"regexp"
	"strconv"
	"sync"

	"gitea.dev/modules/cache"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"
)

const (
	cacheKeyPrefix = "captcha_"
	ttlSeconds     = 600
	codeLength     = 6
)

var primaryColors = []color.RGBA{ // readable on both light and dark backgrounds
	{R: 234, G: 67, B: 53, A: 255},
	{R: 66, G: 133, B: 244, A: 255},
	{R: 52, G: 168, B: 83, A: 255},
	{R: 251, G: 188, B: 5, A: 255},
	{R: 171, G: 71, B: 188, A: 255},
}

type pngBufferPool struct{ sync.Pool }

func (p *pngBufferPool) Get() *png.EncoderBuffer {
	buf, _ := p.Pool.Get().(*png.EncoderBuffer)
	return buf
}

func (p *pngBufferPool) Put(buf *png.EncoderBuffer) {
	p.Pool.Put(buf)
}

func randomCode() string {
	s := "000000" + strconv.Itoa(util.FastCryptoRandomInt(1000000))
	return s[len(s)-6:]
}

var globalVars = sync.OnceValue(func() (ret struct {
	IdLength int
	IdRegexp *regexp.Regexp
	NoiseKey []byte
},
) {
	ret.IdLength = 40
	ret.IdRegexp = regexp.MustCompile(`^[0-9a-f]{40}$`)
	ret.NoiseKey = util.FastCryptoRandomBytes(32)
	return
})

// noiseRand makes refetches of an image identical, so averaging them does not remove the noise
func noiseRand(id, code string) *rand.Rand {
	mac := hmac.New(sha256.New, globalVars().NoiseKey)
	_, _ = mac.Write([]byte(id + "\x00" + code))
	return util.FastCryptoRand([32]byte(mac.Sum(nil)))
}

func CreateNew() (string, error) {
	id := util.FastCryptoRandomHex(globalVars().IdLength)
	_, err := PrepareCode(id, true)
	return id, err
}

func PrepareCode(id string, generateNew bool) (code string, err error) {
	if !globalVars().IdRegexp.MatchString(id) {
		return "", nil
	}
	cacheKey := cacheKeyPrefix + id
	if generateNew {
		code = randomCode()
		if err = cache.GetCache().Put(cacheKey, code, ttlSeconds); err != nil {
			return "", err
		}
	} else {
		code, _ = cache.GetCache().Get(cacheKey)
	}
	return code, nil
}

func Verify(id, answer string) bool {
	if !globalVars().IdRegexp.MatchString(id) {
		return false
	}
	key := cacheKeyPrefix + id
	code, ok := cache.GetCache().Get(key)
	_ = cache.GetCache().Delete(key)
	return ok && answer == code
}

func ServeImage(resp http.ResponseWriter, req *http.Request) {
	urlQuery := req.URL.Query()
	id, reload := urlQuery.Get("id"), urlQuery.Get("reload") != ""
	code, err := PrepareCode(id, reload)
	if err != nil {
		log.Error("Failed to prepare captcha code for id %s: %v", id, err)
		http.Error(resp, "Failed to prepare captcha code", http.StatusInternalServerError)
		return
	} else if code == "" {
		http.NotFound(resp, req)
		return
	}

	resp.Header().Set("Cache-Control", "no-store")
	resp.Header().Set("Content-Type", "image/png")
	if req.Method == http.MethodGet {
		pngEncoder := png.Encoder{BufferPool: &pngBufferPool{}}
		_ = pngEncoder.Encode(resp, drawImage(noiseRand(id, code), code))
	}
}
