// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package imagecaptcha

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"errors"
	"image/color"
	"image/png"
	"math/rand/v2"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"gitea.dev/modules/cache"
	"gitea.dev/modules/log"
	"gitea.dev/modules/util"

	"github.com/go-chi/chi/v5"
)

const (
	cacheKeyPrefix = "captcha_"
	ttlSeconds     = 600
	digitCount     = 6
)

var primaryColors = []color.RGBA{ // readable on both light and dark backgrounds
	{R: 234, G: 67, B: 53, A: 255},
	{R: 66, G: 133, B: 244, A: 255},
	{R: 52, G: 168, B: 83, A: 255},
	{R: 251, G: 188, B: 5, A: 255},
	{R: 171, G: 71, B: 188, A: 255},
}

var idPattern = regexp.MustCompile(`^[0-9A-Za-z]{20}$`)

type pngBufferPool struct{ sync.Pool }

func (p *pngBufferPool) Get() *png.EncoderBuffer {
	buf, _ := p.Pool.Get().(*png.EncoderBuffer)
	return buf
}

func (p *pngBufferPool) Put(buf *png.EncoderBuffer) {
	p.Pool.Put(buf)
}

var pngEncoder = png.Encoder{BufferPool: &pngBufferPool{}}

func randomDigits() string {
	digits := make([]byte, digitCount)
	for i := range digits {
		digits[i] = byte(util.CryptoRandomInt(10))
	}
	return string(digits)
}

var noiseKey = util.CryptoRandomBytes(32)

// noiseRand makes refetches of an image identical, so averaging them does not remove the noise
func noiseRand(id, digits string) *rand.Rand {
	mac := hmac.New(sha256.New, noiseKey)
	_, _ = mac.Write([]byte(id + "\x00" + digits))
	return rand.New(rand.NewChaCha8([32]byte(mac.Sum(nil))))
}

func Create() (string, error) {
	id := util.CryptoRandomString(20)
	if err := cache.GetCache().Put(cacheKeyPrefix+id, randomDigits(), ttlSeconds); err != nil {
		return "", err
	}
	return id, nil
}

func Verify(id, answer string) bool {
	digits, ok := cache.GetCache().GetAndDelete(cacheKeyPrefix + id)
	return ok && answer == strings.Map(func(digit rune) rune { return digit + '0' }, digits)
}

func renderImage(id string, reload bool) ([]byte, error) {
	key := cacheKeyPrefix + id
	var digits string
	if reload {
		if !idPattern.MatchString(id) {
			return nil, util.ErrNotExist
		}
		digits = randomDigits() // also for an expired id, so the form's captcha_id stays usable
		if err := cache.GetCache().Put(key, digits, ttlSeconds); err != nil {
			return nil, err
		}
	} else {
		var ok bool
		if digits, ok = cache.GetCache().Get(key); !ok {
			return nil, util.ErrNotExist
		}
	}
	var buf bytes.Buffer
	err := pngEncoder.Encode(&buf, drawImage(noiseRand(id, digits), []byte(digits)))
	return buf.Bytes(), err
}

func ServeImage(resp http.ResponseWriter, req *http.Request) {
	resp.Header().Set("Cache-Control", "no-store")
	img, err := renderImage(chi.URLParam(req, "id"), req.URL.Query().Get("reload") != "")
	switch {
	case errors.Is(err, util.ErrNotExist):
		http.NotFound(resp, req)
	case err != nil:
		log.Error("Unable to render captcha: %v", err)
		http.Error(resp, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	default:
		resp.Header().Set("Content-Type", "image/png")
		_, _ = resp.Write(img)
	}
}
