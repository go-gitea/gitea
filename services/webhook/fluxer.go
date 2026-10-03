// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package webhook

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/netip"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"

	webhook_model "gitea.dev/models/webhook"
	"gitea.dev/modules/json"
	webhook_module "gitea.dev/modules/webhook"

	"golang.org/x/net/idna"
)

type FluxerMeta struct {
	Username string `json:"username"`
	IconURL  string `json:"icon_url"`
}

type FluxerEmbedAuthor struct {
	Name    string `json:"name"`
	URL     string `json:"url,omitempty"`
	IconURL string `json:"icon_url,omitempty"`
}

type FluxerEmbed struct {
	Title       string             `json:"title,omitempty"`
	Description string             `json:"description,omitempty"`
	URL         string             `json:"url,omitempty"`
	Color       int                `json:"color"`
	Author      *FluxerEmbedAuthor `json:"author,omitempty"`
}

type FluxerAllowedMentions struct {
	Parse []string `json:"parse"`
}

type FluxerPayload struct {
	Username        string                `json:"username,omitempty"`
	AvatarURL       string                `json:"avatar_url,omitempty"`
	Embeds          []FluxerEmbed         `json:"embeds"`
	AllowedMentions FluxerAllowedMentions `json:"allowed_mentions"`
}

func fluxerWhitespace(r rune) bool {
	return r >= '\t' && r <= '\r' || r == ' ' || r == '\u00a0' || r == '\u1680' ||
		r >= '\u2000' && r <= '\u200a' || r == '\u2028' || r == '\u2029' ||
		r == '\u202f' || r == '\u205f' || r == '\u3000' || r == '\ufeff'
}

func normalizeFluxerString(s string) string {
	s = strings.Map(func(r rune) rune {
		if r == '\u000c' || r == '\u202e' {
			return -1
		}
		return r
	}, s)
	return strings.TrimFunc(s, fluxerWhitespace)
}

// Fluxer measures string limits in UTF-16 code units.
func truncateFluxerString(s string, limit int) string {
	s = normalizeFluxerString(s)
	for i, r := range s {
		limit -= utf16.RuneLen(r)
		if limit < 0 {
			return s[:i]
		}
	}
	return s
}

var fluxerTLD = regexp.MustCompile(`^(?:[a-zA-Z\x{00A1}-\x{00A8}\x{00AA}-\x{D7FF}\x{F900}-\x{FDCF}\x{FDF0}-\x{FFEF}]{2,}|xn[a-zA-Z0-9-]{2,})$`)

var fluxerHostnameLabel = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

func fluxerURL(s string) string {
	s = normalizeFluxerString(s)
	if (!strings.HasPrefix(s, "http://") && !strings.HasPrefix(s, "https://")) || s != truncateFluxerString(s, 2048) || strings.ContainsAny(s, "<>\\") || strings.ContainsFunc(s, fluxerWhitespace) {
		return ""
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Host == "" || u.Opaque != "" {
		return ""
	}
	if port := u.Port(); port != "" {
		p, err := strconv.Atoi(port)
		if err != nil || p < 1 || p > 65535 {
			return ""
		}
	}
	host := u.Hostname()
	if strings.ContainsAny(host, "。．｡") || strings.ContainsFunc(host, func(r rune) bool { return r >= '\uff01' && r <= '\uff5e' }) {
		return ""
	}
	if ip, err := netip.ParseAddr(host); err == nil {
		if ip.Zone() != "" || (ip.Is6() != strings.HasPrefix(u.Host, "[")) {
			return ""
		}
		return s
	}
	if strings.HasPrefix(u.Host, "[") {
		return ""
	}
	rawLabels := strings.Split(host, ".")
	if len(rawLabels) < 2 || !fluxerTLD.MatchString(rawLabels[len(rawLabels)-1]) {
		return ""
	}
	for _, label := range rawLabels {
		if label != truncateFluxerString(label, 63) {
			return ""
		}
	}
	host, err = idna.Lookup.ToASCII(host)
	if err != nil || len(host) > 253 {
		return ""
	}
	for label := range strings.SplitSeq(host, ".") {
		if !fluxerHostnameLabel.MatchString(label) {
			return ""
		}
	}
	return s
}

func (m *FluxerMeta) Validate() error {
	m.Username = normalizeFluxerString(m.Username)
	m.IconURL = normalizeFluxerString(m.IconURL)
	if m.Username != truncateFluxerString(m.Username, 80) {
		return errors.New("Fluxer username must not exceed 80 UTF-16 code units")
	}
	if m.IconURL != "" && fluxerURL(m.IconURL) == "" {
		return errors.New("Fluxer icon URL must be a valid HTTP(S) URL of at most 2048 UTF-16 code units")
	}
	return nil
}

func GetFluxerHook(w *webhook_model.Webhook) (*FluxerMeta, error) {
	m := &FluxerMeta{}
	if w.Meta != "" {
		if err := json.Unmarshal([]byte(w.Meta), m); err != nil {
			return nil, fmt.Errorf("GetFluxerHook: %w", err)
		}
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return m, nil
}

func fluxerPayloadFromDiscord(p DiscordPayload) FluxerPayload {
	result := FluxerPayload{
		Username:        p.Username,
		AvatarURL:       p.AvatarURL,
		Embeds:          make([]FluxerEmbed, 0, len(p.Embeds)),
		AllowedMentions: FluxerAllowedMentions{Parse: []string{}},
	}
	for _, embed := range p.Embeds {
		e := FluxerEmbed{
			Title:       truncateFluxerString(embed.Title, 256),
			Description: truncateFluxerString(embed.Description, 4096),
			URL:         fluxerURL(embed.URL),
			Color:       embed.Color,
		}
		if name := truncateFluxerString(embed.Author.Name, 256); name != "" {
			e.Author = &FluxerEmbedAuthor{
				Name:    name,
				URL:     fluxerURL(embed.Author.URL),
				IconURL: fluxerURL(embed.Author.IconURL),
			}
		}
		result.Embeds = append(result.Embeds, e)
	}
	return result
}

func newFluxerRequest(_ context.Context, w *webhook_model.Webhook, t *webhook_model.HookTask) (*http.Request, []byte, error) {
	meta, err := GetFluxerHook(w)
	if err != nil {
		return nil, nil, err
	}
	payload, err := newPayload(discordConvertor{Username: meta.Username, AvatarURL: meta.IconURL}, []byte(t.PayloadContent), t.EventType)
	if err != nil {
		return nil, nil, err
	}
	return prepareJSONRequest(fluxerPayloadFromDiscord(payload), w, t, true)
}

func init() {
	RegisterWebhookRequester(webhook_module.FLUXER, newFluxerRequest)
}
